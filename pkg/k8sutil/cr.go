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
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateCrWithNodeAffinity updates the given Cr with NodeAffinity status
func UpdateCrWithNodeAffinity(current *corev1.Pod, cr *v1beta1.KafkaCluster, client runtimeClient.Client) error {
	failureDomainSelectors, err := failureDomainSelectors(current.Spec.NodeName, client)
	if err != nil {
		return errors.WrapIfWithDetails(err, "determining Node selector failed")
	}

	// don't set node affinity when none of the selector labels are available for the node
	if len(failureDomainSelectors) < 1 {
		return nil
	}

	brokers := []v1beta1.Broker{}

	for _, broker := range cr.Spec.Brokers {
		if strconv.Itoa(int(broker.Id)) == current.Labels["brokerId"] {
			nodeAffinity := &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: failureDomainSelectors,
				},
			}
			if broker.BrokerConfig != nil {
				broker.BrokerConfig.NodeAffinity = nodeAffinity
			} else {
				bConfig := &v1beta1.BrokerConfig{
					NodeAffinity: nodeAffinity,
				}
				broker.BrokerConfig = bConfig
			}
		}
		brokers = append(brokers, broker)
	}
	cr.Spec.Brokers = brokers
	return updateCr(cr, client)
}

// UpdateCrWithRackAwarenessConfig updates the CR with rack awareness config
func UpdateCrWithRackAwarenessConfig(pod *corev1.Pod, cr *v1beta1.KafkaCluster, client runtimeClient.Client) error {

	if pod.Spec.NodeName == "" {
		return errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("pod does not scheduled to node yet"), "trying")
	}
	rackConfigMap, err := getSpecificNodeLabels(pod.Spec.NodeName, client, cr.Spec.RackAwareness.Labels)
	if err != nil {
		return errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating cr with rack awareness info failed")
	}
	rackConfigValues := make([]string, 0, len(rackConfigMap))
	for _, value := range rackConfigMap {
		rackConfigValues = append(rackConfigValues, value)
	}
	brokerConfigs := []v1beta1.Broker{}

	for _, broker := range cr.Spec.Brokers {
		if strconv.Itoa(int(broker.Id)) == pod.Labels["brokerId"] {
			if broker.ReadOnlyConfig == "" {
				readOnlyConfig := fmt.Sprintf("broker.rack=%s\n", strings.Join(rackConfigValues, ","))
				broker.ReadOnlyConfig = readOnlyConfig
			} else if !strings.Contains(broker.ReadOnlyConfig, "broker.rack=") {
				readOnlyConfig := broker.ReadOnlyConfig + fmt.Sprintf("broker.rack=%s\n", strings.Join(rackConfigValues, ","))
				broker.ReadOnlyConfig = readOnlyConfig
			}
		}
		brokerConfigs = append(brokerConfigs, broker)
	}
	cr.Spec.Brokers = brokerConfigs
	return updateCr(cr, client)
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
	tempConfigs := cr.Spec.Brokers[:0]
	for _, broker := range cr.Spec.Brokers {
		if strconv.Itoa(int(broker.Id)) == brokerId {
			broker.BrokerConfig.StorageConfigs = append(broker.BrokerConfig.StorageConfigs, *storageConfig)
		}
		tempConfigs = append(tempConfigs, broker)
	}

	cr.Spec.Brokers = tempConfigs
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
