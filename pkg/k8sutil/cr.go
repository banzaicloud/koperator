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

package k8sutil

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"emperror.dev/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
)

// UpdateCrWithRackAwarenessConfig updates the CR with rack awareness config
func UpdateCrWithRackAwarenessConfig(pod *corev1.Pod, cr *v1beta1.KafkaCluster, client runtimeClient.Client, directClient runtimeClient.Reader) (v1beta1.RackAwarenessState, error) {
	rackConfigMap, err := getSpecificNodeLabels(pod.Spec.NodeName, directClient, cr.Spec.RackAwareness.Labels)
	if err != nil {
		return "", errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating cr with rack awareness info failed")
	}
	rackConfigValues := make([]string, 0, len(rackConfigMap))
	for _, value := range rackConfigMap {
		rackConfigValues = append(rackConfigValues, value)
	}
	sort.Strings(rackConfigValues)

	rackAwarenessState, brokers := rackAwarenessLabelsToReadonlyConfig(pod, cr, rackConfigValues)
	cr.Spec.Brokers = brokers
	return rackAwarenessState, updateCr(cr, client)
}

func rackAwarenessLabelsToReadonlyConfig(pod *corev1.Pod, cr *v1beta1.KafkaCluster, rackConfigValues []string) (v1beta1.RackAwarenessState, []v1beta1.Broker) {

	brokerConfigs := []v1beta1.Broker{}
	var readOnlyConfig string
	var rackAwaranessState string
	brokerId := pod.Labels["brokerId"]
	for _, broker := range cr.Spec.Brokers {
		if strconv.Itoa(int(broker.Id)) == brokerId {
			rackAwaranessState = fmt.Sprintf("broker.rack=%s\n", strings.Join(rackConfigValues, ","))
			if _, ok := cr.Status.BrokersState[brokerId]; ok && cr.Status.BrokersState[brokerId].RackAwarenessState != "" && cr.Status.BrokersState[brokerId].RackAwarenessState != v1beta1.Configured {
				if !strings.Contains(broker.ReadOnlyConfig, "broker.rack=") {
					readOnlyConfig = broker.ReadOnlyConfig + string(cr.Status.BrokersState[brokerId].RackAwarenessState)
				} else {
					readOnlyConfig = broker.ReadOnlyConfig
				}
				rackAwaranessState = string(cr.Status.BrokersState[brokerId].RackAwarenessState)
			} else if broker.ReadOnlyConfig == "" {
				readOnlyConfig = rackAwaranessState
			} else if !strings.Contains(broker.ReadOnlyConfig, "broker.rack=") {
				readOnlyConfig = broker.ReadOnlyConfig + rackAwaranessState
			}
			broker.ReadOnlyConfig = readOnlyConfig
		}
		brokerConfigs = append(brokerConfigs, broker)
	}
	return v1beta1.RackAwarenessState(rackAwaranessState), brokerConfigs
}

// AddNewBrokerToCr modifies the CR and adds a new broker
func AddNewBrokerToCr(broker v1beta1.Broker, crName, namespace string, client runtimeClient.Client) error {
	cr, err := GetCr(crName, namespace, client)
	if err != nil {
		return err
	}
	cr.Spec.Brokers = append(cr.Spec.Brokers, broker)

	return updateCr(cr, client)
}

// RemoveBrokerFromCr modifies the CR and removes the given broker from the cluster
func RemoveBrokerFromCr(brokerId, crName, namespace string, client runtimeClient.Client) error {

	cr, err := GetCr(crName, namespace, client)
	if err != nil {
		return err
	}

	tmpBrokers := cr.Spec.Brokers[:0]
	for _, broker := range cr.Spec.Brokers {
		if strconv.Itoa(int(broker.Id)) != brokerId {
			tmpBrokers = append(tmpBrokers, broker)
		}
	}
	cr.Spec.Brokers = tmpBrokers
	return updateCr(cr, client)
}

// AddPvToSpecificBroker adds a new PV to a specific broker
func AddPvToSpecificBroker(brokerId, crName, namespace string, storageConfig *v1beta1.StorageConfig, client runtimeClient.Client) error {
	cr, err := GetCr(crName, namespace, client)
	if err != nil {
		return err
	}

	for i, broker := range cr.Spec.Brokers {
		if strconv.Itoa(int(broker.Id)) == brokerId {
			if broker.BrokerConfig == nil {
				cr.Spec.Brokers[i].BrokerConfig = &v1beta1.BrokerConfig{}
			}
			cr.Spec.Brokers[i].BrokerConfig.StorageConfigs = append(cr.Spec.Brokers[i].BrokerConfig.StorageConfigs, *storageConfig)
		}
	}

	return updateCr(cr, client)
}

// GetCr returns the given cr object
func GetCr(name, namespace string, client runtimeClient.Client) (*v1beta1.KafkaCluster, error) {
	cr := &v1beta1.KafkaCluster{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cr)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "could not get cr from k8s", "crName", name, "namespace", namespace)
	}
	return cr, nil
}

func updateCr(cr *v1beta1.KafkaCluster, client runtimeClient.Client) error {
	typeMeta := cr.TypeMeta
	err := client.Update(context.TODO(), cr)
	if err != nil {
		return err
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cr.TypeMeta = typeMeta
	return nil
}

// UpdateCrWithRollingUpgrade modifies CR status
func UpdateCrWithRollingUpgrade(errorCount int, cr *v1beta1.KafkaCluster, client runtimeClient.Client) error {

	cr.Status.RollingUpgrade.ErrorCount = errorCount
	return updateCr(cr, client)
}
