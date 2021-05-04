// Copyright © 2021 Banzai Cloud
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

package v1beta1

import (
	"emperror.dev/errors"
	"github.com/imdario/mergo"

	corev1 "k8s.io/api/core/v1"
)

// GetBrokerConfig compose the brokerConfig for a given broker
func GetBrokerConfig(broker Broker, clusterSpec KafkaClusterSpec) (*BrokerConfig, error) {
	bConfig := &BrokerConfig{}
	if broker.BrokerConfigGroup == "" {
		return broker.BrokerConfig, nil
	} else if broker.BrokerConfig != nil {
		bConfig = broker.BrokerConfig.DeepCopy()
	}

	groupConfig, exists := clusterSpec.BrokerConfigGroups[broker.BrokerConfigGroup]
	if !exists {
		return nil, errors.NewWithDetails("missing brokerConfigGroup", "key", broker.BrokerConfigGroup)
	}

	dstAffinity := &corev1.Affinity{}
	srcAffinity := &corev1.Affinity{}

	if groupConfig.Affinity != nil {
		dstAffinity = groupConfig.Affinity.DeepCopy()
	}
	if bConfig.Affinity != nil {
		srcAffinity = bConfig.Affinity
	}

	if err := mergo.Merge(dstAffinity, srcAffinity, mergo.WithOverride); err != nil {
		return nil, errors.WrapIf(err, "could not merge brokerConfig.Affinity with ConfigGroup.Affinity")
	}

	err := mergo.Merge(bConfig, groupConfig, mergo.WithAppendSlice)
	if err != nil {
		return nil, errors.WrapIf(err, "could not merge brokerConfig with ConfigGroup")
	}

	bConfig.StorageConfigs = dedupStorageConfigs(bConfig.StorageConfigs)

	if groupConfig.Affinity != nil || bConfig.Affinity != nil {
		bConfig.Affinity = dstAffinity
	}

	return bConfig, nil
}

func dedupStorageConfigs(elements []StorageConfig) []StorageConfig {
	encountered := make(map[string]struct{})
	result := make([]StorageConfig, 0)

	for _, v := range elements {
		if _, ok := encountered[v.MountPath]; !ok {
			encountered[v.MountPath] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}
