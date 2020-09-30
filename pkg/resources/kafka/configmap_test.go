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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGenerateBrokerConfig(t *testing.T) {
	tests := []struct {
		testName                string
		readOnlyConfig          string
		zkAddresses             []string
		zkPath                  string
		kubernetesClusterDomain string
		clusterWideConfig       string
		perBrokerReadOnlyConfig string
		perBrokerConfig         string
		expectedConfig          string
		perBrokerStorageConfig  []v1beta1.StorageConfig
	}{
		{
			testName:                "basicConfig",
			readOnlyConfig:          ``,
			zkAddresses:             []string{"example.zk:2181"},
			zkPath:                  ``,
			kubernetesClusterDomain: ``,
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                "basicConfigWithZKPath",
			readOnlyConfig:          ``,
			zkPath:                  `/kafka`,
			kubernetesClusterDomain: ``,
			zkAddresses:             []string{"example.zk:2181"},
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/kafka`,
		},
		{
			testName:                "basicConfigWithSimpleZkPath",
			readOnlyConfig:          ``,
			zkPath:                  `/`,
			kubernetesClusterDomain: ``,
			zkAddresses:             []string{"example.zk:2181"},
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                "basicConfigWithMultipleZKAddressAndPath",
			readOnlyConfig:          ``,
			zkPath:                  `/kafka`,
			zkAddresses:             []string{"example.zk:2181", "example.zk-1:2181"},
			kubernetesClusterDomain: ``,
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181,example.zk-1:2181/kafka`,
		},
		{
			testName:                "basicConfigWithCustomStorage",
			readOnlyConfig:          ``,
			zkAddresses:             []string{"example.zk:2181"},
			zkPath:                  ``,
			kubernetesClusterDomain: ``,
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			perBrokerStorageConfig: []v1beta1.StorageConfig{
				{
					MountPath: "/kafka-logs",
				},
			},
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                "basicConfigWithClusterDomain",
			readOnlyConfig:          ``,
			zkAddresses:             []string{"example.zk:2181"},
			zkPath:                  ``,
			kubernetesClusterDomain: `foo.bar`,
			clusterWideConfig:       ``,
			perBrokerConfig:         ``,
			perBrokerReadOnlyConfig: ``,
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.foo.bar:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.foo.bar:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                "readOnlyRedefinedInOneBroker",
			zkAddresses:             []string{"example.zk:2181"},
			zkPath:                  ``,
			kubernetesClusterDomain: ``,
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
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
auto.create.topics.enable=true
broker.id=0
control.plane.listener.name=thisisatest
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
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
							ZKAddresses: test.zkAddresses,
							ZKPath:      test.zkPath,
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
							},
							ReadOnlyConfig:          test.readOnlyConfig,
							KubernetesClusterDomain: test.kubernetesClusterDomain,
							ClusterWideConfig:       test.clusterWideConfig,
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
			generatedConfig := r.generateBrokerConfig(0, r.KafkaCluster.Spec.Brokers[0].BrokerConfig, []string{}, "", "", []string{}, logf.NullLogger{})

			if generatedConfig != test.expectedConfig {
				t.Errorf("the expected config is %s, received: %s", test.expectedConfig, generatedConfig)
			}
		})
	}
}
