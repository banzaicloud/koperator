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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/util"
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
			generatedConfig := r.generateBrokerConfig(0, r.KafkaCluster.Spec.Brokers[0].BrokerConfig, map[string]string{}, "", "", []string{}, logf.NullLogger{})

			if generatedConfig != test.expectedConfig {
				t.Errorf("the expected config is %s, received: %s", test.expectedConfig, generatedConfig)
			}
		})
	}
}

func TestShouldRefreshOnlyPerBrokerConfigs(t *testing.T) {
	testCases := []struct {
		Description    string
		CurrentConfigs map[string]string
		DesiredConfigs map[string]string
		Result         bool
	}{
		{
			Description: "configs did not change",
			CurrentConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "unmodified_value_2",
			},
			DesiredConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "unmodified_value_2",
			},
			Result: true,
		},
		{
			Description: "only non per-broker config changed",
			CurrentConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "modified_value_2",
			},
			DesiredConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "modified_value_3",
			},
			Result: false,
		},
		{
			Description: "only per-broker config changed",
			CurrentConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"ssl.client.auth":     "modified_value_2",
			},
			DesiredConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"ssl.client.auth":     "modified_value_3",
			},
			Result: true,
		},
		{
			Description: "per-broker and non per-broker configs have changed",
			CurrentConfigs: map[string]string{
				"modified_config_1": "modified_value_1",
				"ssl.client.auth":   "modified_value_3",
			},
			DesiredConfigs: map[string]string{
				"modified_config_1": "modified_value_2",
				"ssl.client.auth":   "modified_value_4",
			},
			Result: false,
		},
		{
			Description: "security protocol map can be changed as a per-broker config",
			CurrentConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol2",
			},
			DesiredConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener3:protocol3",
			},
			Result: true,
		},
		{
			Description: "security protocol map can't be changed as a per-broker config",
			CurrentConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol2",
			},
			DesiredConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol3",
			},
			Result: false,
		},
		{
			Description:    "security protocol map added as config",
			CurrentConfigs: map[string]string{},
			DesiredConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol2",
			},
			Result: true,
		},
	}
	logger := util.CreateLogger(false, false)
	for i, testCase := range testCases {
		if shouldRefreshOnlyPerBrokerConfigs(testCase.CurrentConfigs, testCase.DesiredConfigs, logger) != testCase.Result {
			t.Errorf("test case %d failed: %s", i, testCase.Description)
		}
	}
}

func TestCollectTouchedConfigs(t *testing.T) {
	testCases := []struct {
		DesiredConfigs string
		CurrentConfigs string
		Result         map[string]struct{}
	}{
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value2
key3=value3`,
			Result: map[string]struct{}{},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2`,
			CurrentConfigs: `key1=value1
key2=value2
key3=value3`,
			Result: map[string]struct{}{
				"key3": {},
			},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value2`,
			Result: map[string]struct{}{
				"key3": {},
			},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value4
key3=value3`,
			Result: map[string]struct{}{
				"key2": {},
			},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3
key4=value4`,
			CurrentConfigs: `key1=value6
key2=value2
key3=value3
key5=value5`,
			Result: map[string]struct{}{
				"key1": {},
				"key4": {},
				"key5": {},
			},
		},
	}

	logger := util.CreateLogger(false, false)
	for _, testCase := range testCases {
		desired := &corev1.ConfigMap{
			Data: map[string]string{
				"broker-config": testCase.DesiredConfigs,
			},
		}
		current := &corev1.ConfigMap{
			Data: map[string]string{
				"broker-config": testCase.CurrentConfigs,
			},
		}
		touchedConfigs := collectTouchedConfigs(
			util.ParsePropertiesFormat(current.Data["broker-config"]),
			util.ParsePropertiesFormat(desired.Data["broker-config"]), logger)
		if !reflect.DeepEqual(touchedConfigs, testCase.Result) {
			t.Errorf("comparison failed - expected: %s, actual: %s", testCase.Result, touchedConfigs)
		}
	}
}
