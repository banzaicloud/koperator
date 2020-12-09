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
	"strconv"

	"emperror.dev/errors"
	"github.com/Shopify/sarama"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/banzaicloud/kafka-operator/pkg/util/kafka"
)

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

	fullPerBrokerConfig := util.ParsePropertiesFormat(brokerConfig.Config)

	currentPerBrokerConfigState := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(brokerId))].PerBrokerConfigurationState
	if len(fullPerBrokerConfig) == 0 && currentPerBrokerConfigState != v1beta1.PerBrokerConfigOutOfSync {
		return nil
	}

	// overwrite configs from configmap
	configsFromConfigMap := util.ParsePropertiesFormat(configMap.Data[kafka.ConfigPropertyName])
	for _, perBrokerConfig := range kafka.PerBrokerConfigs {
		if configValue, ok := configsFromConfigMap[perBrokerConfig]; ok {
			fullPerBrokerConfig[perBrokerConfig] = configValue
		}
	}

	// query the current configs
	brokerConfigKeys := make([]string, 0, len(fullPerBrokerConfig))
	for key := range fullPerBrokerConfig {
		brokerConfigKeys = append(brokerConfigKeys, key)
	}
	response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not describe broker config", "brokerId", brokerId)
	}

	if shouldUpdatePerBrokerConfig(response, fullPerBrokerConfig) {
		if currentPerBrokerConfigState == v1beta1.PerBrokerConfigInSync {
			log.V(1).Info("setting per broker config status to out of sync")
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigOutOfSync, log)
			if statusErr != nil {
				return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", "brokerId", brokerId)
			}
		}

		// validate the config
		err := kClient.AlterPerBrokerConfig(brokerId, util.ConvertMapStringToMapStringPointer(fullPerBrokerConfig), true)
		if err != nil {
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigError, log)
			if statusErr != nil {
				err = errors.Combine(err, statusErr)
			}
			return errors.WrapIfWithDetails(err, "could not validate per-broker broker config", "brokerId", brokerId)
		}

		// alter the config
		err = kClient.AlterPerBrokerConfig(brokerId, util.ConvertMapStringToMapStringPointer(fullPerBrokerConfig), false)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not alter broker config", "brokerId", brokerId)
		}

		// query the updated config
		response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not describe broker config", "brokerId", brokerId)
		}

		// update per-broker config status based on the response
		if shouldUpdatePerBrokerConfig(response, fullPerBrokerConfig) {
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
	if configIdentical {
		for _, conf := range currentConfig {
			if val, ok := parsedClusterWideConfig[conf.Name]; ok {
				if val != conf.Value {
					configIdentical = false
					break
				}
			}
		}
	}

	if !configIdentical {
		err = kClient.AlterClusterWideConfig(util.ConvertMapStringToMapStringPointer(parsedClusterWideConfig), true)
		if err != nil {
			return errors.WrapIf(err, "validation of cluster wide config update failed")
		}

		err = kClient.AlterClusterWideConfig(util.ConvertMapStringToMapStringPointer(parsedClusterWideConfig), false)
		if err != nil {
			return errors.WrapIf(err, "could not alter cluster wide broker config")
		}
	}

	return nil
}

func shouldUpdatePerBrokerConfig(response []*sarama.ConfigEntry, parsedBrokerConfig map[string]string) bool {
	for _, conf := range response {
		if val, ok := parsedBrokerConfig[conf.Name]; ok {
			if val != conf.Value {
				return true
			}
		}
	}

	return false
}
