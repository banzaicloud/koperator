// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources"
	mocks "github.com/banzaicloud/koperator/pkg/resources/kafka/mocks"
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

func TestGetMountPathsFromBrokerConfigMap(t *testing.T) {
	tests := []struct {
		testName        string
		brokerConfigMap v1.ConfigMap
		expectedLogDirs []string
	}{
		{
			testName: "simple case",
			brokerConfigMap: v1.ConfigMap{
				Data: map[string]string{kafkautils.ConfigPropertyName: `inter.broker.listener.name=INTERNAL\nlistener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=INTERNAL://:29092,CONTROLLER://:29093
log.dirs=/kafka-logs3/kafka,/kafka-logs/kafka,/kafka-logs2/kafka,/kafka-logs4/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter\noffsets.topic.replication.factor=2
zookeeper.connect=zookeeper-server-client.zookeeper:2181/
`},
			},
			expectedLogDirs: []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
		},
		{
			testName: "no old configs",
			brokerConfigMap: v1.ConfigMap{
				Data: map[string]string{},
			},
			expectedLogDirs: []string{},
		},
	}
	for _, test := range tests {
		logDirs, err := getMountPathsFromBrokerConfigMap(&test.brokerConfigMap)
		if err != nil {
			t.Errorf("err should be nil, got: %v", err)
		}
		if len(logDirs) != 0 && len(test.expectedLogDirs) != 0 {
			if !reflect.DeepEqual(logDirs, test.expectedLogDirs) {
				t.Errorf("expected: %s, got: %s", test.expectedLogDirs, logDirs)
			}
		}
	}
}

