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
	"time"

	"emperror.dev/errors"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func IsAlreadyOwnedError(err error) bool {
	return errors.As(err, controllerutil.AlreadyOwnedError{})
}

func IsMarkedForDeletion(m metav1.ObjectMeta) bool {
	return m.GetDeletionTimestamp() != nil
}

func UpdateBrokerStatus(c client.Client, brokerId string, cluster *banzaicloudv1alpha1.KafkaCluster, state interface{}, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	if cluster.Status.BrokersState == nil {
		switch s := state.(type) {
		case banzaicloudv1alpha1.RackAwarenessState:
			cluster.Status.BrokersState = map[string]banzaicloudv1alpha1.BrokerState{brokerId: {RackAwarenessState: s}}
		case banzaicloudv1alpha1.GracefulActionState:
			cluster.Status.BrokersState = map[string]banzaicloudv1alpha1.BrokerState{brokerId: {GracefulActionState: s}}
		case banzaicloudv1alpha1.ConfigurationState:
			cluster.Status.BrokersState = map[string]banzaicloudv1alpha1.BrokerState{brokerId: {ConfigurationState: s}}
		}
	} else if val, ok := cluster.Status.BrokersState[brokerId]; ok {
		switch s := state.(type) {
		case banzaicloudv1alpha1.RackAwarenessState:
			val.RackAwarenessState = s
		case banzaicloudv1alpha1.GracefulActionState:
			val.GracefulActionState = s
		case banzaicloudv1alpha1.ConfigurationState:
			val.ConfigurationState = s
		}
		cluster.Status.BrokersState[brokerId] = val
	} else {
		switch s := state.(type) {
		case banzaicloudv1alpha1.RackAwarenessState:
			cluster.Status.BrokersState[brokerId] = banzaicloudv1alpha1.BrokerState{RackAwarenessState: s}
		case banzaicloudv1alpha1.GracefulActionState:
			cluster.Status.BrokersState[brokerId] = banzaicloudv1alpha1.BrokerState{GracefulActionState: s}
		case banzaicloudv1alpha1.ConfigurationState:
			cluster.Status.BrokersState[brokerId] = banzaicloudv1alpha1.BrokerState{ConfigurationState: s}
		}
	}

	err := c.Status().Update(context.Background(), cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIff(err, "could not update Kafka broker %s state", brokerId)
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating status")
		}

		if cluster.Status.BrokersState == nil {
			switch s := state.(type) {
			case banzaicloudv1alpha1.RackAwarenessState:
				cluster.Status.BrokersState = map[string]banzaicloudv1alpha1.BrokerState{brokerId: {RackAwarenessState: s}}
			case banzaicloudv1alpha1.GracefulActionState:
				cluster.Status.BrokersState = map[string]banzaicloudv1alpha1.BrokerState{brokerId: {GracefulActionState: s}}
			case banzaicloudv1alpha1.ConfigurationState:
				cluster.Status.BrokersState = map[string]banzaicloudv1alpha1.BrokerState{brokerId: {ConfigurationState: s}}
			}
		} else if val, ok := cluster.Status.BrokersState[brokerId]; ok {
			switch s := state.(type) {
			case banzaicloudv1alpha1.RackAwarenessState:
				val.RackAwarenessState = s
			case banzaicloudv1alpha1.GracefulActionState:
				val.GracefulActionState = s
			case banzaicloudv1alpha1.ConfigurationState:
				val.ConfigurationState = s
			}
			cluster.Status.BrokersState[brokerId] = val
		} else {
			switch s := state.(type) {
			case banzaicloudv1alpha1.RackAwarenessState:
				cluster.Status.BrokersState[brokerId] = banzaicloudv1alpha1.BrokerState{RackAwarenessState: s}
			case banzaicloudv1alpha1.GracefulActionState:
				cluster.Status.BrokersState[brokerId] = banzaicloudv1alpha1.BrokerState{GracefulActionState: s}
			case banzaicloudv1alpha1.ConfigurationState:
				cluster.Status.BrokersState[brokerId] = banzaicloudv1alpha1.BrokerState{ConfigurationState: s}
			}
		}

		err = c.Status().Update(context.Background(), cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return errors.WrapIff(err, "could not update Kafka clusters broker %s state", brokerId)
		}
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info("Kafka cluster state updated")
	return nil
}

// DeleteStatus deletes the given broker state from the CR
func DeleteStatus(c client.Client, brokerId string, cluster *banzaicloudv1alpha1.KafkaCluster, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	brokerStatus := cluster.Status.BrokersState

	delete(brokerStatus, brokerId)

	cluster.Status.BrokersState = brokerStatus

	err := c.Status().Update(context.Background(), cluster)
	if apierrors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !apierrors.IsConflict(err) {
			return errors.WrapIff(err, "could not delete Kafka cluster broker %s state ", brokerId)
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return errors.WrapIf(err, "could not get config for updating status")
		}
		brokerStatus = cluster.Status.BrokersState

		delete(brokerStatus, brokerId)

		cluster.Status.BrokersState = brokerStatus
		err = c.Status().Update(context.Background(), cluster)
		if apierrors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return errors.WrapIff(err, "could not delete Kafka clusters broker %s state ", brokerId)
		}
	}

	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info(fmt.Sprintf("Kafka broker %s state deleted", brokerId))
	return nil
}

// UpdateCRStatus updates the cluster state
func UpdateCRStatus(c client.Client, cluster *banzaicloudv1alpha1.KafkaCluster, state interface{}, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	switch s := state.(type) {
	case banzaicloudv1alpha1.ClusterState:
		cluster.Status.State = s
	case banzaicloudv1alpha1.CruiseControlTopicStatus:
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
		case banzaicloudv1alpha1.ClusterState:
			cluster.Status.State = s
		case banzaicloudv1alpha1.CruiseControlTopicStatus:
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

func UpdateRollingUpgradeState(c client.Client, cluster *banzaicloudv1alpha1.KafkaCluster, time time.Time, logger logr.Logger) error {
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
