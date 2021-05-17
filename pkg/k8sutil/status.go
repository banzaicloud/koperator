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
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/banzaicloud/koperator/api/v1beta1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	clientutil "github.com/banzaicloud/koperator/pkg/util/client"
)

// IsAlreadyOwnedError checks if a controller already own the instance
func IsAlreadyOwnedError(err error) bool {
	return errors.Is(err, &controllerutil.AlreadyOwnedError{})
}

// IsMarkedForDeletion determines if the object is marked for deletion
func IsMarkedForDeletion(m metav1.ObjectMeta) bool {
	return m.GetDeletionTimestamp() != nil
}

// UpdateBrokerStatus updates the broker status with rack and configuration infos
func UpdateBrokerStatus(c client.Client, brokerIDs []string, cluster *banzaicloudv1beta1.KafkaCluster, state interface{}, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	generateBrokerState(brokerIDs, cluster, state)

	err := c.Status().Update(context.Background(), cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIff(err, "could not update Kafka broker(s) %s state", strings.Join(brokerIDs, ","))
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating status")
		}

		generateBrokerState(brokerIDs, cluster, state)

		err = c.Status().Update(context.Background(), cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return errors.WrapIff(err, "could not update Kafka clusters broker(s) %s state", strings.Join(brokerIDs, ","))
		}
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info("Kafka cluster state updated")
	return nil
}

func generateBrokerState(brokerIDs []string, cluster *banzaicloudv1beta1.KafkaCluster, state interface{}) {
	brokersState := cluster.Status.BrokersState
	if brokersState == nil {
		brokersState = make(map[string]banzaicloudv1beta1.BrokerState, len(brokerIDs))
	}

	for _, brokerID := range brokerIDs {
		brokerState, ok := brokersState[brokerID]
		if !ok {
			brokerState = banzaicloudv1beta1.BrokerState{}
		}
		switch s := state.(type) {
		case banzaicloudv1beta1.ExternalListenerConfigNames:
			brokerState.ExternalListenerConfigNames = s
		case banzaicloudv1beta1.RackAwarenessState:
			brokerState.RackAwarenessState = s
		case banzaicloudv1beta1.GracefulActionState:
			brokerState.GracefulActionState = s
		case map[string]banzaicloudv1beta1.GracefulActionState:
			state := s[brokerID]
			brokerState.GracefulActionState = state
		case banzaicloudv1beta1.ConfigurationState:
			brokerState.ConfigurationState = s
		case banzaicloudv1beta1.PerBrokerConfigurationState:
			brokerState.PerBrokerConfigurationState = s
		case map[string]banzaicloudv1beta1.VolumeState:
			if brokerState.GracefulActionState.VolumeStates == nil {
				brokerState.GracefulActionState.VolumeStates = make(map[string]banzaicloudv1beta1.VolumeState)
			}
			for mountPath, volumeState := range s {
				brokerState.GracefulActionState.VolumeStates[mountPath] = volumeState
			}
		case map[string]map[string]banzaicloudv1beta1.VolumeState:
			state := s[brokerID]
			if brokerState.GracefulActionState.VolumeStates == nil {
				brokerState.GracefulActionState.VolumeStates = make(map[string]banzaicloudv1beta1.VolumeState)
			}
			for mountPath, volumeState := range state {
				brokerState.GracefulActionState.VolumeStates[mountPath] = volumeState
			}
		case banzaicloudv1beta1.KafkaVersion:
			brokerState.Image = s.Image
			brokerState.Version = s.Version
		}
		brokersState[brokerID] = brokerState
	}
	cluster.Status.BrokersState = brokersState
}

// DeleteStatus deletes the given broker state from the CR
func DeleteStatus(c client.Client, brokerID string, cluster *banzaicloudv1beta1.KafkaCluster, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	brokerStatus := cluster.Status.BrokersState

	delete(brokerStatus, brokerID)

	cluster.Status.BrokersState = brokerStatus

	err := c.Status().Update(context.Background(), cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIff(err, "could not delete Kafka cluster broker %s state ", brokerID)
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating status")
		}
		brokerStatus = cluster.Status.BrokersState

		delete(brokerStatus, brokerID)

		cluster.Status.BrokersState = brokerStatus
		err = c.Status().Update(context.Background(), cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return errors.WrapIff(err, "could not delete Kafka clusters broker %s state ", brokerID)
		}
	}

	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info(fmt.Sprintf("Kafka broker %s state deleted", brokerID))
	return nil
}

// UpdateCRStatus updates the cluster state
func UpdateCRStatus(c client.Client, cluster *banzaicloudv1beta1.KafkaCluster, state interface{}, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	switch s := state.(type) {
	case banzaicloudv1beta1.ClusterState:
		cluster.Status.State = s
	case banzaicloudv1beta1.CruiseControlTopicStatus:
		cluster.Status.CruiseControlTopicStatus = s
	}

	err := c.Status().Update(context.Background(), cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIf(err, "could not update CR state")
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating status")
		}
		switch s := state.(type) {
		case banzaicloudv1beta1.ClusterState:
			cluster.Status.State = s
		case banzaicloudv1beta1.CruiseControlTopicStatus:
			cluster.Status.CruiseControlTopicStatus = s
		}

		err = c.Status().Update(context.Background(), cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return errors.WrapIf(err, "could not update CR state")
		}
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info("CR status updated", "status", state)
	return nil
}

