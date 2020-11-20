// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafka

import (
	"reflect"
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGeneratePerBrokerConfig(t *testing.T) {
	tests := []struct {
		testName                string
		brokerId                int32
		loadBalancerIPs         []string
		serverPass              string
		sslSecrets              *v1beta1.SSLSecrets
		externalListeners       []v1beta1.ExternalListenerConfig
		brokerConfig            v1beta1.BrokerConfig
		kubernetesClusterDomain string
		headlessServiceEnabled  bool
		expectedOutput          map[string]string
	}{
		{
			testName: "basicConfig",
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-0.kafka.svc.cluster.local:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
			},
		},
		{
			testName: "customBrokerId",
			brokerId: 1,
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-1.kafka.svc.cluster.local:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
			},
		},
		{
			testName:   "sslSecretsWithServerPass",
			serverPass: "test",
			sslSecrets: &v1beta1.SSLSecrets{
				TLSSecretName:   "testsecret",
				JKSPasswordName: "testpwd",
				Create:          false,
				PKIBackend:      "envoy",
			},
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-0.kafka.svc.cluster.local:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
				"ssl.client.auth":                "required",
				"ssl.keystore.location":          "/var/run/secrets/java.io/keystores/server/keystore.jks",
				"ssl.keystore.password":          "test",
				"ssl.truststore.location":        "/var/run/secrets/java.io/keystores/server/truststore.jks",
				"ssl.truststore.password":        "test",
			},
		},
		{
			testName:        "externalListenerWithLoadBalancerIP",
			loadBalancerIPs: []string{"localhost"},
			externalListeners: []v1beta1.ExternalListenerConfig{{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9093,
				},
			}},
			expectedOutput: map[string]string{
				"advertised.listeners":           "EXTERNAL://localhost:0,INTERNAL://kafka_cluster-0.kafka.svc.cluster.local:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092,EXTERNAL://:9093",
			},
		},
		{
			testName:                "customClusterDomain",
			kubernetesClusterDomain: "test1",
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-0.kafka.svc.test1:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
			},
		},
		{
			testName:                "headlessMode",
			headlessServiceEnabled:  true,
			kubernetesClusterDomain: "test2",
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-0.kafka_cluster-headless.kafka.svc.test2:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
			},
		},
		{
			testName: "customUserConfig",
			brokerConfig: v1beta1.BrokerConfig{
				Config: "sasl.enabled.mechanisms=PLAIN",
			},
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-0.kafka.svc.cluster.local:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
				"sasl.enabled.mechanisms":        "PLAIN",
			},
		},
		{
			testName: "nonOverridableConfig",
			brokerConfig: v1beta1.BrokerConfig{
				Config: "advanced.listeners:should-not-override",
			},
			expectedOutput: map[string]string{
				"advertised.listeners":           "INTERNAL://kafka_cluster-0.kafka.svc.cluster.local:9092",
				"listener.security.protocol.map": "INTERNAL:PLAINTEXT",
				"listeners":                      "INTERNAL://:9092",
			},
		},
	}

	t.Parallel()

	for _, test := range tests {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			r := Reconciler{
				Scheme: scheme.Scheme,
				Reconciler: resources.Reconciler{
					KafkaCluster: &v1beta1.KafkaCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kafka_cluster",
							Namespace: "kafka",
						},
						Spec: v1beta1.KafkaClusterSpec{
							HeadlessServiceEnabled:  test.headlessServiceEnabled,
							KubernetesClusterDomain: test.kubernetesClusterDomain,
							ListenersConfig: v1beta1.ListenersConfig{
								InternalListeners: []v1beta1.InternalListenerConfig{
									{CommonListenerSpec: v1beta1.CommonListenerSpec{
										Type:          "plaintext",
										Name:          "internal",
										ContainerPort: 9092,
									},
										UsedForInnerBrokerCommunication: true,
									},
								},
								ExternalListeners: test.externalListeners,
								SSLSecrets:        test.sslSecrets,
							},
							Brokers: []v1beta1.Broker{
								{
									Id: 0,
								},
								{
									Id: 1,
								},
							},
						},
					},
				},
			}
			generatedConfig := r.getDynamicConfigs(test.brokerId, test.loadBalancerIPs, test.serverPass, &test.brokerConfig, logf.NullLogger{})

			if !reflect.DeepEqual(generatedConfig, test.expectedOutput) {
				t.Errorf("the expected config is %s, received: %s", test.expectedOutput, generatedConfig)
			}
		})
	}
}
