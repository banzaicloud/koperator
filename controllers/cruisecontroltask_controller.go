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
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiutil "github.com/banzaicloud/koperator/api/util"
	banzaiv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaiv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
	"github.com/banzaicloud/koperator/pkg/util"
)

const (
	DefaultRequeueAfterTimeInSec = 20
)

// CruiseControlTaskReconciler reconciles a kafka cluster object
type CruiseControlTaskReconciler struct {
	client.Client
	DirectClient client.Reader
	Scheme       *runtime.Scheme
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch

func (r *CruiseControlTaskReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)

	// Fetch the KafkaCluster instance
	instance := &banzaiv1beta1.KafkaCluster{}
	err := r.DirectClient.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconciled()
		}
		// Error reading the object - requeue the request.
		return requeueWithError(log, err.Error(), err)
	}

	log.Info("reconciling Cruise Control tasks")

	// Get all active tasks reported in status of Kafka Cluster CR
	tasksAndStates := getActiveTasksFromCluster(instance)
	if tasksAndStates.IsEmpty() {
		log.Info("no active tasks found in Kafka Cluster status")
		return reconciled()
	}

	ccOperationList := banzaiv1alpha1.CruiseControlOperationList{}
	matchingLabels := client.MatchingLabels(apiutil.LabelsForKafka(instance.Name))
	err = r.DirectClient.List(ctx, &ccOperationList, client.ListOption(client.InNamespace(request.Namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return requeueWithError(log, err.Error(), err)
	}
	// Selecting owned CruiseControlOperations
	var ccOperations []*banzaiv1alpha1.CruiseControlOperation
	for i := range ccOperationList.Items {
		operation := &ccOperationList.Items[i]
		for _, ownerRef := range operation.GetOwnerReferences() {
			if ownerRef.Name == instance.Name {
				ccOperations = append(ccOperations, operation)
				break
			}
		}
	}

	// Update task states with information from Cruise Control
	err = updateActiveTasks(tasksAndStates, ccOperations)
	if err != nil {
		log.Error(err, "requeue event as updating state of active tasks failed")
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	scaler, err := scale.NewCruiseControlScaler(ctx, scale.CruiseControlURLFromKafkaCluster(instance))
	if err != nil {
		return requeueWithError(log, "failed to create Cruise Control Scaler instance", err)
	}

	switch {
	case tasksAndStates.NumActiveTasksByOp(banzaiv1alpha1.OperationAddBroker) > 0:
		brokerIDs := make([]string, 0)
		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationAddBroker) {
			brokerIDs = append(brokerIDs, task.BrokerID)
		}
		if len(brokerIDs) == 0 {
			break
		}
		details := []interface{}{"operation", "add broker", "brokers", brokerIDs}

		unavailableBrokers, err := checkBrokersAvailability(scaler, log, brokerIDs)
		if err != nil {
			log.Error(err, "could not get unavailable brokers for upscale")
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}
		if len(unavailableBrokers) > 0 {
			log.Info("requeue... broker(s) are not ready for upscale", "brokerIDs", unavailableBrokers)
			// This requeue is not necessary because the cruisecontrloperation controller retry the errored task
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}

		cruiseControlOpRef, err := r.addBrokers(ctx, instance, brokerIDs)
		if err != nil {
			log.Error(err, "creating CruiseControlOperation for upscale has failed", details...)
		}

		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationAddBroker) {
			if task == nil {
				continue
			}
			task.SetCruiseControlOperationRef(cruiseControlOpRef)
			task.SetStateScheduled()
		}
	case tasksAndStates.NumActiveTasksByOp(banzaiv1alpha1.OperationRemoveBroker) > 0:
		var removeTask *CruiseControlTask
		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRemoveBroker) {
			removeTask = task
			break
		}

		if removeTask == nil {
			break
		}

		details := []interface{}{"operation", "remove broker", "brokers", removeTask.BrokerID}

		cruiseControlOpRef, err := r.removeBroker(ctx, instance, removeTask.BrokerID)
		if err != nil {
			log.Error(err, "creating CruiseControlOperation for downscale has failed", details...)
		}
		removeTask.SetCruiseControlOperationRef(cruiseControlOpRef)
		removeTask.SetStateScheduled()

	case tasksAndStates.NumActiveTasksByOp(banzaiv1alpha1.OperationRebalance) > 0:
		logDirsByBroker, err := scaler.LogDirsByBroker()
		if err != nil {
			log.Error(err, "failed to get list of volumes per broker from Cruise Control")
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}
		brokerIDs := make([]string, 0)
		unavailableBrokerIDs := make([]string, 0)
		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRebalance) {
			brokerIDs = append(brokerIDs, task.BrokerID)
			found := false
			if onlineDirs, ok := logDirsByBroker[task.BrokerID][scale.LogDirStateOnline]; ok {
				for _, dir := range onlineDirs {
					if strings.HasPrefix(strings.TrimSpace(dir), strings.TrimSpace(task.Volume)) {
						found = true
					}
				}
				if !found {
					unavailableBrokerIDs = append(unavailableBrokerIDs, task.BrokerID)
				}
			}
		}
		if len(unavailableBrokerIDs) > 0 {
			log.Info("requeue as there are offline broker log dirs for rebalance", "brokerIDs", unavailableBrokerIDs)
			// This requeue is not necessary because the cruisecontrloperation controller retry the errored task
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}
		if len(unavailableBrokerIDs) > 0 {
			log.Info("requeue as there are offline broker log dirs for rebalance", "brokerIDs", unavailableBrokerIDs)
			// This requeue is not necessary because the cruisecontrloperation controller retry the errored task
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}

		details := []interface{}{"operation", "rebalance disks", "brokers", brokerIDs}
		cruiseControlOpRef, err := r.rebalanceDisks(ctx, instance, brokerIDs)
		if err != nil {
			log.Error(err, "creating CruiseControlOperation for re-balancing disks has failed", details...)
		}
		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRebalance) {
			if task == nil {
				continue
			}
			task.SetCruiseControlOperationRef(cruiseControlOpRef)
			task.SetStateScheduled()
		}
	}

	if err = r.UpdateStatus(ctx, instance, tasksAndStates); err != nil {
		log.Error(err, "failed to update Kafka Cluster status")
	}

	return reconciled()
}