// UpdateRollingUpgradeState updates the state of the cluster with rolling upgrade info
func UpdateRollingUpgradeState(c client.Client, cluster *banzaicloudv1beta1.KafkaCluster, time time.Time, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	timeStamp := time.Format("2006-01-02 15:04:05")
	cluster.Status.RollingUpgrade.LastSuccess = timeStamp

	err := c.Status().Update(context.Background(), cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIf(err, "could not update rolling upgrade state")
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating status")
		}

		cluster.Status.RollingUpgrade.LastSuccess = timeStamp

		err = c.Status().Update(context.Background(), cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return errors.WrapIf(err, "could not update rolling upgrade state")
		}
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info("Rolling upgrade status updated", "status", timeStamp)
	return nil
}

func UpdateListenerStatuses(ctx context.Context, c client.Client, cluster *banzaicloudv1beta1.KafkaCluster, logger logr.Logger,
	intListenerStatuses, extListenerStatuses map[string]banzaicloudv1beta1.ListenerStatusList) error {
	typeMeta := cluster.TypeMeta

	cluster.Status.ListenerStatuses = banzaicloudv1beta1.ListenerStatuses{
		InternalListeners: intListenerStatuses,
		ExternalListeners: extListenerStatuses,
	}

	err := c.Status().Update(ctx, cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(ctx, cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIf(err, "could not update listener status")
		}
		err := c.Get(ctx, types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating listener status")
		}

		cluster.Status.ListenerStatuses = banzaicloudv1beta1.ListenerStatuses{
			InternalListeners: intListenerStatuses,
			ExternalListeners: extListenerStatuses,
		}

		err = c.Status().Update(ctx, cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(ctx, cluster)
		}
		if err != nil {
			return errors.WrapIf(err, "could not update listener statuses")
		}
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info("updated listener statuses")
	return nil
}

func CreateInternalListenerStatuses(kafkaCluster *banzaicloudv1beta1.KafkaCluster, externalListenerStatus map[string]banzaicloudv1beta1.ListenerStatusList) (map[string]banzaicloudv1beta1.ListenerStatusList, map[string]banzaicloudv1beta1.ListenerStatusList) {
	intListenerStatuses := make(map[string]banzaicloudv1beta1.ListenerStatusList, len(kafkaCluster.Spec.ListenersConfig.InternalListeners))
	controllerIntListenerStatuses := make(map[string]banzaicloudv1beta1.ListenerStatusList)

	internalAddress := clientutil.GenerateKafkaAddressWithoutPort(kafkaCluster)
	for _, iListener := range kafkaCluster.Spec.ListenersConfig.InternalListeners {
		listenerStatusList := banzaicloudv1beta1.ListenerStatusList{}

		// add headless or any broker address
		name := "any-broker"
		if kafkaCluster.Spec.HeadlessServiceEnabled {
			name = "headless"
		}
		listenerStatusList = append(listenerStatusList, banzaicloudv1beta1.ListenerStatus{
			Name:    name,
			Address: fmt.Sprintf("%s:%d", internalAddress, iListener.ContainerPort),
		})

		// add addresses per broker
		for _, broker := range kafkaCluster.Spec.Brokers {
			var address = ""
			if iListener.ExternalListenerForHostname != "" && iListener.InternalStartingPort > 0 {
				if eListenerStatus, ok := externalListenerStatus[iListener.ExternalListenerForHostname]; ok {
					address = fmt.Sprintf("%s:%d", getHostnameForBrokerId(eListenerStatus, broker.Id),
						iListener.InternalStartingPort+broker.Id)
				}
			}

			if address == "" {
				if kafkaCluster.Spec.HeadlessServiceEnabled {
					address = fmt.Sprintf("%s-%d.%s-headless.%s.svc.%s:%d", kafkaCluster.Name, broker.Id, kafkaCluster.Name,
						kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), iListener.ContainerPort)
				} else {
					address = fmt.Sprintf("%s-%d.%s.svc.%s:%d", kafkaCluster.Name, broker.Id, kafkaCluster.Namespace,
						kafkaCluster.Spec.GetKubernetesClusterDomain(), iListener.ContainerPort)
				}
			}
			listenerStatusList = append(listenerStatusList, banzaicloudv1beta1.ListenerStatus{
				Name:    fmt.Sprintf("broker-%d", broker.Id),
				Address: address,
			})
		}

		if iListener.UsedForControllerCommunication {
			controllerIntListenerStatuses[iListener.Name] = listenerStatusList
		} else {
			intListenerStatuses[iListener.Name] = listenerStatusList
		}
	}

	return intListenerStatuses, controllerIntListenerStatuses
}

func getHostnameForBrokerId(eListenerStatusList v1beta1.ListenerStatusList, brokerId int32) string {
	for _, eListenerStatus := range eListenerStatusList {
		if eListenerStatus.Name == fmt.Sprintf("broker-%d", brokerId) {
			return strings.Split(eListenerStatus.Address, ":")[0]
		}
	}
	return ""
}
