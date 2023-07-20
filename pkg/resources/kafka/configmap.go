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
	"context"
	"fmt"
	"sort"
	"strings"

	"emperror.dev/errors"
	zookeeperutils "github.com/banzaicloud/koperator/pkg/util/zookeeper"
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
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

func (r *Reconciler) getConfigProperties(bConfig *v1beta1.BrokerConfig, broker v1beta1.Broker,
	extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPasses map[string]string, clientPass string, superUsers []string, log logr.Logger) *properties.Properties {
	config := properties.NewProperties()

	bootstrapServers, err := kafkautils.GetBootstrapServersService(r.KafkaCluster)
	if err != nil {
		log.Error(err, "getting Kafka bootstrap servers for Cruise Control failed")
	}

	// Cruise Control metrics reporter configuration
	configCCMetricsReporter(r.KafkaCluster, config, clientPass, bootstrapServers, log)

	// Kafka Broker configurations
	if r.KafkaCluster.Spec.KraftMode() {
		configureBrokerKRaftMode(broker, r.KafkaCluster, config, bootstrapServers, serverPasses, extListenerStatuses,
			intListenerStatuses, controllerIntListenerStatuses, log)
	} else {
		configureBrokerZKMode(broker.Id, r.KafkaCluster, config, serverPasses, extListenerStatuses,
			intListenerStatuses, controllerIntListenerStatuses, log)
	}

	// This logic prevents the removal of the mountPath from the broker configmap
	brokerConfigMapName := fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, broker.Id)
	var brokerConfigMapOld v1.ConfigMap
	err = r.Client.Get(context.Background(), client.ObjectKey{Name: brokerConfigMapName, Namespace: r.KafkaCluster.GetNamespace()}, &brokerConfigMapOld)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "getting broker configmap from the Kubernetes API server resulted an error")
	}

	mountPathsOld, err := getMountPathsFromBrokerConfigMap(&brokerConfigMapOld)
	if err != nil {
		log.Error(err, "could not get mountPaths from broker configmap", v1beta1.BrokerIdLabelKey, broker.Id)
	}

	mountPathsNew := generateStorageConfig(bConfig.StorageConfigs)
	mountPathsMerged, isMountPathRemoved := mergeMountPaths(mountPathsOld, mountPathsNew)

	if isMountPathRemoved {
		log.Error(errors.New("removed storage is found in the KafkaCluster CR"),
			"removing storage from broker is not supported", v1beta1.BrokerIdLabelKey, broker.Id, "mountPaths",
			mountPathsOld, "mountPaths in kafkaCluster CR ", mountPathsNew)
	}

	if len(mountPathsMerged) != 0 {
		if err := config.Set(kafkautils.KafkaConfigBrokerLogDirectory, strings.Join(mountPathsMerged, ",")); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigBrokerLogDirectory))
		}
	}

	// Add superuser configuration
	su := strings.Join(generateSuperUsers(superUsers), ";")
	if su != "" {
		if err := config.Set(kafkautils.KafkaConfigSuperUsers, su); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigSuperUsers))
		}
	}
	return config
}

func configCCMetricsReporter(kafkaCluster *v1beta1.KafkaCluster, config *properties.Properties, clientPass, bootstrapServers string, log logr.Logger) {
	// Add Cruise Control Metrics Reporter SSL configuration
	if util.IsSSLEnabledForInternalCommunication(kafkaCluster.Spec.ListenersConfig.InternalListeners) {
		if !kafkaCluster.Spec.IsClientSSLSecretPresent() {
			log.Error(errors.New("cruise control metrics reporter needs ssl but client certificate hasn't specified"), "")
		}

		keyStoreLoc := clientKeystorePath + "/" + v1alpha1.TLSJKSKeyStore
		trustStoreLoc := clientKeystorePath + "/" + v1alpha1.TLSJKSTrustStore

		sslConfig := map[string]string{
			kafkautils.KafkaConfigSecurityProtocol:      "SSL",
			kafkautils.KafkaConfigSSLTrustStoreLocation: trustStoreLoc,
			kafkautils.KafkaConfigSSLKeyStoreLocation:   keyStoreLoc,
			kafkautils.KafkaConfigSSLKeyStorePassword:   clientPass,
			kafkautils.KafkaConfigSSLTrustStorePassword: clientPass,
		}

		for k, v := range sslConfig {
			if err := config.Set(fmt.Sprintf("cruise.control.metrics.reporter.%s", k), v); err != nil {
				log.Error(err, fmt.Sprintf("setting cruise.control.metrics.reporter.%s parameter in broker configuration resulted in an error", k))
			}
		}
	}

	// Add Cruise Control Metrics Reporter configuration
	if err := config.Set(kafkautils.CruiseControlConfigMetricsReporters, kafkautils.CruiseControlConfigMetricsReportersVal); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.CruiseControlConfigMetricsReporters))
	}
	if err := config.Set(kafkautils.CruiseControlConfigMetricsReportersBootstrapServers, bootstrapServers); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.CruiseControlConfigMetricsReportersBootstrapServers))
	}
	if err := config.Set(kafkautils.CruiseControlConfigMetricsReporterK8sMode, true); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.CruiseControlConfigMetricsReporterK8sMode))
	}
}