func checkBrokersAvailability(scaler scale.CruiseControlScaler, log logr.Logger, brokerIDs []string) ([]string, error) {
	states := []scale.KafkaBrokerState{scale.KafkaBrokerAlive, scale.KafkaBrokerNew}
	availableBrokers, err := scaler.BrokersWithState(states...)
	if err != nil {
		return nil, errors.WrapIf(err, "failed to retrieve list of available brokers from Cruise Control")
	}

	availableBrokersMap := scale.StringSliceToMap(availableBrokers)
	unavailableBrokerIDs := make([]string, 0, len(brokerIDs))
	for _, id := range brokerIDs {
		if _, ok := availableBrokersMap[id]; !ok {
			unavailableBrokerIDs = append(unavailableBrokerIDs, id)
		}
	}

	return unavailableBrokerIDs, nil
}

func (r *CruiseControlTaskReconciler) addBrokers(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, bokerIDs []string) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, banzaiv1alpha1.OperationAddBroker, bokerIDs)
}

func (r *CruiseControlTaskReconciler) removeBroker(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, brokerID string) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, banzaiv1alpha1.OperationRemoveBroker, []string{brokerID})
}

func (r *CruiseControlTaskReconciler) rebalanceDisks(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, bokerIDs []string) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, banzaiv1alpha1.OperationRebalance, bokerIDs)
}

func (r *CruiseControlTaskReconciler) createCCOperation(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, errorPolicy banzaiv1alpha1.ErrorPolicyType,
	operationType banzaiv1alpha1.CruiseControlTaskOperation, bokerIDs []string) (corev1.LocalObjectReference, error) {
	operation := &banzaiv1alpha1.CruiseControlOperation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-%s-", kafkaCluster.Name, strings.ReplaceAll(string(operationType), "_", "")),
			Namespace:    kafkaCluster.Namespace,
			Labels:       apiutil.LabelsForKafka(kafkaCluster.Name),
		},
		Spec: banzaiv1alpha1.CruiseControlOperationSpec{
			ErrorPolicy: errorPolicy,
		},
	}
	if err := controllerutil.SetControllerReference(kafkaCluster, operation, r.Scheme); err != nil {
		return corev1.LocalObjectReference{}, err
	}
	if err := r.Client.Create(ctx, operation); err != nil {
		return corev1.LocalObjectReference{}, err
	}

	operation.Status.CurrentTask = &banzaiv1alpha1.CruiseControlTask{
		Operation: operationType,
		Parameters: map[string]string{
			"exclude_recently_demoted_brokers": "true",
			"exclude_recently_removed_brokers": "true",
		},
	}

	if operationType == banzaiv1alpha1.OperationRebalance {
		operation.Status.CurrentTask.Parameters["destination_broker_ids"] = strings.Join(bokerIDs, ",")
	} else {
		operation.Status.CurrentTask.Parameters["brokerid"] = strings.Join(bokerIDs, ",")
	}

	if err := r.Status().Update(ctx, operation); err != nil {
		return corev1.LocalObjectReference{}, err
	}
	return corev1.LocalObjectReference{
		Name: operation.Name,
	}, nil
}

// UpdateStatus updates the Status of the provided banzaiv1beta1.KafkaCluster instance with the status of the tasks
// from a CruiseControlTasksAndStates and sends the updates to the Kubernetes API if any changes in the Status field is
// detected. Otherwise, this step is skipped.
func (r *CruiseControlTaskReconciler) UpdateStatus(ctx context.Context, instance *banzaiv1beta1.KafkaCluster,
	taskAndStates *CruiseControlTasksAndStates) error {
	log := logr.FromContextOrDiscard(ctx)

	currentStatus := instance.Status.DeepCopy()
	taskAndStates.SyncState(instance)
	if reflect.DeepEqual(currentStatus, instance.Status) {
		log.Info("there are no updates to apply to Kafka Cluster Status")
		return nil
	}

	if err := util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
		log.Info("Updating status....")
		err := r.Status().Update(ctx, instance)
		if apiErrors.IsConflict(err) {
			err := r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, instance)
			if err != nil {
				return errors.WithMessage(err, "failed to get updated Kafka Cluster CR before updating its status")
			}
			taskAndStates.SyncState(instance)
		}
		return err
	}); err != nil {
		return err
	}

	return nil
}

