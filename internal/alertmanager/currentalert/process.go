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
	"errors"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ccConfig struct {
	Name                  string
	Namespace             string
	CruiseControlEndpoint string
}

type disableScaling struct {
	Up   bool
	Down bool
}

func (e *examiner) getCR() (*v1beta1.KafkaCluster, *ccConfig, error) {
	cr, err := k8sutil.GetCr(string(e.Alert.Labels["kafka_cr"]), string(e.Alert.Labels["namespace"]), e.Client)
	if err != nil {
		return nil, nil, err
	}
	cc := &ccConfig{
		Name:                  cr.Name,
		Namespace:             cr.Namespace,
		CruiseControlEndpoint: cr.Spec.CruiseControlConfig.CruiseControlEndpoint,
	}
	return cr, cc, nil
}

func (e *examiner) examineAlert(rollingUpgradeAlertCount int) error {

	cr, cc, err := e.getCR()
	if err != nil {
		return err
	}

	if !e.IgnoreCCStatus {
		if err := cc.getCruiseControlStatus(); err != nil {
			return err
		}
	}

	if err := k8sutil.UpdateCrWithRollingUpgrade(rollingUpgradeAlertCount, cr, e.Client); err != nil {
		return err
	}

	if cr.Status.State == "rollingupgrade" {
		return nil
	}

	ds := disableScaling{}
	if cr.Spec.AlertManagerConfig != nil {
		if len(cr.Spec.Brokers) <= cr.Spec.AlertManagerConfig.MinBrokerCount {
			ds.Down = true
		}
		if len(cr.Spec.Brokers) >= cr.Spec.AlertManagerConfig.MaxBrokerCount {
			ds.Up = true
		}
	}

	return e.processAlert(ds)
}

func (e *examiner) processAlert(ds disableScaling) error {

	switch e.Alert.Annotations["command"] {
	case "addPVC":
		err := addPVC(e.Alert.Labels, e.Alert.Annotations, e.Client)
		if err != nil {
			return err
		}
	case "downScale":
		if ds.Down {
			return errors.New("downscaling is skipped due to minimum broker count")
		}
		err := downScale(e.Alert.Labels, e.Client)
		if err != nil {
			return err
		}
	case "upScale":
		if ds.Up {
			return errors.New("upscaling is skipped due to maximum broker count")
		}
		err := upScale(e.Alert.Labels, e.Alert.Annotations, e.Client)
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

	var broker v1beta1.Broker

	brokerConfigGroupName := string(annotations["brokerConfigGroup"])

	if _, ok := cr.Spec.BrokerConfigGroups[brokerConfigGroupName]; ok {

		broker.BrokerConfigGroup = brokerConfigGroupName
		broker.Id = biggestId + 1

	} else {

		broker = v1beta1.Broker{
			Id: biggestId + 1,
			BrokerConfig: &v1beta1.BrokerConfig{
				Image: string(annotations["image"]),
				StorageConfigs: []v1beta1.StorageConfig{
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

func (c *ccConfig) getCruiseControlStatus() error {
	return scale.GetCruiseControlStatus(c.Namespace, c.CruiseControlEndpoint, c.Name)
}
