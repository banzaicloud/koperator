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
	"context"
	"fmt"
	"strconv"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

type disableScaling struct {
	Up   bool
	Down bool
}

const (
	// AddPvcCommand command name for addPvc
	AddPvcCommand = "addPvc"
	// DownScaleCommand command name for donscale
	DownScaleCommand = "downScale"
	// UpScaleCommand command name for ipscale
	UpScaleCommand = "upScale"
	// ResizePvcCommand command name for resizePvc
	ResizePvcCommand = "resizePvc"
)

// GetCommandList returns list of supported commands
func GetCommandList() []string {
	return []string{
		AddPvcCommand,
		DownScaleCommand,
		UpScaleCommand,
		ResizePvcCommand,
	}
}
func (e *examiner) getKafkaCr() (*v1beta1.KafkaCluster, error) {
	var cr *v1beta1.KafkaCluster
	if kafkaCr, ok := e.Alert.Labels["kafka_cr"]; ok {
		var err error
		cr, err = k8sutil.GetCr(string(kafkaCr), string(e.Alert.Labels["namespace"]), e.Client)
		if err != nil {
			return nil, err
		}
	} else {
		// If kafka_cr is not a valid alert label, try to get it from the persistentvolumeclaim
		if pvcName, ok := e.Alert.Labels["persistentvolumeclaim"]; ok {
			pvc, err := getPvc(string(pvcName), string(e.Alert.Labels["namespace"]), e.Client)
			if err != nil {
				return nil, err
			}
			if kafkaCr, ok := pvc.GetLabels()["kafka_cr"]; ok {
				cr, err = k8sutil.GetCr(kafkaCr, string(e.Alert.Labels["namespace"]), e.Client)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, errors.New("persistentvolumeclaim doesn't contain kafka_cr label")
			}
		}
	}

	return cr, nil
}

func (e *examiner) examineAlert(rollingUpgradeAlertCount int) (bool, error) {

	cr, err := e.getKafkaCr()
	if err != nil {
		return false, err
	}

	if err := k8sutil.UpdateCrWithRollingUpgrade(rollingUpgradeAlertCount, cr, e.Client); err != nil {
		return false, err
	}

	if cr.Status.State != v1beta1.KafkaClusterRunning {
		return false, nil
	}

	ds := disableScaling{}
	if cr.Spec.AlertManagerConfig != nil {
		if len(cr.Spec.Brokers) <= cr.Spec.AlertManagerConfig.DownScaleLimit {
			ds.Down = true
		}
		if cr.Spec.AlertManagerConfig.UpScaleLimit > 0 && len(cr.Spec.Brokers) >= cr.Spec.AlertManagerConfig.UpScaleLimit {
			ds.Up = true
		}
	}

	return e.processAlert(ds)
}

func (e *examiner) processAlert(ds disableScaling) (bool, error) {

	switch e.Alert.Annotations["command"] {
	case AddPvcCommand:
		validators := AlertValidators{newAddPvcValidator(e.Alert)}
		if err := validators.ValidateAlert(); err != nil {
			return false, err
		}
		err := addPvc(e.Log, e.Alert.Labels, e.Alert.Annotations, e.Client)
		if err != nil {
			return false, err
		}

		return true, nil
	case ResizePvcCommand:
		validators := AlertValidators{newResizePvcValidator(e.Alert)}
		if err := validators.ValidateAlert(); err != nil {
			return false, err
		}
		err := resizePvc(e.Alert.Labels, e.Alert.Annotations, e.Client)
		if err != nil {
			return false, err
		}

		return true, nil
	case DownScaleCommand:
		validators := AlertValidators{newDownScaleValidator(e.Alert)}
		if err := validators.ValidateAlert(); err != nil {
			return false, err
		}
		if ds.Down {
			e.Log.Info("downscale is skipped due to downscale limit")
			return true, nil
		}
		err := downScale(e.Log, e.Alert.Labels, e.Client)
		if err != nil {
			return false, err
		}

		return true, nil
	case UpScaleCommand:
		validators := AlertValidators{newUpScaleValidator(e.Alert)}
		if err := validators.ValidateAlert(); err != nil {
			return false, err
		}
		if ds.Up {
			e.Log.Info("upscale is skipped due to upscale limit")
			return true, nil
		}
		err := upScale(e.Log, e.Alert.Labels, e.Alert.Annotations, e.Client)
		if err != nil {
			return false, err
		}

		return true, nil
	}
	return false, nil
}

