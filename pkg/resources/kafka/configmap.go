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
	"context"
	"fmt"
	"sort"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"

	apiutil "github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
	zookeeperutils "github.com/banzaicloud/koperator/pkg/util/zookeeper"
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

const brokerLogDirPropertyName = "log.dirs"

func (r *Reconciler) getConfigProperties(bConfig *v1beta1.BrokerConfig, id int32,
	extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPasses map[string]string, clientPass string, superUsers []string, log logr.Logger) *properties.Properties {
	config := properties.NewProperties()

	// Add listener configuration
	listenerConf := generateListenerSpecificConfig(&r.KafkaCluster.Spec.ListenersConfig, serverPasses, log)
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

	// Add Cruise Control SSL configuration
	if util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners) {
		if !r.KafkaCluster.Spec.IsClientSSLSecretPresent() {
			log.Error(errors.New("cruise control metrics reporter needs ssl but client certificate hasn't specified"), "")
		}
		if err := config.Set("cruise.control.metrics.reporter.security.protocol", "SSL"); err != nil {
			log.Error(err, "setting cruise.control.metrics.reporter.security.protocol in broker configuration resulted an error")
		}
		if err := config.Set("cruise.control.metrics.reporter.ssl.truststore.location", clientKeystorePath+"/"+v1alpha1.TLSJKSTrustStore); err != nil {
			log.Error(err, "setting cruise.control.metrics.reporter.ssl.truststore.location in broker configuration resulted an error")
		}
		if err := config.Set("cruise.control.metrics.reporter.ssl.truststore.password", clientPass); err != nil {
			log.Error(err, "setting cruise.control.metrics.reporter.ssl.truststore.password parameter in broker configuration resulted an error")
		}
		if err := config.Set("cruise.control.metrics.reporter.ssl.keystore.location", clientKeystorePath+"/"+v1alpha1.TLSJKSKeyStore); err != nil {
			log.Error(err, "setting cruise.control.metrics.reporter.ssl.keystore.location parameter in broker configuration resulted an error")
		}
		if err := config.Set("cruise.control.metrics.reporter.ssl.keystore.password", clientPass); err != nil {
			log.Error(err, "setting cruise.control.metrics.reporter.ssl.keystore.password parameter in broker configuration resulted an error")
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

	// This logic prevents the removal of the mountPath from the broker configmap
	brokerConfigMapName := fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id)
	var brokerConfigMapOld v1.ConfigMap
	err = r.Client.Get(context.Background(), client.ObjectKey{Name: brokerConfigMapName, Namespace: r.KafkaCluster.GetNamespace()}, &brokerConfigMapOld)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "getting broker configmap from the Kubernetes API server resulted an error")
	}

	mountPathsOld := getMountPathsFromBrokerConfigMap(&brokerConfigMapOld)
	mountPathsNew := generateStorageConfig(bConfig.StorageConfigs)
	mountPathsMerged, isMountPathRemoved := mergeMountPaths(mountPathsOld, mountPathsNew)

	if isMountPathRemoved {
		log.Error(errors.New("removing storage from a running broker is not supported"), "", "brokerID", id)
	}

	if len(mountPathsMerged) != 0 {
		if err := config.Set(brokerLogDirPropertyName, strings.Join(mountPathsMerged, ",")); err != nil {
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

// mergeMountPaths is merges the new mountPaths with the old.
// It returns the merged []string and a bool which true or false depend on mountPathsNew contains or not all of the elements of the mountPathsOld
func mergeMountPaths(mountPathsOld, mountPathsNew []string) ([]string, bool) {
	var mountPathsMerged []string
	mountPathsMerged = append(mountPathsMerged, mountPathsOld...)
	mountPathsOldLen := len(mountPathsOld)
	// Merging the new mountPaths with the old. If any of them is removed we can check the difference in the mountPathsOldLen
	for i := range mountPathsNew {
		found := false
		for k := range mountPathsOld {
			if mountPathsOld[k] == mountPathsNew[i] {
				found = true
				mountPathsOldLen--
				break
			}
		}
		// if this is a new mountPath then add it to te current
		if !found {
			mountPathsMerged = append(mountPathsMerged, mountPathsNew[i])
		}
	}
	// If any of them is removed we can check the difference in the mountPathsOldLen
	isMountPathRemoved := false
	if mountPathsOldLen > 0 {
		isMountPathRemoved = true
	}

	return mountPathsMerged, isMountPathRemoved
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
	serverPasses map[string]string, clientPass string, superUsers []string, log logr.Logger) *corev1.ConfigMap {
	brokerConf := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id),
			apiutil.MergeLabels(
				apiutil.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{"brokerId": fmt.Sprintf("%d", id)},
			),
			r.KafkaCluster,
		),
		Data: map[string]string{kafkautils.ConfigPropertyName: r.generateBrokerConfig(id, brokerConfig, extListenerStatuses,
			intListenerStatuses, controllerIntListenerStatuses, serverPasses, clientPass, superUsers, log)},
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

func getMountPathsFromBrokerConfigMap(configMap *v1.ConfigMap) []string {
	if configMap == nil {
		return nil
	}
	brokerConfig := configMap.Data[kafkautils.ConfigPropertyName]
	brokerConfigsLines := strings.Split(brokerConfig, "\n")
	var mountPaths string
	for i := range brokerConfigsLines {
		keyVal := strings.Split(brokerConfigsLines[i], "=")
		if len(keyVal) == 2 && keyVal[0] == brokerLogDirPropertyName {
			mountPaths = keyVal[1]
		}
	}
	return strings.Split(mountPaths, ",")
}

func generateStorageConfig(sConfig []v1beta1.StorageConfig) []string {
	mountPaths := make([]string, 0, len(sConfig))
	for _, storage := range sConfig {
		mountPaths = append(mountPaths, util.StorageConfigKafkaMountPath(storage.MountPath))
	}
	return mountPaths
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

func generateListenerSpecificConfig(l *v1beta1.ListenersConfig, serverPasses map[string]string, log logr.Logger) *properties.Properties {
	var (
		interBrokerListenerName   string
		securityProtocolMapConfig []string
		listenerConfig            []string
	)

	config := properties.NewProperties()

	for _, iListener := range l.InternalListeners {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListenerName == "" {
				interBrokerListenerName = strings.ToUpper(iListener.Name)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
		UpperedListenerType := iListener.Type.ToUpperString()
		UpperedListenerName := strings.ToUpper(iListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, iListener.ContainerPort))
		// Add internal listeners SSL configuration
		if iListener.Type == v1beta1.SecurityProtocolSSL {
			generateListenerSSLConfig(config, iListener.Name, iListener.SSLClientAuth, "JKS", serverPasses, log)
		}
	}

	for _, eListener := range l.ExternalListeners {
		UpperedListenerType := eListener.Type.ToUpperString()
		UpperedListenerName := strings.ToUpper(eListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
		// Add external listeners SSL configuration
		if eListener.Type == v1beta1.SecurityProtocolSSL {
			generateListenerSSLConfig(config, eListener.Name, eListener.SSLClientAuth, "JKS", serverPasses, log)
		}
	}
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

func generateListenerSSLConfig(config *properties.Properties, name string, sslClientAuth v1beta1.SSLClientAuthentication, certificateStoreType string, passwordKeyMap map[string]string, log logr.Logger) {
	if certificateStoreType == "JKS" {
		namedKeystorePath := fmt.Sprintf(listenerServerKeyStorePathTemplate, serverKeystorePath, name)
		listenerSSLConfig := map[string]string{
			fmt.Sprintf(`listener.name.%s.ssl.keystore.location`, name):   namedKeystorePath + "/" + v1alpha1.TLSJKSKeyStore,
			fmt.Sprintf("listener.name.%s.ssl.truststore.location", name): namedKeystorePath + "/" + v1alpha1.TLSJKSTrustStore,
			fmt.Sprintf("listener.name.%s.ssl.keystore.password", name):   passwordKeyMap[name],
			fmt.Sprintf("listener.name.%s.ssl.truststore.password", name): passwordKeyMap[name],
			fmt.Sprintf("listener.name.%s.ssl.keystore.type", name):       "JKS",
			fmt.Sprintf("listener.name.%s.ssl.truststore.type", name):     "JKS",
		}

		// enable 2-way SSL authentication if SSL is enabled but this field is not provided in the listener config
		if sslClientAuth == "" {
			listenerSSLConfig[fmt.Sprintf("listener.name.%s.ssl.client.auth", name)] = string(v1beta1.SSLClientAuthRequired)
		} else {
			listenerSSLConfig[fmt.Sprintf("listener.name.%s.ssl.client.auth", name)] = string(sslClientAuth)
		}

		for k, v := range listenerSSLConfig {
			if err := config.Set(k, v); err != nil {
				log.Error(err, fmt.Sprintf("setting %s parameter in broker configuration resulted an error", k))
			}
		}
	}
}

func (r Reconciler) generateBrokerConfig(id int32, brokerConfig *v1beta1.BrokerConfig, extListenerStatuses,
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPasses map[string]string, clientPass string, superUsers []string, log logr.Logger) string {
	finalBrokerConfig := getBrokerReadOnlyConfig(id, r.KafkaCluster, log)

	// Get operator generated configuration
	opGenConf := r.getConfigProperties(brokerConfig, id, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPasses, clientPass, superUsers, log)

	// Merge operator generated configuration to the final one
	if opGenConf != nil {
		finalBrokerConfig.Merge(opGenConf)
	}

	finalBrokerConfig.Sort()

	return finalBrokerConfig.String()
}

// TODO move this into api in the future (adamantal)
func getBrokerReadOnlyConfig(id int32, kafkaCluster *v1beta1.KafkaCluster, log logr.Logger) *properties.Properties {
	// Parse cluster-wide readonly configuration
	finalBrokerConfig, err := properties.NewFromString(kafkaCluster.Spec.ReadOnlyConfig)
	if err != nil {
		log.Error(err, "failed to parse readonly cluster configuration")
	}

	// Parse readonly broker configuration
	var parsedReadOnlyBrokerConfig *properties.Properties
	// Find configuration for broker with id
	for _, broker := range kafkaCluster.Spec.Brokers {
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

	return finalBrokerConfig
}
