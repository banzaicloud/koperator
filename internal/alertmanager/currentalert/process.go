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

package currentalert

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func examineAlert(alert *currentAlertStruct, client client.Client, rollingUpgradeAlertCount int) error {

	cr, err := k8sutil.GetCr(string(alert.Labels["kafka_cr"]), string(alert.Labels["namespace"]), client)
	if err != nil {
		return err
	}
	k8sutil.UpdateCrWithRollingUpgrade(rollingUpgradeAlertCount, cr, client)

	if cr.Status.State == "rollingupgrade" {
		return nil
	}
	return processAlert(alert, client)
}

func processAlert(alert *currentAlertStruct, client client.Client) error {

	switch alert.Annotations["command"] {
	case "addPVC":
		err := addPVC(alert.Labels, alert.Annotations, client)
		if err != nil {
			return err
		}
	case "downScale":
		err := downScale(alert.Labels, client)
		if err != nil {
			return err
		}
	case "upScale":
		err := upScale(alert.Labels, alert.Annotations, client)
		if err != nil {
			return err
		}
	}
	return nil
}

func addPVC(labels model.LabelSet, annotations model.LabelSet, client client.Client) error {
	//TODO
	return nil
}

func downScale(labels model.LabelSet, client client.Client) error {

	cr, err := k8sutil.GetCr(string(labels["kafka_cr"]), string(labels["namespace"]), client)
	if err != nil {
		return err
	}

	brokerId, err := scale.GetBrokerIDWithLeastPartition(string(labels["namespace"]), cr.Spec.CruiseControlConfig.CruiseControlEndpoint, cr.Name)
	if err != nil {
		return err
	}
	err = k8sutil.RemoveBrokerFromCr(brokerId, string(labels["kafka_cr"]), string(labels["namespace"]), client)
	if err != nil {
		return err
	}
	return nil
}

func upScale(labels model.LabelSet, annotations model.LabelSet, client client.Client) error {

	cr, err := k8sutil.GetCr(string(labels["kafka_cr"]), string(labels["namespace"]), client)
	if err != nil {
		return err
	}

	biggestId := int32(0)
	for _, broker := range cr.Spec.Brokers {
		if broker.Id > biggestId {
			biggestId = broker.Id
		}
	}

	var broker banzaicloudv1alpha1.Broker

	brokerConfigGroupName := string(annotations["brokerConfigGroup"])

	if _, ok := cr.Spec.BrokerConfigGroups[brokerConfigGroupName]; ok {

		broker.BrokerConfigGroup = brokerConfigGroupName
		broker.Id = biggestId + 1

	} else {

		broker = banzaicloudv1alpha1.Broker{
			Id: biggestId + 1,
			BrokerConfig: &banzaicloudv1alpha1.BrokerConfig{
				Image: string(annotations["image"]),
				StorageConfigs: []banzaicloudv1alpha1.StorageConfig{
					{
						MountPath: string(annotations["mountPath"]),
						PVCSpec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							StorageClassName: util.StringPointer(string(annotations["storageClass"])),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse(string(annotations["diskSize"])),
								},
							},
						},
					},
				},
			},
		}
	}

	err = k8sutil.AddNewBrokerToCr(broker, string(labels["kafka_cr"]), string(labels["namespace"]), client)
	if err != nil {
		return err
	}
	return nil
}
