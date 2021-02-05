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
	"fmt"
	"sort"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	zookeeperutils "github.com/banzaicloud/kafka-operator/pkg/util/zookeeper"
	properties "github.com/banzaicloud/kafka-operator/properties/pkg"
)

func (r *Reconciler) getConfigProperties(bConfig *v1beta1.BrokerConfig, id int32,
	extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPass, clientPass string, superUsers []string, log logr.Logger) *properties.Properties {
	config := properties.NewProperties()

	// Add listener configuration
	listenerConf := generateListenerSpecificConfig(&r.KafkaCluster.Spec.ListenersConfig, log, id, extListenerStatuses)
	config.Merge(listenerConf)

	// Add listener configuration
	advertisedListenerConf := generateAdvertisedListenerConfig(id, r.KafkaCluster.Spec.ListenersConfig, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses)
	if len(advertisedListenerConf) > 0 {
		if err := config.Set("advertised.listeners", advertisedListenerConf); err != nil {
			log.Error(err, "setting advertised.listeners in broker configuration resulted an error")
		}
	}

	// Add control plane listener
	cclConf := generateControlPlaneListener(r.KafkaCluster.Spec.ListenersConfig.InternalListeners)
	if cclConf != "" {
		if err := config.Set("control.plane.listener.name", cclConf); err != nil {
			log.Error(err, "setting control.plane.listener.name parameter in broker configuration resulted an error")
		}
	}

	// Add Zookeeper configuration
	if err := config.Set("zookeeper.connect", zookeeperutils.PrepareConnectionAddress(r.KafkaCluster.Spec.ZKAddresses, r.KafkaCluster.Spec.GetZkPath())); err != nil {
		log.Error(err, "setting zookeeper.connect parameter in broker configuration resulted an error")
	}

	// Add SSL configuration
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil {
		if err := config.Set("ssl.keystore.location", serverKeystorePath+"/"+v1alpha1.TLSJKSKeyStore); err != nil {
			log.Error(err, "setting ssl.keystore.location parameter in broker configuration resulted an error")
		}
		if err := config.Set("ssl.truststore.location", serverKeystorePath+"/"+v1alpha1.TLSJKSTrustStore); err != nil {
			log.Error(err, "setting ssl.truststore.location parameter in broker configuration resulted an error")
		}
		if err := config.Set("ssl.keystore.password", serverPass); err != nil {
			log.Error(err, "setting ssl.keystore.password parameter in broker configuration resulted an error")
		}
		if err := config.Set("ssl.truststore.password", serverPass); err != nil {
			log.Error(err, "setting ssl.truststore.password parameter in broker configuration resulted an error")
		}
		if err := config.Set("ssl.client.auth", "required"); err != nil {
			log.Error(err, "setting ssl.client.auth parameter in broker configuration resulted an error")
		}

		// Add Cruise Control SSL configuration
		if util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners) {
			if err := config.Set("cruise.control.metrics.reporter.security.protocol", "SSL"); err != nil {
				log.Error(err, "setting cruise.control.metrics.reporter.security.protocol in broker configuration resulted an error")
			}
			if err := config.Set("cruise.control.metrics.reporter.ssl.truststore.location", clientKeystorePath+"/"+v1alpha1.TLSJKSTrustStore); err != nil {
				log.Error(err, "setting cruise.control.metrics.reporter.ssl.truststore.location in broker configuration resulted an error")
			}
			if err := config.Set("cruise.control.metrics.reporter.ssl.keystore.location", clientKeystorePath+"/"+v1alpha1.TLSJKSKeyStore); err != nil {
				log.Error(err, "setting cruise.control.metrics.reporter.ssl.keystore.location parameter in broker configuration resulted an error")
			}
			if err := config.Set("cruise.control.metrics.reporter.ssl.keystore.password", clientPass); err != nil {
				log.Error(err, "setting cruise.control.metrics.reporter.ssl.keystore.password parameter in broker configuration resulted an error")
			}
		}
	}

	// Add Cruise Control Metrics Reporter configuration
	if err := config.Set("metric.reporters", "com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter"); err != nil {
		log.Error(err, "setting metric.reporters in broker configuration resulted an error")
	}
	bootstrapServers, err := kafkautils.GetBootstrapServersService(r.KafkaCluster)
	if err != nil {
		log.Error(err, "getting Kafka bootstrap servers for Cruise Control failed")
	}
	if err := config.Set("cruise.control.metrics.reporter.bootstrap.servers", bootstrapServers); err != nil {
		log.Error(err, "setting cruise.control.metrics.reporter.bootstrap.servers in broker configuration resulted an error")
	}
	if err := config.Set("cruise.control.metrics.reporter.kubernetes.mode", true); err != nil {
		log.Error(err, "setting cruise.control.metrics.reporter.kubernetes.mode in broker configuration resulted an error")
	}

	// Kafka Broker configuration
	if err := config.Set("broker.id", id); err != nil {
		log.Error(err, "setting broker.id in broker configuration resulted an error")
	}

	// Storage configuration
	storageConf := generateStorageConfig(bConfig.StorageConfigs)
	if storageConf != "" {
		if err := config.Set("log.dirs", storageConf); err != nil {
			log.Error(err, "setting log.dirs in broker configuration resulted an error")
		}
	}

	// Add superuser configuration
	su := strings.Join(generateSuperUsers(superUsers), ";")
	if su != "" {
		if err := config.Set("super.users", su); err != nil {
			log.Error(err, "setting super.users in broker configuration resulted an error")
		}
	}
	return config
}