func TestMergeMountPaths(t *testing.T) {
	tests := []struct {
		testName                string
		mountPathNew            []string
		mountPathOld            []string
		expectedMergedMountPath []string
		expectedRemoved         bool
	}{
		{
			testName:                "no old mountPath",
			mountPathNew:            []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			mountPathOld:            []string{},
			expectedMergedMountPath: []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedRemoved:         false,
		},
		{
			testName:                "same",
			mountPathNew:            []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			mountPathOld:            []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedMergedMountPath: []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedRemoved:         false,
		},
		{
			testName:                "changed order",
			mountPathNew:            []string{"/kafka-logs/kafka", "/kafka-logs3/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			mountPathOld:            []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedMergedMountPath: []string{"/kafka-logs/kafka", "/kafka-logs3/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedRemoved:         false,
		},
		{
			testName:                "removed one",
			mountPathNew:            []string{"/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			mountPathOld:            []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedMergedMountPath: []string{"/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka", "/kafka-logs3/kafka"},
			expectedRemoved:         true,
		},
		{
			testName:                "removed all",
			mountPathNew:            []string{},
			mountPathOld:            []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedMergedMountPath: []string{"/kafka-logs3/kafka", "/kafka-logs/kafka", "/kafka-logs2/kafka", "/kafka-logs4/kafka"},
			expectedRemoved:         true,
		},
	}
	for _, test := range tests {
		mergedMountPaths, isRemoved := mergeMountPaths(test.mountPathOld, test.mountPathNew)
		if !reflect.DeepEqual(mergedMountPaths, test.expectedMergedMountPath) {
			t.Errorf("testName: %s, expected: %s, got: %s", test.testName, test.expectedMergedMountPath, mergedMountPaths)
		}
		require.Equal(t, test.expectedRemoved, isRemoved)
	}
}

func TestGenerateBrokerConfig(t *testing.T) { //nolint funlen
	tests := []struct {
		testName                  string
		readOnlyConfig            string
		zkAddresses               []string
		zkPath                    string
		kubernetesClusterDomain   string
		clusterWideConfig         string
		perBrokerReadOnlyConfig   string
		perBrokerConfig           string
		advertisedListenerAddress string
		listenerType              string
		sslClientAuth             v1beta1.SSLClientAuthentication
		expectedConfig            string
		perBrokerStorageConfig    []v1beta1.StorageConfig
	}{
		{
			testName:                  "basicConfig",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "basicConfigWithZKPath",
			readOnlyConfig:            ``,
			zkPath:                    `/kafka`,
			kubernetesClusterDomain:   ``,
			zkAddresses:               []string{"example.zk:2181"},
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/kafka`,
		},
		{
			testName:                  "basicConfigWithSimpleZkPath",
			readOnlyConfig:            ``,
			zkPath:                    `/`,
			kubernetesClusterDomain:   ``,
			zkAddresses:               []string{"example.zk:2181"},
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "basicConfigWithMultipleZKAddressAndPath",
			readOnlyConfig:            ``,
			zkPath:                    `/kafka`,
			zkAddresses:               []string{"example.zk:2181", "example.zk-1:2181"},
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
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
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "basicConfigWithClusterDomain",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   `foo.bar`,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.foo.bar:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.foo.bar:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.foo.bar:9092
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
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
auto.create.topics.enable=true
broker.id=0
control.plane.listener.name=thisisatest
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSasl",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "sasl_plaintext",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:SASL_PLAINTEXT
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_SSLClientAuth_not_provided",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=required
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_SSLClientAuth_required",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "required",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=required
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_SSLClientAuth_requested",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "requested",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=requested
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_SSLClientAuth_none",
			readOnlyConfig:            ``,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "none",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=none
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_with_readOnly-superUsers1",
			readOnlyConfig:            `super.users=User:CN=custom-superuser1;User:CN=custom-superuser2`,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "none",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=none
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local;User:CN=custom-superuser1;User:CN=custom-superuser2
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_with_readOnly-superUsers2",
			readOnlyConfig:            `super.users=User:CN=kafka-headless.kafka.svc.cluster.local`,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "none",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=none
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_with_readOnly-superUsers3",
			readOnlyConfig:            `super.users=User:CN=custom-superuser1;User:CN=custom-superuser2;User:CN=kafka-headless.kafka.svc.cluster.local`,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "none",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=none
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local;User:CN=custom-superuser1;User:CN=custom-superuser2
zookeeper.connect=example.zk:2181/`,
		},
		{
			testName:                  "configWithSSL_with_readOnly-superUsers4",
			readOnlyConfig:            `super.users=`,
			zkAddresses:               []string{"example.zk:2181"},
			zkPath:                    ``,
			kubernetesClusterDomain:   ``,
			clusterWideConfig:         ``,
			perBrokerConfig:           ``,
			perBrokerReadOnlyConfig:   ``,
			advertisedListenerAddress: `kafka-0.kafka.svc.cluster.local:9092`,
			listenerType:              "ssl",
			sslClientAuth:             "none",
			expectedConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client/keystore.jks
cruise.control.metrics.reporter.ssl.keystore.password=keystore_clientpassword123
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client/truststore.jks
cruise.control.metrics.reporter.ssl.truststore.password=keystore_clientpassword123
inter.broker.listener.name=INTERNAL
listener.name.internal.ssl.client.auth=none
listener.name.internal.ssl.keystore.location=/var/run/secrets/java.io/keystores/server/internal/keystore.jks
listener.name.internal.ssl.keystore.password=keystore_serverpassword123
listener.name.internal.ssl.keystore.type=JKS
listener.name.internal.ssl.truststore.location=/var/run/secrets/java.io/keystores/server/internal/truststore.jks
listener.name.internal.ssl.truststore.password=keystore_serverpassword123
listener.name.internal.ssl.truststore.type=JKS
listener.security.protocol.map=INTERNAL:SSL
listeners=INTERNAL://:9092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
super.users=User:CN=kafka-headless.kafka.svc.cluster.local
zookeeper.connect=example.zk:2181/`,
		},
	}

	t.Parallel()
	mockCtrl := gomock.NewController(t)

	for _, test := range tests {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			mockClient := mocks.NewMockClient(mockCtrl)
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			r := Reconciler{
				Reconciler: resources.Reconciler{
					Client: mockClient,
					KafkaCluster: &v1beta1.KafkaCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kafka",
							Namespace: "kafka",
						},
						Spec: v1beta1.KafkaClusterSpec{
							ZKAddresses: test.zkAddresses,
							ZKPath:      test.zkPath,
							ClientSSLCertSecret: &v1.LocalObjectReference{
								Name: "client-secret",
							},
							ListenersConfig: v1beta1.ListenersConfig{
								InternalListeners: []v1beta1.InternalListenerConfig{
									{
										CommonListenerSpec: v1beta1.CommonListenerSpec{
											Type:          v1beta1.SecurityProtocol(test.listenerType),
											Name:          "internal",
											ContainerPort: 9092,
											ServerSSLCertSecret: &v1.LocalObjectReference{
												Name: "server-secret",
											},
											SSLClientAuth: test.sslClientAuth,
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

			controllerListenerStatus := map[string]v1beta1.ListenerStatusList{
				"internal": {
					{
						Name:    "broker-0",
						Address: test.advertisedListenerAddress,
					},
				},
			}
			var (
				serverPasses map[string]string
				clientPass   string
				superUsers   []string
			)

			if strings.Contains(test.testName, "configWithSSL") {
				serverPasses = map[string]string{"internal": "keystore_serverpassword123"}
				clientPass = "keystore_clientpassword123"
				superUsers = []string{"CN=kafka-headless.kafka.svc.cluster.local"}
			}

			generatedConfig := r.generateBrokerConfig(r.KafkaCluster.Spec.Brokers[0], r.KafkaCluster.Spec.Brokers[0].BrokerConfig, nil, map[string]v1beta1.ListenerStatusList{},
				map[string]v1beta1.ListenerStatusList{}, controllerListenerStatus, serverPasses, clientPass, superUsers, logr.Discard())

			generated, err := properties.NewFromString(generatedConfig)
			if err != nil {
				t.Fatalf("failed parsing generated configuration as Properties: %s", generatedConfig)
			}

			expected, err := properties.NewFromString(test.expectedConfig)
			if err != nil {
				t.Fatalf("failed parsing expected configuration as Properties: %s", expected)
			}

			require.Equal(t, expected, generated)
		})
	}
}

// TestGenerateBrokerConfigKRaftMode serves as an aggregated test on top of TestGenerateBrokerConfig to verify basic broker configurations under KRaft mode
// Note: most of the test cases under TestGenerateBrokerConfig are not replicated here since running KRaft mode doesn't affect things like SSL and storage configurations
func TestGenerateBrokerConfigKRaftMode(t *testing.T) {
	testCases := []struct {
		testName                 string
		brokers                  []v1beta1.Broker
		listenersConfig          v1beta1.ListenersConfig
		internalListenerStatuses map[string]v1beta1.ListenerStatusList
		controllerListenerStatus map[string]v1beta1.ListenerStatusList
		expectedBrokerConfigs    []string
	}{
		{
			testName: "a Kafka cluster with a mix of broker-only and controller-only nodes; broker-only nodes with multiple mount paths",
			brokers: []v1beta1.Broker{
				{
					Id: 0,
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/test-kafka-logs",
							},
							{
								MountPath: "/test-kafka-logs-0",
							},
						},
					},
				},
				{
					Id: 500,
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/test-kafka-logs",
							},
						},
					},
				},
				{
					Id: 200,
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/test-kafka-logs",
							},
						},
					},
				},
			},
			listenersConfig: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:          v1beta1.SecurityProtocol("PLAINTEXT"),
							Name:          "internal",
							ContainerPort: 9092,
						},
						UsedForInnerBrokerCommunication: true,
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:          v1beta1.SecurityProtocol("PLAINTEXT"),
							Name:          "controller",
							ContainerPort: 9093,
						},
						UsedForControllerCommunication: true,
					},
				},
			},
			internalListenerStatuses: map[string]v1beta1.ListenerStatusList{
				"internal": {
					{
						Name:    "broker-0",
						Address: "kafka-0.kafka.svc.cluster.local:9092",
					},
					{
						Name:    "broker-500",
						Address: "kafka-500.kafka.svc.cluster.local:9092",
					},
					{
						Name:    "broker-200",
						Address: "kafka-200.kafka.svc.cluster.local:9092",
					},
				},
			},
			controllerListenerStatus: map[string]v1beta1.ListenerStatusList{
				"controller": {
					{
						Name:    "broker-0",
						Address: "kafka-0.kafka.svc.cluster.local:9093",
					},
					{
						Name:    "broker-500",
						Address: "kafka-500.kafka.svc.cluster.local:9093",
					},
					{
						Name:    "broker-200",
						Address: "kafka-200.kafka.svc.cluster.local:9093",
					},
				},
			},
			expectedBrokerConfigs: []string{
				`advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=500@kafka-500.kafka.svc.cluster.local:9093
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=INTERNAL://:9092
log.dirs=/test-kafka-logs/kafka,/test-kafka-logs-0/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
node.id=0
process.roles=broker
`,
				`controller.listener.names=CONTROLLER
controller.quorum.voters=500@kafka-500.kafka.svc.cluster.local:9093
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=CONTROLLER://:9093
log.dirs=/test-kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
node.id=500
process.roles=controller
`,
				`advertised.listeners=INTERNAL://kafka-200.kafka.svc.cluster.local:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=500@kafka-500.kafka.svc.cluster.local:9093
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=INTERNAL://:9092
log.dirs=/test-kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
node.id=200
process.roles=broker
`},
		},
		{
			testName: "a Kafka cluster with a mix of broker-only, controller-only, and combined roles; controller nodes with multiple mount paths",
			brokers: []v1beta1.Broker{
				{
					Id: 0,
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/test-kafka-logs",
							},
						},
					},
				},
				{
					Id: 50,
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/test-kafka-logs",
							},
							{
								MountPath: "/test-kafka-logs-50",
							},
						},
					},
				},
				{
					Id: 100,
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker", "controller"},
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/test-kafka-logs",
							},
							{
								MountPath: "/test-kafka-logs-50",
							},
							{
								MountPath: "/test-kafka-logs-100",
							},
						},
					},
				},
			},
			listenersConfig: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:          v1beta1.SecurityProtocol("PLAINTEXT"),
							Name:          "internal",
							ContainerPort: 9092,
						},
						UsedForInnerBrokerCommunication: true,
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:          v1beta1.SecurityProtocol("PLAINTEXT"),
							Name:          "controller",
							ContainerPort: 9093,
						},
						UsedForControllerCommunication: true,
					},
				},
			},
			internalListenerStatuses: map[string]v1beta1.ListenerStatusList{
				"internal": {
					{
						Name:    "broker-0",
						Address: "kafka-0.kafka.svc.cluster.local:9092",
					},
					{
						Name:    "broker-50",
						Address: "kafka-50.kafka.svc.cluster.local:9092",
					},
					{
						Name:    "broker-100",
						Address: "kafka-100.kafka.svc.cluster.local:9092",
					},
				},
			},
			controllerListenerStatus: map[string]v1beta1.ListenerStatusList{
				"controller": {
					{
						Name:    "broker-0",
						Address: "kafka-0.kafka.svc.cluster.local:9093",
					},
					{
						Name:    "broker-50",
						Address: "kafka-50.kafka.svc.cluster.local:9093",
					},
					{
						Name:    "broker-100",
						Address: "kafka-100.kafka.svc.cluster.local:9093",
					},
				},
			},
			expectedBrokerConfigs: []string{
				`advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=50@kafka-50.kafka.svc.cluster.local:9093,100@kafka-100.kafka.svc.cluster.local:9093
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=INTERNAL://:9092
log.dirs=/test-kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
node.id=0
process.roles=broker
`,
				`controller.listener.names=CONTROLLER
controller.quorum.voters=50@kafka-50.kafka.svc.cluster.local:9093,100@kafka-100.kafka.svc.cluster.local:9093
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=CONTROLLER://:9093
log.dirs=/test-kafka-logs/kafka,/test-kafka-logs-50/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
node.id=50
process.roles=controller
`,
				`advertised.listeners=INTERNAL://kafka-100.kafka.svc.cluster.local:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=50@kafka-50.kafka.svc.cluster.local:9093,100@kafka-100.kafka.svc.cluster.local:9093
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=INTERNAL://:9092,CONTROLLER://:9093
log.dirs=/test-kafka-logs/kafka,/test-kafka-logs-50/kafka,/test-kafka-logs-100/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
node.id=100
process.roles=broker,controller
`},
		},
	}

	t.Parallel()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			r := Reconciler{
				Reconciler: resources.Reconciler{
					Client: mockClient,
					KafkaCluster: &v1beta1.KafkaCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kafka",
							Namespace: "kafka",
						},
						Spec: v1beta1.KafkaClusterSpec{
							KRaftMode:       true,
							ListenersConfig: test.listenersConfig,
							Brokers:         test.brokers,
						},
					},
				},
			}

			for i, b := range test.brokers {
				quorumVoters, err := generateQuorumVoters(r.KafkaCluster, test.controllerListenerStatus)
				if err != nil {
					t.Error(err)
				}

				generatedConfig := r.generateBrokerConfig(b, b.BrokerConfig, quorumVoters, map[string]v1beta1.ListenerStatusList{},
					test.internalListenerStatuses, test.controllerListenerStatus, nil, "", nil, logr.Discard())

				require.Equal(t, test.expectedBrokerConfigs[i], generatedConfig)
			}
		})
	}
}
