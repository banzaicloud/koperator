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
	"strconv"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/goph/emperror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func updateCrWithNodeAffinity(current *corev1.Pod, cr *banzaicloudv1alpha1.KafkaCluster, client runtimeClient.Client) error {
	nodeZoneAndRegion, err := determineNodeZoneAndRegion(current.Spec.NodeName, client)
	if err != nil {
		return emperror.WrapWith(err, "determining Node zone failed")
	}

	brokerConfigs := []banzaicloudv1alpha1.BrokerConfig{}

	for _, brokerConfig := range cr.Spec.BrokerConfigs {
		if strconv.Itoa(int(brokerConfig.Id)) == current.Labels["brokerId"] {
			nodeAffinity := &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      zoneLabel,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{nodeZoneAndRegion.Zone},
								},
								{
									Key:      regionLabel,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{nodeZoneAndRegion.Region},
								},
							},
						},
					},
				},
			}
			brokerConfig.NodeAffinity = nodeAffinity
		}
		brokerConfigs = append(brokerConfigs, brokerConfig)
	}
	cr.Spec.BrokerConfigs = brokerConfigs
	return updateCr(cr, client)
}

func AddNewBrokerToCr(brokerConfig *banzaicloudv1alpha1.BrokerConfig, crName, namespace string, client runtimeClient.Client) error {
	cr, err := GetCr(crName, namespace, client)
	if err != nil {
		return err
	}
	cr.Spec.BrokerConfigs = append(cr.Spec.BrokerConfigs, *brokerConfig)

	return updateCr(cr, client)
}

func RemoveBrokerFromCr(brokerId, crName, namespace string, client runtimeClient.Client) error {

	cr, err := GetCr(crName, namespace, client)
	if err != nil {
		return err
	}

	tmpBrokers := cr.Spec.BrokerConfigs[:0]
	for _, broker := range cr.Spec.BrokerConfigs {
		if strconv.Itoa(int(broker.Id)) != brokerId {
			tmpBrokers = append(tmpBrokers, broker)
		}
	}
	cr.Spec.BrokerConfigs = tmpBrokers
	return updateCr(cr, client)
}

func AddPvToSpecificBroker(brokerId, crName, namespace string, storageConfig *banzaicloudv1alpha1.StorageConfig, client runtimeClient.Client) error {
	cr, err := GetCr(crName, namespace, client)
	if err != nil {
		return err
	}
	tempConfigs := cr.Spec.BrokerConfigs[:0]
	for _, brokerConfig := range cr.Spec.BrokerConfigs {
		if strconv.Itoa(int(brokerConfig.Id)) == brokerId {
			brokerConfig.StorageConfigs = append(brokerConfig.StorageConfigs, *storageConfig)
		}
		tempConfigs = append(tempConfigs, brokerConfig)
	}

	cr.Spec.BrokerConfigs = tempConfigs
	return updateCr(cr, client)
}

func GetCr(name, namespace string, client runtimeClient.Client) (*banzaicloudv1alpha1.KafkaCluster, error) {
	cr := &banzaicloudv1alpha1.KafkaCluster{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cr)
	if err != nil {
		return nil, emperror.WrapWith(err, "could not get cr from k8s", "crName", name, "namespace", namespace)
	}
	return cr, nil
}

func updateCr(cr *banzaicloudv1alpha1.KafkaCluster, client runtimeClient.Client) error {
	err := client.Update(context.TODO(), cr)
	if err != nil {
		return err
	}
	return nil
}