func configureBrokerKRaftMode(broker v1beta1.Broker, kafkaCluster *v1beta1.KafkaCluster, config *properties.Properties,
	bootstrapServers string, serverPasses map[string]string, extListenerStatuses, intListenerStatuses,
	controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList, log logr.Logger) {
	if err := config.Set(kafkautils.KafkaConfigNodeID, broker.Id); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigNodeID))
	}

	if err := config.Set(kafkautils.KafkaConfigProcessRoles, broker.Roles); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigProcessRoles))
	}

	if err := config.Set(kafkautils.KafkaConfigControllerQuorumVoters, generateQuorumVoters(kafkaCluster.Spec.Brokers, controllerIntListenerStatuses)); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigControllerQuorumVoters))
	}

	controllerListenerName := generateControlPlaneListener(kafkaCluster.Spec.ListenersConfig.InternalListeners)
	if controllerListenerName != "" {
		if err := config.Set(kafkautils.KafkaConfigControllerListenerName, controllerListenerName); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigControllerListenerName))
		}
	}

	// Add listener configuration
	listenerConf, listenerConfig := generateListenerSpecificConfig(&kafkaCluster.Spec.ListenersConfig, serverPasses, log)
	config.Merge(listenerConf)

	var advertisedListenerConf []string

	// expose controller listener in the listeners configuration only if this broker is a controller
	// also expose only controller listener as the advertised.listeners configuration when this broker is a controller
	if !isControllerNode(broker.Roles) {
		advertisedListenerConf = generateAdvertisedListenerConfig(broker.Id, kafkaCluster.Spec.ListenersConfig,
			extListenerStatuses, intListenerStatuses, nil)
		if err := config.Set(kafkautils.KafkaConfigAdvertisedListeners, advertisedListenerConf); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigAdvertisedListeners))
		}
	}

	// "listeners" configuration can only contain controller configuration when the node is a controller-only node
	if isControllerNodeOnly(broker.Roles) {
		for _, listener := range listenerConfig {
			if listener[:len(controllerListenerName)] == strings.ToUpper(controllerListenerName) {
				if err := config.Set(kafkautils.KafkaConfigListeners, listener); err != nil {
					log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigListeners))
				}
				break
			}
		}
	}

	// "listeners" configuration cannot contain controller configuration when the node is a broker-only node
	var nonControllerListener []string
	if isBrokerNodeOnly(broker.Roles) {
		for _, listener := range listenerConfig {
			if listener[:len(controllerListenerName)] != strings.ToUpper(controllerListenerName) {
				nonControllerListener = append(nonControllerListener, listener)
			}
		}
		if err := config.Set(kafkautils.KafkaConfigListeners, nonControllerListener); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigListeners))
		}
	}
}

func configureBrokerZKMode(brokerID int32, kafkaCluster *v1beta1.KafkaCluster, config *properties.Properties,
	serverPasses map[string]string, extListenerStatuses, intListenerStatuses,
	controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList, log logr.Logger) {
	if err := config.Set(kafkautils.KafkaConfigBrokerID, brokerID); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigBrokerID))
	}

	// Add listener configuration
	listenerConf, _ := generateListenerSpecificConfig(&kafkaCluster.Spec.ListenersConfig, serverPasses, log)
	config.Merge(listenerConf)

	// Add advertised listener configuration
	advertisedListenerConf := generateAdvertisedListenerConfig(brokerID, kafkaCluster.Spec.ListenersConfig,
		extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses)
	if len(advertisedListenerConf) > 0 {
		if err := config.Set(kafkautils.KafkaConfigAdvertisedListeners, advertisedListenerConf); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigAdvertisedListeners))
		}
	}

	// Add control plane listener
	cclConf := generateControlPlaneListener(kafkaCluster.Spec.ListenersConfig.InternalListeners)
	if cclConf != "" {
		if err := config.Set(kafkautils.KafkaConfigControlPlaneListener, cclConf); err != nil {
			log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigControlPlaneListener))
		}
	}

	// Add Zookeeper configuration
	if err := config.Set(kafkautils.KafkaConfigZooKeeperConnect, zookeeperutils.PrepareConnectionAddress(
		kafkaCluster.Spec.ZKAddresses, kafkaCluster.Spec.GetZkPath())); err != nil {
		log.Error(err, fmt.Sprintf(kafkautils.BrokerConfigErrorMsgTemplate, kafkautils.KafkaConfigZooKeeperConnect))
	}
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

