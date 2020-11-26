// Copyright © 2020 Banzai Cloud
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
	"reflect"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/Shopify/sarama"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

// these configurations will not trigger rolling upgrade when updated
var perBrokerConfigs = []string{
	// currently hardcoded in configmap.go
	"ssl.client.auth",

	// listener related config change will trigger rolling upgrade anyways due to pod spec change
	"listeners",
	"advertised.listeners",
}

const securityProtocolMap = "listener.security.protocol.map"

func (r *Reconciler) reconcilePerBrokerDynamicConfig(brokerId int32, brokerConfig *v1beta1.BrokerConfig, configMap *corev1.ConfigMap, log logr.Logger) error {
	kClient, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer func() {
		if err := kClient.Close(); err != nil {
			log.Error(err, "could not close client")
		}
	}()

	parsedBrokerConfig := util.ParsePropertiesFormat(brokerConfig.Config)

	currentPerBrokerConfigState := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(brokerId))].PerBrokerConfigurationState
	if len(parsedBrokerConfig) == 0 && currentPerBrokerConfigState != v1beta1.PerBrokerConfigOutOfSync {
		return nil
	}

	// overwrite configs that are generated in the configmap
	configsFromConfigMap := getBrokerConfigsFromConfigMap(configMap)
	for _, perBrokerConfig := range perBrokerConfigs {
		if configValue, ok := configsFromConfigMap[perBrokerConfig]; ok {
			parsedBrokerConfig[perBrokerConfig] = configValue
		}
	}

	brokerConfigKeys := make([]string, 0, len(parsedBrokerConfig)+len(perBrokerConfigs))
	for key := range parsedBrokerConfig {
		brokerConfigKeys = append(brokerConfigKeys, key)
	}
	brokerConfigKeys = append(brokerConfigKeys, perBrokerConfigs...)

	response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not describe broker config", "brokerId", brokerId)
	}

	changedConfigs := collectNonIdenticalConfigsFromResponse(response, parsedBrokerConfig)

	if len(changedConfigs) > 0 {
		if currentPerBrokerConfigState == v1beta1.PerBrokerConfigInSync {
			log.V(1).Info("setting per broker config status to out of sync")
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigOutOfSync, log)
			if statusErr != nil {
				return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", "brokerId", brokerId)
			}
		}

		// validate the config
		err := kClient.AlterPerBrokerConfig(brokerId, util.ConvertMapStringToMapStringPointer(changedConfigs), true)
		if err != nil {
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigError, log)
			if statusErr != nil {
				err = errors.Combine(err, statusErr)
			}
			return errors.WrapIfWithDetails(err, "could not validate per-broker broker config", "brokerId", brokerId)
		}

		// alter the config
		err = kClient.AlterPerBrokerConfig(brokerId, util.ConvertMapStringToMapStringPointer(changedConfigs), false)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not alter broker config", "brokerId", brokerId)
		}

		// request for the updated config
		response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not describe broker config", "brokerId", brokerId)
		}

		// update per-broker config status
		notUpdatedConfigs := collectNonIdenticalConfigsFromResponse(response, parsedBrokerConfig)
		if len(notUpdatedConfigs) > 0 {
			return errorfactory.New(errorfactory.PerBrokerConfigNotReady{}, errors.New("configuration is out of sync"), "per-broker configuration updated")
		} else {
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigInSync, log)
			if statusErr != nil {
				return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", "brokerId", brokerId)
			}
		}
	} else if currentPerBrokerConfigState != v1beta1.PerBrokerConfigInSync {
		log.V(1).Info("setting per broker config status to in sync")
		statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigInSync, log)
		if statusErr != nil {
			return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", "brokerId", brokerId)
		}
	}

	return nil
}

func (r *Reconciler) reconcileClusterWideDynamicConfig(log logr.Logger) error {
	kClient, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer func() {
		if err := kClient.Close(); err != nil {
			log.Error(err, "could not close client")
		}
	}()

	configIdentical := true

	currentConfig, err := kClient.DescribeClusterWideConfig()
	if err != nil {
		return errors.WrapIf(err, "could not describe cluster wide broker config")
	}
	parsedClusterWideConfig := util.ParsePropertiesFormat(r.KafkaCluster.Spec.ClusterWideConfig)
	if len(currentConfig) != len(parsedClusterWideConfig) {
		configIdentical = false
	}
	for _, conf := range currentConfig {
		if val, ok := parsedClusterWideConfig[conf.Name]; ok {
			if val != conf.Value {
				configIdentical = false
				break
			}
		}
	}
	// TODO add validation to the cluster-wide config update
	if !configIdentical {
		err = kClient.AlterClusterWideConfig(util.ConvertMapStringToMapStringPointer(parsedClusterWideConfig))
		if err != nil {
			return errors.WrapIf(err, "could not alter cluster wide broker config")
		}
	}

	return nil
}

