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
	serverPasses map[string]string, clientPass string, superUsers []string, serverJKSFormatCert map[string]bool, clientJKSFormatCert bool, log logr.Logger) *properties.Properties {
	config := properties.NewProperties()

	// Add listener configuration
	listenerConf := generateListenerSpecificConfig(&r.KafkaCluster.Spec.ListenersConfig, serverPasses, serverJKSFormatCert, log)
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
		keyStoreType := "JKS"
		keyStoreLoc := clientKeystorePath + "/" + v1alpha1.TLSJKSKeyStore
		trustStoreType := "JKS"
		trustStoreLoc := clientKeystorePath + "/" + v1alpha1.TLSJKSTrustStore
		if !clientJKSFormatCert {
			keyStoreType = "PEM"
			trustStoreType = "PEM"
			trustStoreLoc = clientKeystorePath + "/" + v1alpha1.TLSPEMTrustStore
			keyStoreLoc = clientKeystorePath + "/" + v1alpha1.TLSPEMKeyStore
		}

		sslConfig := map[string]string{
			"security.protocol":       "SSL",
			"ssl.truststore.type":     trustStoreType,
			"ssl.keystore.type":       keyStoreType,
			"ssl.truststore.location": trustStoreLoc,
			"ssl.keystore.location":   keyStoreLoc,
		}
		if clientJKSFormatCert {
			sslConfig["ssl.keystore.password"] = clientPass
			sslConfig["ssl.truststore.password"] = clientPass
		} else {
			sslConfig["ssl.key.password"] = ""
		}

		for k, v := range sslConfig {
			if err := config.Set(fmt.Sprintf("cruise.control.metrics.reporter.%s", k), v); err != nil {
				log.Error(err, fmt.Sprintf("setting cruise.control.metrics.reporter.%s parameter in broker configuration resulted an error", k))
			}
		}
		if err := config.Set("cruise.control.metrics.reporter.security.protocol", "SSL"); err != nil {
			log.Error(err, "setting cruise.control.metrics.reporter.security.protocol in broker configuration resulted an error")
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

	mountPathsOld, err := getMountPathsFromBrokerConfigMap(&brokerConfigMapOld)
	if err != nil {
		log.Error(err, "could not get mounthPaths from broker configmap", v1beta1.BrokerIdLabelKey, id)
	}
	mountPathsNew := generateStorageConfig(bConfig.StorageConfigs)
	mountPathsMerged, isMountPathRemoved := mergeMountPaths(mountPathsOld, mountPathsNew)

	if isMountPathRemoved {
		log.Error(errors.New("removed storage is found in the KafkaCluster CR"), "removing storage from broker is not supported", v1beta1.BrokerIdLabelKey, id, "mountPaths", mountPathsOld, "mountPaths in kafkaCluster CR ", mountPathsNew)
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
	mountPathsMerged = append(mountPathsMerged, mountPathsNew...)
	isMountPathRemoved := false
	// Merging the new mountPaths with the old. If any of them is removed we can check the difference in the mountPathsOldLen
	for i := range mountPathsOld {
		found := false
		for k := range mountPathsNew {
			if mountPathsOld[i] == mountPathsNew[k] {
				found = true
				break
			}
		}
		// if this is a new mountPath then add it to the current
		if !found {
			mountPathsMerged = append(mountPathsMerged, mountPathsOld[i])
			isMountPathRemoved = true
		}
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
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList, log logr.Logger) (*corev1.ConfigMap, error) {

	// We need to grab names for servers and client in case user is enabling ACLs
	// That way we can continue to manage topics and users
	// We also need to greb server PEM format certificates when JKS is not available

	clientPass, serverPasses, superUsers, serverJKSFormatCert, clientKSFormatCert, err := r.getPasswordKeysSuperUsersAndFormat()
	if err != nil {
		return nil, err
	}

	brokerConf := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id),
			apiutil.MergeLabels(
				apiutil.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{v1beta1.BrokerIdLabelKey: fmt.Sprintf("%d", id)},
			),
			r.KafkaCluster,
		),
		Data: map[string]string{kafkautils.ConfigPropertyName: r.generateBrokerConfig(id, brokerConfig, extListenerStatuses,
			intListenerStatuses, controllerIntListenerStatuses, serverPasses, clientPass, superUsers, serverJKSFormatCert, clientKSFormatCert, log)},
	}
	if brokerConfig.Log4jConfig != "" {
		brokerConf.Data["log4j.properties"] = brokerConfig.Log4jConfig
	}
	return brokerConf, nil
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

func getMountPathsFromBrokerConfigMap(configMap *v1.ConfigMap) ([]string, error) {
	if configMap == nil {
		return nil, nil
	}
	brokerConfig := configMap.Data[kafkautils.ConfigPropertyName]
	brokerConfigProperties, err := properties.NewFromString(brokerConfig)
	if err != nil {
		return nil, err
	}
	brokerLogDirProperty, found := brokerConfigProperties.Get(brokerLogDirPropertyName)
	if !found || brokerLogDirProperty.Value() == "" {
		return nil, nil
	}

	return strings.Split(brokerLogDirProperty.Value(), ","), nil
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

func generateListenerSpecificConfig(l *v1beta1.ListenersConfig, serverPasses map[string]string, jksFormatCert map[string]bool, log logr.Logger) *properties.Properties {
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
			generateListenerSSLConfig(config, iListener.Name, iListener.SSLClientAuth, serverPasses[iListener.Name], jksFormatCert[iListener.Name], log)
		}
	}

	for _, eListener := range l.ExternalListeners {
		UpperedListenerType := eListener.Type.ToUpperString()
		UpperedListenerName := strings.ToUpper(eListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
		// Add external listeners SSL configuration
		if eListener.Type == v1beta1.SecurityProtocolSSL {
			generateListenerSSLConfig(config, eListener.Name, eListener.SSLClientAuth, serverPasses[eListener.Name], jksFormatCert[eListener.Name], log)
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

func generateListenerSSLConfig(config *properties.Properties, name string, sslClientAuth v1beta1.SSLClientAuthentication, password string, jksFormatCert bool, log logr.Logger) {
	var listenerSSLConfig map[string]string
	namedKeystorePath := fmt.Sprintf(listenerServerKeyStorePathTemplate, serverKeystorePath, name)
	keyStoreType := "JKS"
	keyStoreLoc := namedKeystorePath + "/" + v1alpha1.TLSJKSKeyStore
	trustStoreType := "JKS"
	trustStoreLoc := namedKeystorePath + "/" + v1alpha1.TLSJKSTrustStore
	if !jksFormatCert {
		keyStoreType = "PEM"
		trustStoreType = "PEM"
		trustStoreLoc = namedKeystorePath + "/" + v1alpha1.TLSPEMTrustStore
		keyStoreLoc = namedKeystorePath + "/" + v1alpha1.TLSPEMKeyStore
	}

	listenerSSLConfig = map[string]string{
		fmt.Sprintf(`listener.name.%s.ssl.keystore.location`, name):   keyStoreLoc,
		fmt.Sprintf("listener.name.%s.ssl.truststore.location", name): trustStoreLoc,
		fmt.Sprintf("listener.name.%s.ssl.keystore.type", name):       keyStoreType,
		fmt.Sprintf("listener.name.%s.ssl.truststore.type", name):     trustStoreType,
	}

	if jksFormatCert {
		listenerSSLConfig[fmt.Sprintf("listener.name.%s.ssl.truststore.password", name)] = password
		listenerSSLConfig[fmt.Sprintf("listener.name.%s.ssl.keystore.password", name)] = password
	} else {
		listenerSSLConfig[fmt.Sprintf("listener.name.%s.ssl.key.password", name)] = ""
	}

	// listenerSSLConfig = map[string]string{
	// 	fmt.Sprintf(`listener.name.%s.ssl.keystore.certificate.chain`, name): serverCertsPEM[name][corev1.TLSCertKey],
	// 	fmt.Sprintf("listener.name.%s.ssl.keystore.key", name):               serverCertsPEM[name][corev1.TLSPrivateKeyKey],
	// 	//	fmt.Sprintf("listener.name.%s.ssl.key.password", name):               passwordKeyMap[name],
	// 	fmt.Sprintf("listener.name.%s.ssl.truststore.certificates", name): serverCertsPEM[name][v1alpha1.CoreCACertKey],
	// 	fmt.Sprintf("listener.name.%s.ssl.keystore.type", name):           "PEM",
	// 	fmt.Sprintf("listener.name.%s.ssl.truststore.type", name):         "PEM",
	// }

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

func (r Reconciler) generateBrokerConfig(id int32, brokerConfig *v1beta1.BrokerConfig, extListenerStatuses,
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPasses map[string]string, clientPass string, superUsers []string, serverJKSFormatCert map[string]bool, clientJKSFormatCert bool, log logr.Logger) string {
	finalBrokerConfig := getBrokerReadOnlyConfig(id, r.KafkaCluster, log)

	// Get operator generated configuration
	opGenConf := r.getConfigProperties(brokerConfig, id, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPasses, clientPass, superUsers, serverJKSFormatCert, clientJKSFormatCert, log)

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