func (r *Reconciler) configMap(broker v1beta1.Broker, brokerConfig *v1beta1.BrokerConfig, extListenerStatuses,
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPasses map[string]string, clientPass string, superUsers []string, log logr.Logger) *corev1.ConfigMap {
	brokerConf := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, broker.Id),
			apiutil.MergeLabels(
				apiutil.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{v1beta1.BrokerIdLabelKey: fmt.Sprintf("%d", broker.Id)},
			),
			r.KafkaCluster,
		),
		Data: map[string]string{kafkautils.ConfigPropertyName: r.generateBrokerConfig(broker, brokerConfig, extListenerStatuses,
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

func getMountPathsFromBrokerConfigMap(configMap *v1.ConfigMap) ([]string, error) {
	if configMap == nil {
		return nil, nil
	}
	brokerConfig := configMap.Data[kafkautils.ConfigPropertyName]
	brokerConfigProperties, err := properties.NewFromString(brokerConfig)
	if err != nil {
		return nil, err
	}
	brokerLogDirProperty, found := brokerConfigProperties.Get(kafkautils.KafkaConfigBrokerLogDirectory)
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

func generateListenerSpecificConfig(l *v1beta1.ListenersConfig, serverPasses map[string]string, log logr.Logger) (*properties.Properties, []string) {
	config := properties.NewProperties()

	interBrokerListenerName, securityProtocolMapConfig, listenerConfig, internalListenerSSLConfig, externalListenerSSLConfig := getListenerSpecificConfig(l, serverPasses, log)

	for k, v := range internalListenerSSLConfig {
		if err := config.Set(k, v); err != nil {
			log.Error(err, fmt.Sprintf("setting '%s' parameter in broker configuration resulted an error", k))
		}
	}

	for k, v := range externalListenerSSLConfig {
		if err := config.Set(k, v); err != nil {
			log.Error(err, fmt.Sprintf("setting '%s' parameter in broker configuration resulted an error", k))
		}
	}

	if err := config.Set(kafkautils.KafkaConfigListenerSecurityProtocolMap, securityProtocolMapConfig); err != nil {
		log.Error(err, fmt.Sprintf("setting '%s' parameter in broker configuration resulted an error", kafkautils.KafkaConfigListenerSecurityProtocolMap))
	}
	if err := config.Set(kafkautils.KafkaConfigInterBrokerListenerName, interBrokerListenerName); err != nil {
		log.Error(err, fmt.Sprintf("setting '%s' parameter in broker configuration resulted an error", kafkautils.KafkaConfigInterBrokerListenerName))
	}
	if err := config.Set(kafkautils.KafkaConfigListeners, listenerConfig); err != nil {
		log.Error(err, fmt.Sprintf("setting '%s' parameter in broker configuration resulted an error", kafkautils.KafkaConfigListeners))
	}

	return config, listenerConfig
}

func getListenerSpecificConfig(l *v1beta1.ListenersConfig, serverPasses map[string]string, log logr.Logger) (string, []string, []string, map[string]string, map[string]string) {
	var (
		interBrokerListenerName   string
		securityProtocolMapConfig []string
		listenerConfig            []string
		internalListenerSSLConfig map[string]string
		externalListenerSSLConfig map[string]string
	)

	for _, iListener := range l.InternalListeners {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListenerName == "" {
				interBrokerListenerName = strings.ToUpper(iListener.Name)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
		upperedListenerType := iListener.Type.ToUpperString()
		upperedListenerName := strings.ToUpper(iListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", upperedListenerName, upperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", upperedListenerName, iListener.ContainerPort))

		// Add internal listeners SSL configuration
		if iListener.Type == v1beta1.SecurityProtocolSSL {
			internalListenerSSLConfig = generateListenerSSLConfig(iListener.Name, iListener.SSLClientAuth, serverPasses[iListener.Name])
		}
	}

	for _, eListener := range l.ExternalListeners {
		upperedListenerType := eListener.Type.ToUpperString()
		upperedListenerName := strings.ToUpper(eListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", upperedListenerName, upperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", upperedListenerName, eListener.ContainerPort))
		// Add external listeners SSL configuration
		if eListener.Type == v1beta1.SecurityProtocolSSL {
			externalListenerSSLConfig = generateListenerSSLConfig(eListener.Name, eListener.SSLClientAuth, serverPasses[eListener.Name])
		}
	}

	return interBrokerListenerName, securityProtocolMapConfig, listenerConfig, internalListenerSSLConfig, externalListenerSSLConfig
}

func generateListenerSSLConfig(name string, sslClientAuth v1beta1.SSLClientAuthentication, password string) map[string]string {
	var listenerSSLConfig map[string]string
	namedKeystorePath := fmt.Sprintf(listenerServerKeyStorePathTemplate, serverKeystorePath, name)
	keyStoreType := "JKS"
	keyStoreLoc := namedKeystorePath + "/" + v1alpha1.TLSJKSKeyStore
	trustStoreType := "JKS"
	trustStoreLoc := namedKeystorePath + "/" + v1alpha1.TLSJKSTrustStore

	listenerSSLConfig = map[string]string{
		fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLKeyStoreLocation):   keyStoreLoc,
		fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLTrustStoreLocation): trustStoreLoc,
		fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLKeystoreType):       keyStoreType,
		fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLTrustStoreType):     trustStoreType,
		fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLTrustStorePassword): password,
		fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLKeyStorePassword):   password,
	}

	// enable 2-way SSL authentication if SSL is enabled but this field is not provided in the listener config
	if sslClientAuth == "" {
		listenerSSLConfig[fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLClientAuth)] = string(v1beta1.SSLClientAuthRequired)
	} else {
		listenerSSLConfig[fmt.Sprintf("%s.%s.%s", kafkautils.KafkaConfigListenerName, name, kafkautils.KafkaConfigSSLClientAuth)] = string(sslClientAuth)
	}

	return listenerSSLConfig
}

