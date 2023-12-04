// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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
	"github.com/IBM/sarama"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/util/kafka"

	properties "github.com/banzaicloud/koperator/properties/pkg"
)

func (r *Reconciler) reconcilePerBrokerDynamicConfig(brokerId int32, brokerConfig *v1beta1.BrokerConfig, configMap *corev1.ConfigMap, log logr.Logger) error {
	kClient, close, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer close()

	fullPerBrokerConfig, err := properties.NewFromString(brokerConfig.Config)
	if err != nil {
		return errors.WrapIf(err, "could not parse broker configuration")
	}

	currentPerBrokerConfigState := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(brokerId))].PerBrokerConfigurationState
	if fullPerBrokerConfig.Len() == 0 && currentPerBrokerConfigState != v1beta1.PerBrokerConfigOutOfSync {
		return nil
	}

	// overwrite configs from configmap
	configsFromConfigMap, err := properties.NewFromString(configMap.Data[kafka.ConfigPropertyName])
	if err != nil {
		return errors.WrapIf(err, "could not parse broker configuration from configmap")
	}
	for _, perBrokerConfig := range kafka.PerBrokerConfigs {
		if configProperty, ok := configsFromConfigMap.Get(perBrokerConfig); ok {
			fullPerBrokerConfig.Put(configProperty)
		}
	}

	// query the current config
	brokerConfigKeys := fullPerBrokerConfig.Keys()
	response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not describe broker config", v1beta1.BrokerIdLabelKey, brokerId)
	}

	if shouldUpdatePerBrokerConfig(response, fullPerBrokerConfig) {
		if currentPerBrokerConfigState == v1beta1.PerBrokerConfigInSync {
			log.V(1).Info("setting per broker config status to out of sync")
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigOutOfSync, log)
			if statusErr != nil {
				return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", v1beta1.BrokerIdLabelKey, brokerId)
			}
		}

		// validate the config
		err := kClient.AlterPerBrokerConfig(brokerId, util.ConvertPropertiesToMapStringPointer(fullPerBrokerConfig), true)
		if err != nil {
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigError, log)
			if statusErr != nil {
				err = errors.Combine(err, statusErr)
			}
			return errors.WrapIfWithDetails(err, "could not validate per-broker broker config", v1beta1.BrokerIdLabelKey, brokerId)
		}

		// alter the config
		err = kClient.AlterPerBrokerConfig(brokerId, util.ConvertPropertiesToMapStringPointer(fullPerBrokerConfig), false)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not alter broker config", v1beta1.BrokerIdLabelKey, brokerId)
		}

		// query the updated config
		response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not describe broker config", v1beta1.BrokerIdLabelKey, brokerId)
		}

		// update per-broker config status based on the response
		if shouldUpdatePerBrokerConfig(response, fullPerBrokerConfig) {
			return errorfactory.New(errorfactory.PerBrokerConfigNotReady{}, errors.New("configuration is out of sync"), "per-broker configuration updated")
		}

		statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigInSync, log)
		if statusErr != nil {
			return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", v1beta1.BrokerIdLabelKey, brokerId)
		}
	} else if currentPerBrokerConfigState != v1beta1.PerBrokerConfigInSync {
		log.V(1).Info("setting per broker config status to in sync")
		statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster, v1beta1.PerBrokerConfigInSync, log)
		if statusErr != nil {
			return errors.WrapIfWithDetails(err, "updating status for per-broker configuration status failed", v1beta1.BrokerIdLabelKey, brokerId)
		}
	}

	return nil
}

func (r *Reconciler) reconcileClusterWideDynamicConfig() error {
	kClient, close, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer close()

	currentConfig, err := kClient.DescribeClusterWideConfig()
	if err != nil {
		return errors.WrapIf(err, "could not describe cluster wide broker config")
	}

	currentClusterWideConfig, err := util.ConvertConfigEntryListToProperties(currentConfig)
	if err != nil {
		return errors.WrapIf(err, "could not convert current cluster-wide config to properties")
	}

	parsedClusterWideConfig, err := properties.NewFromString(r.KafkaCluster.Spec.ClusterWideConfig)
	if err != nil {
		return errors.WrapIf(err, "could not parse cluster-wide broker config")
	}

	if !currentClusterWideConfig.Equal(parsedClusterWideConfig) {
		err = kClient.AlterClusterWideConfig(util.ConvertPropertiesToMapStringPointer(parsedClusterWideConfig), true)
		if err != nil {
			return errors.WrapIf(err, "validation of cluster wide config update failed")
		}

		err = kClient.AlterClusterWideConfig(util.ConvertPropertiesToMapStringPointer(parsedClusterWideConfig), false)
		if err != nil {
			return errors.WrapIf(err, "could not alter cluster wide broker config")
		}
	}

	return nil
}

func shouldUpdatePerBrokerConfig(response []*sarama.ConfigEntry, brokerConfig *properties.Properties) bool {
	if brokerConfig == nil {
		return false
	}

	for _, conf := range response {
		if val, ok := brokerConfig.Get(conf.Name); ok {
			if val.Value() != conf.Value {
				return true
			}
		}
	}

	return false
}
