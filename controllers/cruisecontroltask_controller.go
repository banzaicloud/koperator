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

package controllers

import (
	"context"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kafkav1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
)

const (
	DefaultRequeueAfterTimeInSec = 20
)

// CruiseControlTaskReconciler reconciles a kafka cluster object
type CruiseControlTaskReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch

func (r *CruiseControlTaskReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)

	// Fetch the KafkaCluster instance
	instance := &kafkav1beta1.KafkaCluster{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconciled()
		}
		// Error reading the object - requeue the request.
		return requeueWithError(log, err.Error(), err)
	}

	log.V(1).Info("reconciling Cruise Control tasks")

	// Get all active tasks reported in status of Kafka Cluster CR
	tasksAndStates := getActiveTasksFromCluster(instance)
	if tasksAndStates.IsEmpty() {
		log.Info("no active tasks found in Kafka Cluster status")
		return reconciled()
	}

	scaler, err := scale.NewCruiseControlScaler(ctx, scale.CruiseControlURLFromKafkaCluster(instance))
	if err != nil {
		return requeueWithError(log, "failed to create Cruise Control Scaler instance", err)
	}

	if !scaler.IsUp() {
		log.Info("requeue event as Cruise Control is not available (yet)")
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	// Update task states with information from Cruise Control
	err = updateActiveTasks(scaler, tasksAndStates)
	if err != nil {
		log.Error(err, "requeue event as updating state of active tasks failed")
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	// Check if CruiseControl is ready as we cannot perform any operation until it is in ready state
	if status := scaler.Status(); status.InExecution() {
		log.Info("updating status of Kafka Cluster and requeue event as Cruise Control is in execution")
		if err := r.UpdateStatus(ctx, instance, tasksAndStates); err != nil {
			log.Error(err, "failed to update Kafka Cluster status")
		}
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	switch {
	case tasksAndStates.NumActiveTasksByOp(OperationAddBroker) > 0:
		addBrokerTasks := make([]*CruiseControlTask, 0)
		brokerIDs := make([]string, 0)
		for _, task := range tasksAndStates.GetActiveTasksByOp(OperationAddBroker) {
			brokerIDs = append(brokerIDs, task.BrokerID)
			addBrokerTasks = append(addBrokerTasks, task)
		}
		details := []interface{}{"operation", "add broker", "brokers", brokerIDs}

		result, err := scaler.AddBrokers(brokerIDs...)
		if err != nil {
			log.Error(err, "adding broker(s) to Kafka cluster via Cruise Control failed", details...)
		}

		for _, task := range addBrokerTasks {
			if task == nil {
				continue
			}
			task.FromResult(result)
		}

	case tasksAndStates.NumActiveTasksByOp(OperationRemoveBroker) > 0:
		var removeTask *CruiseControlTask
		for _, task := range tasksAndStates.GetActiveTasksByOp(OperationRemoveBroker) {
			removeTask = task
			break
		}

		details := []interface{}{"operation", "remove broker", "brokers", removeTask.BrokerID}

		result, err := scaler.RemoveBrokers(removeTask.BrokerID)
		if err != nil {
			log.Error(err, "removing broker(s) from Kafka cluster via Cruise Control failed", details...)
		}
		removeTask.FromResult(result)

	case tasksAndStates.NumActiveTasksByOp(OperationRebalanceDisks) > 0:
		logDirsByBroker, err := scaler.LogDirsByBroker()
		if err != nil {
			log.Error(err, "failed to get list of volumes per broker from Cruise Control")
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}

		brokerIDs := make([]string, 0)
		for _, task := range tasksAndStates.GetActiveTasksByOp(OperationRebalanceDisks) {
			if task == nil || task.IsDone() {
				continue
			}
			if onlineDirs, ok := logDirsByBroker[task.BrokerID][scale.LogDirStateOnline]; ok {
				found := true
				for _, dir := range onlineDirs {
					if !strings.HasPrefix(strings.TrimSpace(dir), strings.TrimSpace(task.Volume)) {
						found = false
					}
				}
				if found {
					brokerIDs = append(brokerIDs, task.BrokerID)
				}
			} else {
				task.Err = "log dir is not available in Cruise Control"
			}
		}

		details := []interface{}{"operation", "rebalance disks", "brokers", brokerIDs}

		result, err := scaler.RebalanceDisks(brokerIDs...)
		if err != nil {
			log.Error(err, "re-balancing disk(s) in Kafka cluster via Cruise Control failed", details...)
		}

		for _, task := range tasksAndStates.GetActiveTasksByOp(OperationRebalanceDisks) {
			if task == nil {
				continue
			}
			task.FromResult(result)
		}
	}

	if err = r.UpdateStatus(ctx, instance, tasksAndStates); err != nil {
		log.Error(err, "failed to update Kafka Cluster status")
	}
	return requeueAfter(DefaultRequeueAfterTimeInSec)
}

// UpdateStatus updates the Status of the provided kafkav1beta1.KafkaCluster instance with the status of the tasks
// from a CruiseControlTasksAndStates and sends the updates to the Kubernetes API if any changes in the Status field is
// detected. Otherwise, this step is skipped.
func (r *CruiseControlTaskReconciler) UpdateStatus(ctx context.Context, instance *kafkav1beta1.KafkaCluster,
	taskAndStates *CruiseControlTasksAndStates) error {
	log := logr.FromContextOrDiscard(ctx)

	currentStatus := instance.Status.DeepCopy()
	taskAndStates.SyncState(instance)
	if reflect.DeepEqual(currentStatus, instance.Status) {
		log.Info("there are no updates to apply to Kafka Cluster Status")
		return nil
	}

	err := r.Client.Status().Update(ctx, instance)
	if err != nil {
		if apiErrors.IsConflict(err) {
			err = r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, instance)
			if err != nil {
				return errors.WithMessage(err, "failed to get updated Kafka Cluster CR before updating its status")
			}
			taskAndStates.SyncState(instance)
			err = r.Client.Status().Update(ctx, instance)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// SetupCruiseControlWithManager registers cruise control controller to the manager
func SetupCruiseControlWithManager(mgr ctrl.Manager) *ctrl.Builder {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1beta1.KafkaCluster{}).
		WithEventFilter(SkipClusterRegistryOwnedResourcePredicate{}).
		Named("CruiseControl")

	builder.WithEventFilter(
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				if _, ok := e.ObjectNew.(*kafkav1beta1.KafkaCluster); ok {
					oldObj := e.ObjectOld.(*kafkav1beta1.KafkaCluster)
					newObj := e.ObjectNew.(*kafkav1beta1.KafkaCluster)
					if !reflect.DeepEqual(oldObj.Status.BrokersState, newObj.Status.BrokersState) ||
						oldObj.GetDeletionTimestamp() != newObj.GetDeletionTimestamp() ||
						oldObj.GetGeneration() != newObj.GetGeneration() {
						return true
					}
					return false
				}
				return true
			},
		})

	return builder
}

// getActiveTasksFromCluster returns a CruiseControlTasksAndStates instance which stores active (operation needed) tasks
// collected from the status field of kafkav1beta1.KafkaCluster instance.
func getActiveTasksFromCluster(instance *kafkav1beta1.KafkaCluster) *CruiseControlTasksAndStates {
	tasksAndStates := newCruiseControlTasksAndStates()

	for brokerId, brokerStatus := range instance.Status.BrokersState {
		if brokerStatus.GracefulActionState.CruiseControlState.IsActive() {
			state := brokerStatus.GracefulActionState
			switch {
			case state.CruiseControlState.IsUpscale():
				t := &CruiseControlTask{
					TaskID:      state.CruiseControlTaskId,
					BrokerID:    brokerId,
					BrokerState: state.CruiseControlState,
					StartedAt:   state.TaskStarted,
					Operation:   OperationAddBroker,
					Err:         state.ErrorMessage,
				}
				tasksAndStates.Add(t)
			case state.CruiseControlState.IsDownscale():
				t := &CruiseControlTask{
					TaskID:      state.CruiseControlTaskId,
					BrokerID:    brokerId,
					BrokerState: state.CruiseControlState,
					StartedAt:   state.TaskStarted,
					Operation:   OperationRemoveBroker,
					Err:         state.ErrorMessage,
				}
				tasksAndStates.Add(t)
			}
		}

		for mountPath, volumeState := range brokerStatus.GracefulActionState.VolumeStates {
			if volumeState.CruiseControlVolumeState.IsActive() {
				t := &CruiseControlTask{
					TaskID:      volumeState.CruiseControlTaskId,
					BrokerID:    brokerId,
					StartedAt:   volumeState.TaskStarted,
					Volume:      mountPath,
					VolumeState: volumeState.CruiseControlVolumeState,
					Operation:   OperationRebalanceDisks,
					Err:         volumeState.ErrorMessage,
				}
				tasksAndStates.Add(t)
			}
		}
	}
	return tasksAndStates
}

// updateActiveTasks updates the state of the tasks from the CruiseControlTasksAndStates instance by getting their
// status from CruiseControl using the provided scale.CruiseControlScaler.
func updateActiveTasks(scaler scale.CruiseControlScaler, tasksAndStates *CruiseControlTasksAndStates) error {
	tasks, err := scaler.GetUserTasks()
	if err != nil {
		return err
	}

	taskResultsByID := make(map[string]*scale.Result, len(tasks))
	for _, task := range tasks {
		taskResultsByID[task.TaskID] = task
	}

	for _, task := range tasksAndStates.tasks {
		if task == nil || task.TaskID == "" {
			continue
		}

		taskResult, ok := taskResultsByID[task.TaskID]
		if !ok {
			continue
		}
		task.FromResult(taskResult)
	}
	return nil
}