// mergeSuperUsersPropertyValue merges the target and source super.users property value, and returns it as string.
// It returns empty string when there were no updates or any of the super.users property value was empty.
func mergeSuperUsersPropertyValue(source *properties.Properties, target *properties.Properties) string {
	sourceVal, foundSource := source.Get(kafkautils.KafkaConfigSuperUsers)
	if !foundSource || sourceVal.IsEmpty() {
		return ""
	}
	targetVal, foundTarget := target.Get(kafkautils.KafkaConfigSuperUsers)
	if !foundTarget || targetVal.IsEmpty() {
		return ""
	}

	sourceSuperUsers := strings.Split(sourceVal.Value(), ";")
	targetSuperUsers := strings.Split(targetVal.Value(), ";")

	inserted := false
	for _, sourceSuperUser := range sourceSuperUsers {
		found := false
		for _, targetSuperUser := range targetSuperUsers {
			if sourceSuperUser == targetSuperUser {
				found = true
				break
			}
		}

		if !found {
			inserted = true
			targetSuperUsers = append(targetSuperUsers, sourceSuperUser)
		}
	}

	if inserted {
		return strings.Join(targetSuperUsers, ";")
	}

	return ""
}

func (r Reconciler) generateBrokerConfig(broker v1beta1.Broker, brokerConfig *v1beta1.BrokerConfig, extListenerStatuses,
	intListenerStatuses, controllerIntListenerStatuses map[string]v1beta1.ListenerStatusList,
	serverPasses map[string]string, clientPass string, superUsers []string, log logr.Logger) string {
	finalBrokerConfig := getBrokerReadOnlyConfig(broker, r.KafkaCluster, log)

	// Get operator generated configuration
	opGenConf := r.getConfigProperties(brokerConfig, broker, extListenerStatuses, intListenerStatuses,
		controllerIntListenerStatuses, serverPasses, clientPass, superUsers, log)

	// Merge operator generated configuration to the final one
	if opGenConf != nil {
		// When there is custom super.users configuration we merge its value with the Koperator generated one
		// to avoid overwrite that happens when the finalBrokerConfig.Merge(opGenConf) is called.
		if suMerged := mergeSuperUsersPropertyValue(finalBrokerConfig, opGenConf); suMerged != "" {
			// Setting string value for a property is not going to run into error, also we don't return error in this function
			//nolint:errcheck
			opGenConf.Set(kafkautils.KafkaConfigSuperUsers, suMerged)
		}
		finalBrokerConfig.Merge(opGenConf)
	}

	finalBrokerConfig.Sort()

	return finalBrokerConfig.String()
}

// TODO move this into api in the future (adamantal)
func getBrokerReadOnlyConfig(broker v1beta1.Broker, kafkaCluster *v1beta1.KafkaCluster, log logr.Logger) *properties.Properties {
	// Parse cluster-wide readonly configuration
	finalBrokerConfig, err := properties.NewFromString(kafkaCluster.Spec.ReadOnlyConfig)
	if err != nil {
		log.Error(err, "failed to parse readonly cluster configuration")
	}

	// Parse readonly broker configuration
	parsedReadOnlyBrokerConfig, err := properties.NewFromString(broker.ReadOnlyConfig)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to parse readonly broker configuration for broker with id: %d", broker.Id))
	}

	// Merge cluster-wide configuration into broker-level configuration
	if parsedReadOnlyBrokerConfig != nil {
		finalBrokerConfig.Merge(parsedReadOnlyBrokerConfig)
	}

	return finalBrokerConfig
}
