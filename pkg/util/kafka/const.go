// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

// ConfigPropertyName name in the ConfigMap's Data field for the broker configuration
const ConfigPropertyName = "broker-config"

// used for Kafka configurations
const (
	KafkaConfigSuperUsers = "super.users"

	KafkaConfigBoostrapServers  = "bootstrap.servers"
	KafkaConfigZooKeeperConnect = "zookeeper.connect"
	// KafkaConfigBrokerID is used in ZooKeeper mode
	KafkaConfigBrokerID           = "broker.id"
	KafkaConfigBrokerLogDirectory = "log.dirs"

	// Configuration keys for KRaft
	KafkaConfigNodeID                 = "node.id"
	KafkaConfigProcessRoles           = "process.roles"
	KafkaConfigControllerQuorumVoters = "controller.quorum.voters"
	KafkaConfigControllerListenerName = "controller.listener.names"

	KafkaConfigListeners                   = "listeners"
	KafkaConfigListenerName                = "listener.name"
	KafkaConfigListenerSecurityProtocolMap = "listener.security.protocol.map"
	KafkaConfigInterBrokerListenerName     = "inter.broker.listener.name"
	KafkaConfigAdvertisedListeners         = "advertised.listeners"
	KafkaConfigControlPlaneListener        = "control.plane.listener.name"

	KafkaConfigSecurityProtocol      = "security.protocol"
	KafkaConfigSSLClientAuth         = "ssl.client.auth"
	KafkaConfigSSLTrustStoreType     = "ssl.truststore.type"
	KafkaConfigSSLTrustStoreLocation = "ssl.truststore.location"
	KafkaConfigSSLTrustStorePassword = "ssl.truststore.password"
	KafkaConfigSSLKeystoreType       = "ssl.keystore.type"
	KafkaConfigSSLKeyStoreLocation   = "ssl.keystore.location"
	KafkaConfigSSLKeyStorePassword   = "ssl.keystore.password"
)

// used for Cruise Control configurations
const (
	CruiseControlConfigMetricsReporters                  = "metric.reporters"
	CruiseControlConfigMetricsReportersBootstrapServers  = "cruise.control.metrics.reporter.bootstrap.servers"
	CruiseControlConfigMetricsReporterK8sMode            = "cruise.control.metrics.reporter.kubernetes.mode"
	CruiseControlConfigTopicConfigProviderClass          = "topic.config.provider.class"
	CruiseControlConfigKafkaBrokerFailureDetectionEnable = "kafka.broker.failure.detection.enable"

	CruiseControlConfigMetricsReportersVal                  = "com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter"
	CruiseControlConfigTopicConfigProviderClassVal          = "com.linkedin.kafka.cruisecontrol.config.KafkaAdminTopicConfigProvider"
	CruiseControlConfigKafkaBrokerFailureDetectionEnableVal = "true"
)