func (r *Reconciler) postConfigMapReconcile(log logr.Logger, desired *corev1.ConfigMap) error {
	desiredType := reflect.TypeOf(desired)
	current := desired.DeepCopyObject()

	key, err := runtimeClient.ObjectKeyFromObject(current)
	if err != nil {
		return errors.WithDetails(err, "kind", desiredType)
	}
	log = log.WithValues("kind", desiredType, "name", key.Name)

	err = r.Client.Get(context.TODO(), key, current)
	if err != nil && !apierrors.IsNotFound(err) {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType, "name", key.Name)
	}

	if k8sutil.CheckIfObjectUpdated(log, desiredType, current, desired) {
		// Only update status when configmap belongs to broker
		if id, ok := desired.Labels["brokerId"]; ok {
			currentConfigs := getBrokerConfigsFromConfigMap(current.(*corev1.ConfigMap))
			desiredConfigs := getBrokerConfigsFromConfigMap(desired)

			var statusErr error
			// if only per broker configs are changed, do not trigger rolling upgrade by setting ConfigOutOfSync status
			if shouldRefreshOnlyPerBrokerConfigs(currentConfigs, desiredConfigs, log) {
				log.V(1).Info("setting per broker config status to out of sync")
				statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{id}, r.KafkaCluster, v1beta1.PerBrokerConfigOutOfSync, log)
			} else {
				statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{id}, r.KafkaCluster, v1beta1.ConfigOutOfSync, log)
			}
			if statusErr != nil {
				return errors.WrapIfWithDetails(err, "updating status for resource failed", "kind", desiredType)
			}
		}
	}

	return nil
}

// collects are the config keys that are either added, removed or updated
// between the current and the desired ConfigMap
func collectTouchedConfigs(currentConfigs, desiredConfigs map[string]string, log logr.Logger) []string {
	touchedConfigs := make([]string, 0)

	currentConfigsCopy := make(map[string]string, 0)
	for k, v := range currentConfigs {
		currentConfigsCopy[k] = v
	}

	for configName, desiredValue := range desiredConfigs {
		if currentValue, ok := currentConfigsCopy[configName]; !ok || currentValue != desiredValue {
			// new or updated config
			touchedConfigs = append(touchedConfigs, configName)
		}
		delete(currentConfigsCopy, configName)
	}

	for configName := range currentConfigsCopy {
		// deleted config
		touchedConfigs = append(touchedConfigs, configName)
	}

	log.V(1).Info("configs have been changed", "configs", touchedConfigs)
	return touchedConfigs
}

func getBrokerConfigsFromConfigMap(configMap *corev1.ConfigMap) map[string]string {
	brokerConfig := configMap.Data["broker-config"]
	configs := strings.Split(brokerConfig, "\n")
	m := make(map[string]string)
	for _, config := range configs {
		elements := strings.Split(config, "=")
		if len(elements) != 2 {
			continue
		}
		m[strings.TrimSpace(elements[0])] = strings.TrimSpace(elements[1])
	}
	return m
}

func collectNonIdenticalConfigsFromResponse(response []*sarama.ConfigEntry, parsedBrokerConfig map[string]string) map[string]string {
	changedConfigs := make(map[string]string, 0)
	for _, conf := range response {
		if val, ok := parsedBrokerConfig[conf.Name]; ok {
			if val != conf.Value {
				changedConfigs[conf.Name] = val
			}
		}
	}

	return changedConfigs
}

func shouldRefreshOnlyPerBrokerConfigs(currentConfigs, desiredConfigs map[string]string, log logr.Logger) bool {
	touchedConfigs := collectTouchedConfigs(currentConfigs, desiredConfigs, log)

OUTERLOOP:
	for _, config := range touchedConfigs {
		for _, perBrokerConfig := range perBrokerConfigs {
			if config == perBrokerConfig {
				continue OUTERLOOP
			}
		}

		// Security protocol cannot be updated for existing listener
		// a rolling upgrade should be triggered in this case
		if config == securityProtocolMap {
			// added or deleted config is ok
			if currentConfigs[securityProtocolMap] == "" || desiredConfigs[securityProtocolMap] == "" {
				continue
			}
			currentListenerProtocolMap := make(map[string]string, 0)
			desiredListenerProtocolMap := make(map[string]string, 0)
			for _, listenerConfig := range strings.Split(currentConfigs[securityProtocolMap], ",") {
				listenerKeyValue := strings.Split(listenerConfig, ":")
				if len(listenerKeyValue) != 2 {
					continue
				}
				currentListenerProtocolMap[listenerKeyValue[0]] = listenerKeyValue[1]
			}
			for _, listenerConfig := range strings.Split(desiredConfigs[securityProtocolMap], ",") {
				listenerKeyValue := strings.Split(listenerConfig, ":")
				if len(listenerKeyValue) != 2 {
					continue
				}
				desiredListenerProtocolMap[listenerKeyValue[0]] = listenerKeyValue[1]
			}

			for protocolName, desiredListenerProtocol := range desiredListenerProtocolMap {
				if currentListenerProtocol, ok := currentListenerProtocolMap[protocolName]; ok {
					if currentListenerProtocol != desiredListenerProtocol {
						return false
					}
				}
			}
			continue
		}
		return false
	}
	return true
}