func generateSuperUsers(users []string) (suStrings []string) {
	suStrings = make([]string, 0)
	for _, x := range users {
		suStrings = append(suStrings, fmt.Sprintf("User:%s", x))
	}
	return
}

func (r *Reconciler) configMap(id int32, brokerConfig *v1beta1.BrokerConfig, extListenerStatuses,
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPass, clientPass string, superUsers []string, log logr.Logger) *corev1.ConfigMap {
	brokerConf := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id),
			util.MergeLabels(
				kafkautils.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{"brokerId": fmt.Sprintf("%d", id)},
			),
			r.KafkaCluster,
		),
		Data: map[string]string{kafkautils.ConfigPropertyName: r.generateBrokerConfig(id, brokerConfig, extListenerStatuses,
			intListenerStatuses, controllerIntListenerStatuses, serverPass, clientPass, superUsers, log)},
	}
	if brokerConfig.Log4jConfig != "" {
		brokerConf.Data["log4j.properties"] = brokerConfig.Log4jConfig
	}
	return brokerConf
}

func generateAdvertisedListenerConfig(id int32, l v1beta1.ListenersConfig,
	extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList) []string {
	advertisedListenerConfig := make([]string, 0, len(l.ExternalListeners)+len(l.InternalListeners))

	advertisedListenerConfig = appendListenerConfigs(advertisedListenerConfig, id, extListenerStatuses)
	advertisedListenerConfig = appendListenerConfigs(advertisedListenerConfig, id, intListenerStatuses)
	advertisedListenerConfig = appendListenerConfigs(advertisedListenerConfig, id, controllerIntListenerStatuses)

	return advertisedListenerConfig
}

func appendListenerConfigs(advertisedListenerConfig []string, id int32,
	listenerStatusList map[string]v1beta1.ListenerStatusList) []string {

	for listenerName, statuses := range listenerStatusList {
		for _, status := range statuses {
			if status.Name == fmt.Sprintf("broker-%d", id) {
				advertisedListenerConfig = append(advertisedListenerConfig,
					fmt.Sprintf("%s://%s", strings.ToUpper(listenerName), status.Address))
				break
			}
		}
	}
	//We have to sort this since we are using map to store listener statuses
	sort.Strings(advertisedListenerConfig)
	return advertisedListenerConfig
}

func generateStorageConfig(sConfig []v1beta1.StorageConfig) string {
	mountPaths := make([]string, 0, len(sConfig))
	for _, storage := range sConfig {
		mountPaths = append(mountPaths, storage.MountPath+`/kafka`)
	}
	return strings.Join(mountPaths, ",")
}

func generateControlPlaneListener(iListeners []v1beta1.InternalListenerConfig) string {
	controlPlaneListener := ""

	for _, iListener := range iListeners {
		if iListener.UsedForControllerCommunication {
			controlPlaneListener = strings.ToUpper(iListener.Name)
		}
	}

	return controlPlaneListener
}

func generateListenerSpecificConfig(l *v1beta1.ListenersConfig, log logr.Logger, id int32,
	extListenerStatuses map[string]v1beta1.ListenerStatusList) *properties.Properties {

	var interBrokerListenerName string
	var securityProtocolMapConfig []string
	var listenerConfig []string

	for _, iListener := range l.InternalListeners {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListenerName == "" {
				interBrokerListenerName = strings.ToUpper(iListener.Name)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
		UpperedListenerType := strings.ToUpper(iListener.Type)
		UpperedListenerName := strings.ToUpper(iListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, iListener.ContainerPort))
	}
	for _, eListener := range l.ExternalListeners {
		UpperedListenerType := strings.ToUpper(eListener.Type)
		UpperedListenerName := strings.ToUpper(eListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
	}

	config := properties.NewProperties()
	if err := config.Set("listener.security.protocol.map", securityProtocolMapConfig); err != nil {
		log.Error(err, "setting listener.security.protocol.map parameter in broker configuration resulted an error")
	}
	if err := config.Set("inter.broker.listener.name", interBrokerListenerName); err != nil {
		log.Error(err, "setting inter.broker.listener.name parameter in broker configuration resulted an error")
	}
	if err := config.Set("listeners", listenerConfig); err != nil {
		log.Error(err, "setting listeners parameter in broker configuration resulted an error")
	}

	return config
}

func (r Reconciler) generateBrokerConfig(id int32, brokerConfig *v1beta1.BrokerConfig, extListenerStatuses,
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPass, clientPass string, superUsers []string, log logr.Logger) string {
	// Parse cluster-wide readonly configuration
	finalBrokerConfig, err := properties.NewFromString(r.KafkaCluster.Spec.ReadOnlyConfig)
	if err != nil {
		log.Error(err, "failed to parse readonly cluster configuration")
	}

	// Parse readonly broker configuration
	var parsedReadOnlyBrokerConfig *properties.Properties
	// Find configuration for broker with id
	for _, broker := range r.KafkaCluster.Spec.Brokers {
		if broker.Id == id {
			parsedReadOnlyBrokerConfig, err = properties.NewFromString(broker.ReadOnlyConfig)
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to parse readonly broker configuration for broker with id: %d", id))
			}
			break
		}
	}

	// Merge cluster-wide configuration into broker-level configuration
	if parsedReadOnlyBrokerConfig != nil {
		finalBrokerConfig.Merge(parsedReadOnlyBrokerConfig)
	}

	// Get operator generated configuration
	opGenConf := r.getConfigProperties(brokerConfig, id, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPass, clientPass, superUsers, log)

	// Merge operator generated configuration to the final one
	if opGenConf != nil {
		finalBrokerConfig.Merge(opGenConf)
	}

	finalBrokerConfig.Sort()

	return finalBrokerConfig.String()
}