func addPvc(log logr.Logger, alertLabels model.LabelSet, alertAnnotations model.LabelSet, client client.Client) error {
	var storageClassName *string

	if alertAnnotations["storageClass"] != "" {
		storageClassName = util.StringPointer(string(alertAnnotations["storageClass"]))
	}

	pvc, err := getPvc(string(alertLabels["persistentvolumeclaim"]), string(alertLabels["namespace"]), client)
	if err != nil {
		return err
	}

	// Check for skipping in case of pending or running CC task
	ccTaskExists, err := pendingOrRunningCCTaskExists(pvc.Labels, client, log)
	if err != nil {
		return err
	}
	if ccTaskExists {
		return nil
	}

	//Check for skipping in case of k8s node cannot attach more PVs, (When there is already a pvc that is unbound)
	unboundPvcExists, err := unboundPvcOnNodeExists(client, pvc, log, string(alertLabels["node"]))
	if err != nil {
		return err
	}
	if unboundPvcExists {
		return nil
	}

	randomIdentifier, err := util.GetRandomString(6)
	if err != nil {
		return err
	}

	storageConfig := v1beta1.StorageConfig{
		MountPath: pvc.Annotations["mountPathPrefix"] + "-" + randomIdentifier,
		PvcSpec: &corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse(string(alertAnnotations["diskSize"])),
				},
			},
		}}

	err = k8sutil.AddPvToSpecificBroker(pvc.Labels["brokerId"], pvc.Labels["kafka_cr"], pvc.Labels["namespace"], &storageConfig, client)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("PV successfully added to broker %s with the following storage configuration: %+v", pvc.Labels["brokerId"], &storageConfig))

	return nil
}

func resizePvc(labels model.LabelSet, annotiations model.LabelSet, client client.Client) error {
	//TODO

	return nil
}

func downScale(log logr.Logger, labels model.LabelSet, client client.Client) error {

	cr, err := k8sutil.GetCr(string(labels["kafka_cr"]), string(labels["namespace"]), client)
	if err != nil {
		return err
	}

	if ids := kafka.GetBrokersWithPendingOrRunningCCTask(cr); len(ids) > 0 {
		var keyVals []interface{}
		for _, id := range ids {
			brokerId := strconv.Itoa(int(id))
			keyVals = append(keyVals, brokerId, cr.Status.BrokersState[brokerId].GracefulActionState.CruiseControlState)
		}

		log.Info("downscale is skipped as there are brokers which are pending task to be initiated in CC or already have a running CC task", keyVals...)

		return nil
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

func upScale(log logr.Logger, labels model.LabelSet, annotations model.LabelSet, client client.Client) error {

	cr, err := k8sutil.GetCr(string(labels["kafka_cr"]), string(labels["namespace"]), client)
	if err != nil {
		return err
	}

	if ids := kafka.GetBrokersWithPendingOrRunningCCTask(cr); len(ids) > 0 {
		var keyVals []interface{}
		for _, id := range ids {
			brokerId := strconv.Itoa(int(id))
			keyVals = append(keyVals, brokerId, cr.Status.BrokersState[brokerId].GracefulActionState.CruiseControlState)
		}

		log.Info("upscale is skipped as there are brokers which are pending task to be initiated in CC or already have a running CC task", keyVals...)

		return nil
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
		var storageClassName *string
		if annotations["storageClass"] != "" {
			storageClassName = util.StringPointer(string(annotations["storageClass"]))
		}

		broker = v1beta1.Broker{
			Id: biggestId + 1,
			BrokerConfig: &v1beta1.BrokerConfig{
				Image: string(annotations["image"]),
				StorageConfigs: []v1beta1.StorageConfig{
					{
						MountPath: string(annotations["mountPathPrefix"]),
						PvcSpec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							StorageClassName: storageClassName,
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

// getPvc returns the given PVC object
func getPvc(name, namespace string, client client.Client) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, pvc)

	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "could not get PVC from k8s", "PVCName", name, "namespace", namespace)
	}
	return pvc, nil
}

func pendingOrRunningCCTaskExists(pvcLabels map[string]string, client client.Client, log logr.Logger) (bool, error) {
	kafkaCluster, err := k8sutil.GetCr(pvcLabels["kafka_cr"], pvcLabels["namespace"], client)
	if err != nil {
		return false, err
	}

	if ids := kafka.GetBrokersWithPendingOrRunningCCTask(kafkaCluster); len(ids) > 0 {
		var keyVals []interface{}
		for _, id := range ids {
			brokerId := strconv.Itoa(int(id))
			keyVals = append(keyVals, brokerId, kafkaCluster.Status.BrokersState[brokerId].GracefulActionState.CruiseControlState)
		}

		log.Info("addPvc is skipped as there are brokers which are pending task to be initiated in CC or already have a running CC task", keyVals...)

		return true, nil
	}
	return false, nil
}

func unboundPvcOnNodeExists(c client.Client, pvc *corev1.PersistentVolumeClaim, log logr.Logger, nodeName string) (bool, error) {
	kafkaPvcList := &corev1.PersistentVolumeClaimList{}

	err := c.List(context.TODO(), kafkaPvcList, client.ListOption(client.InNamespace(pvc.Namespace)),
		client.ListOption(client.MatchingLabels(map[string]string{"app": "kafka", "kafka_cr": pvc.Labels["kafka_cr"]})))
	if err != nil {
		return false, err
	}

	kafkaPvcListOnNode := &corev1.PersistentVolumeClaimList{}
	for _, pvc := range kafkaPvcList.Items {
		if pvc.Annotations["volume.kubernetes.io/selected-node"] == nodeName {
			kafkaPvcListOnNode.Items = append(kafkaPvcListOnNode.Items, pvc)
		}
	}

	for _, pvc := range kafkaPvcListOnNode.Items {
		if pvc.Status.Phase == corev1.ClaimPending {
			log.Info("addPvc is skipped because a PVC exists on the node which is unbound", "node", nodeName)

			return true, nil
		}
	}
	return false, nil
}