// SetupCruiseControlWithManager registers cruise control controller to the manager
func SetupCruiseControlWithManager(mgr ctrl.Manager) *ctrl.Builder {
	cruiseControlOperationPredicate := predicate.Funcs{
		// We dont reconcile when there is no operation state
		CreateFunc: func(e event.CreateEvent) bool {
			obj := e.Object.(*banzaiv1alpha1.CruiseControlOperation)
			return obj.GetCurrentTaskState() != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*banzaiv1alpha1.CruiseControlOperation)
			newObj := e.ObjectNew.(*banzaiv1alpha1.CruiseControlOperation)
			if !reflect.DeepEqual(oldObj.GetCurrentTask(), newObj.GetCurrentTask()) ||
				oldObj.GetDeletionTimestamp() != newObj.GetDeletionTimestamp() ||
				oldObj.GetGeneration() != newObj.GetGeneration() {
				return true
			}
			return false
		},
	}

	kafkaClusterPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if _, ok := e.ObjectNew.(*banzaiv1beta1.KafkaCluster); ok {
				oldObj := e.ObjectOld.(*banzaiv1beta1.KafkaCluster)
				newObj := e.ObjectNew.(*banzaiv1beta1.KafkaCluster)
				if !reflect.DeepEqual(oldObj.Status.BrokersState, newObj.Status.BrokersState) ||
					oldObj.GetDeletionTimestamp() != newObj.GetDeletionTimestamp() ||
					oldObj.GetGeneration() != newObj.GetGeneration() {
					return true
				}
				return false
			}
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&banzaiv1beta1.KafkaCluster{}).
		WithEventFilter(SkipClusterRegistryOwnedResourcePredicate{}).
		WithEventFilter(kafkaClusterPredicate).
		Owns(&banzaiv1alpha1.CruiseControlOperation{}, builder.WithPredicates(cruiseControlOperationPredicate)).
		Named("CruiseControl")
}

// getActiveTasksFromCluster returns a CruiseControlTasksAndStates instance which stores active (operation needed) tasks
// collected from the status field of banzaiv1beta1.KafkaCluster instance.
func getActiveTasksFromCluster(instance *banzaiv1beta1.KafkaCluster) *CruiseControlTasksAndStates {
	tasksAndStates := newCruiseControlTasksAndStates()

	for brokerId, brokerStatus := range instance.Status.BrokersState {
		if brokerStatus.GracefulActionState.CruiseControlState.IsActive() {
			state := brokerStatus.GracefulActionState
			switch {
			case state.CruiseControlState.IsUpscale():
				t := &CruiseControlTask{
					BrokerID:                        brokerId,
					BrokerState:                     state.CruiseControlState,
					Operation:                       banzaiv1alpha1.OperationAddBroker,
					CruiseControlOperationReference: brokerStatus.GracefulActionState.CruiseControlOperationReference,
				}
				tasksAndStates.Add(t)
			case state.CruiseControlState.IsDownscale():
				t := &CruiseControlTask{
					BrokerID:                        brokerId,
					BrokerState:                     state.CruiseControlState,
					Operation:                       banzaiv1alpha1.OperationRemoveBroker,
					CruiseControlOperationReference: brokerStatus.GracefulActionState.CruiseControlOperationReference,
				}
				tasksAndStates.Add(t)
			}
		}

		for mountPath, volumeState := range brokerStatus.GracefulActionState.VolumeStates {
			if volumeState.CruiseControlVolumeState.IsActive() {
				t := &CruiseControlTask{
					BrokerID:                        brokerId,
					Volume:                          mountPath,
					VolumeState:                     volumeState.CruiseControlVolumeState,
					Operation:                       banzaiv1alpha1.OperationRebalance,
					CruiseControlOperationReference: volumeState.CruiseControlOperationReference,
				}
				tasksAndStates.Add(t)
			}
		}
	}
	return tasksAndStates
}

// updateActiveTasks updates the state of the tasks from the CruiseControlTasksAndStates instance by getting their
// status from CruiseControlOperation
func updateActiveTasks(tasksAndStates *CruiseControlTasksAndStates, ccOperations []*banzaiv1alpha1.CruiseControlOperation) error {
	ccOperationMap := make(map[string]*banzaiv1alpha1.CruiseControlOperation)
	for i := range ccOperations {
		operation := ccOperations[i]
		ccOperationMap[operation.Name] = operation
	}

	for _, task := range tasksAndStates.tasks {
		if task == nil || task.CruiseControlOperationReference == nil {
			continue
		}

		task.FromResult(ccOperationMap[task.CruiseControlOperationReference.Name])
	}
	return nil
}
