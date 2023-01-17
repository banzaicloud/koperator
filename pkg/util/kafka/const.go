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

const (
	// AllBrokerServiceTemplate template for Kafka all broker service
	AllBrokerServiceTemplate = "%s-all-broker"
	// HeadlessServiceTemplate template for Kafka headless service
	HeadlessServiceTemplate = "%s-headless"
	// NodePortServiceTemplate template for Kafka nodeport service
	NodePortServiceTemplate = "%s-%d-%s"

	//ConfigPropertyName name in the ConfigMap's Data field for the broker configuration
	ConfigPropertyName = "broker-config"
)

// used for Kafka configurations
const (
	KafkaConfigSecurityProtocolMap     = "listener.security.protocol.map"
	KafkaConfigInterBrokerListenerName = "inter.broker.listener.name"
	KafkaConfigListeners               = "listeners"
	KafkaConfigAdvertisedListeners     = "advertised.listeners"
	KafkaConfigBoostrapServers         = "bootstrap.servers"
	KafkaConfigZooKeeperConnect        = "zookeeper.connect"
	KafkaConfigBrokerId                = "broker.id"
	KafkaConfigControlPlaneListener    = "control.plane.listener.name"
	KafkaConfigBrokerLogDirectory      = "log.dirs"
	KafkaConfigSuperUsers              = "super.users"

	KafkaConfigSecurityProtocol      = "security.protocol"
	KafkaConfigSSLTrustStoreType     = "ssl.truststore.type"
	KafkaConfigSSLTrustStoreLocation = "ssl.truststore.location"
	KafkaConfigSSLTrustStorePassword = "ssl.truststore.password"
	KafkaConfigSSLKeystoreType       = "ssl.keystore.type"
	KafkaConfigSSLKeyStoreLocation   = "ssl.keystore.location"
	KafkaConfigSSLKeyStorePassword   = "ssl.keystore.password"

	KafkaConfigSSLClientAuth = "ssl.client.auth"
)

// used for Cruise Control configurations
const (
	CruiseControlConfigMetricsReporters                 = "metric.reporters"
	CruiseControlConfigMetricsReportersBootstrapServers = "cruise.control.metrics.reporter.bootstrap.servers"
	CruiseControlConfigMetricsReporterK8sMode           = "cruise.control.metrics.reporter.kubernetes.mode"
)
