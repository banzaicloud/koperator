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
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestGenerateBrokerConfig(t *testing.T) {
	tests := []struct {
		testName                string
		readOnlyConfig          string
		clusterWideConfig       string
		perBrokerReadOnlyConfig string
		perBrokerConfig         string
		expectedConfig          string
		perBrokerStorageConfig  []v1beta1.StorageConfig
	}{
		{
			testName:                "basicConfig",
			readOnlyConfig:          ``,
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			expectedConfig: `advertised.listeners=PLAINTEXT://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=PLAINTEXT://kafka-0.kafka.svc.cluster.local:9092
listener.security.protocol.map=PLAINTEXT:PLAINTEXT
listeners=PLAINTEXT://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
security.inter.broker.protocol=PLAINTEXT
super.users=
zookeeper.connect=example.zk:2181`,
		},
		{
			testName:                "basicConfigWithCustomStorage",
			readOnlyConfig:          ``,
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			perBrokerStorageConfig: []v1beta1.StorageConfig{
				{
					MountPath: "/kafka-logs",
				},
			},
			expectedConfig: `advertised.listeners=PLAINTEXT://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=PLAINTEXT://kafka-0.kafka.svc.cluster.local:9092
listener.security.protocol.map=PLAINTEXT:PLAINTEXT
listeners=PLAINTEXT://:9092
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
security.inter.broker.protocol=PLAINTEXT
super.users=
zookeeper.connect=example.zk:2181`,
		},
		{
			testName: "readOnlyRedefinedInOneBroker",
			readOnlyConfig: `
auto.create.topics.enable=false
control.plane.listener.name=thisisatest
`,
			clusterWideConfig: `
background.threads=20
compression.type=snappy
`,
			perBrokerConfig: ``,
			perBrokerReadOnlyConfig: `
auto.create.topics.enable=true
`,
			expectedConfig: `advertised.listeners=PLAINTEXT://kafka-0.kafka.svc.cluster.local:9092
auto.create.topics.enable=true
broker.id=0
control.plane.listener.name=thisisatest
cruise.control.metrics.reporter.bootstrap.servers=PLAINTEXT://kafka-0.kafka.svc.cluster.local:9092
listener.security.protocol.map=PLAINTEXT:PLAINTEXT
listeners=PLAINTEXT://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
security.inter.broker.protocol=PLAINTEXT
super.users=
zookeeper.connect=example.zk:2181`,
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
							Name:      "kafka",
							Namespace: "kafka",
						},
						Spec: v1beta1.KafkaClusterSpec{
							ZKAddresses: []string{"example.zk:2181"},
							ListenersConfig: v1beta1.ListenersConfig{
								InternalListeners: []v1beta1.InternalListenerConfig{
									{
										Type:                            "plaintext",
										Name:                            "plaintext",
										UsedForInnerBrokerCommunication: true,
										ContainerPort:                   9092,
									},
								},
							},
							ReadOnlyConfig:    test.readOnlyConfig,
							ClusterWideConfig: test.clusterWideConfig,
							Brokers: []v1beta1.Broker{{
								Id:             0,
								ReadOnlyConfig: test.perBrokerReadOnlyConfig,
								BrokerConfig: &v1beta1.BrokerConfig{
									Config:         test.perBrokerConfig,
									StorageConfigs: test.perBrokerStorageConfig,
								},
							},
							},
						},
					},
				},
			}
			generatedConfig := r.generateBrokerConfig(0, r.KafkaCluster.Spec.Brokers[0].BrokerConfig, "", "", "", []string{}, nil)

			if generatedConfig != test.expectedConfig {
				t.Errorf("the expected config is %s, received: %s", test.expectedConfig, generatedConfig)
			}
		})
	}
}
