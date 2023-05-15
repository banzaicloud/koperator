// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	koperatorccconf "github.com/banzaicloud/koperator/pkg/resources/cruisecontrol"
	"github.com/banzaicloud/koperator/pkg/scale"
	"github.com/banzaicloud/koperator/pkg/util"
)

const (
	DefaultRequeueAfterTimeInSec = 20
	BrokerCapacityDisk           = "DISK"
	BrokerCapacity               = "capacity"
	True                         = "true"
)

// CruiseControlTaskReconciler reconciles a kafka cluster object
type CruiseControlTaskReconciler struct {
	client.Client
	// DirectClient here is needed because when the next reconciliation is happened instantly after status update then
	// the changes in some cases will not be in the resource otherwise.
	DirectClient client.Reader
	Scheme       *runtime.Scheme
	ScaleFactory func(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster) (scale.CruiseControlScaler, error)
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch

//nolint:funlen,gocyclo
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
	updateActiveTasks(tasksAndStates, ccOperations)

	if err = r.UpdateStatus(ctx, instance, tasksAndStates); err != nil {
		return requeueWithError(log, "failed to update Kafka Cluster status", err)
	}

	scaler, err := r.ScaleFactory(ctx, instance)
	if err != nil {
		return requeueWithError(log, "failed to create Cruise Control Scaler instance", err)
	}

	operationTTLSecondsAfterFinished := instance.Spec.CruiseControlConfig.CruiseControlOperationSpec.GetTTLSecondsAfterFinished()

	switch {
	case tasksAndStates.NumActiveTasksByOp(banzaiv1alpha1.OperationAddBroker) > 0:
		brokerIDs := make([]string, 0)
		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationAddBroker) {
			brokerIDs = append(brokerIDs, task.BrokerID)
		}
		if len(brokerIDs) == 0 {
			break
		}

		unavailableBrokers, err := getUnavailableBrokers(ctx, scaler, brokerIDs)
		if err != nil {
			log.Error(err, "could not get unavailable brokers for upscale")
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}
		if len(unavailableBrokers) > 0 {
			log.Info("requeue as broker(s) are not ready for upscale", "brokerIDs", unavailableBrokers)
			// This requeue is not necessary because the cruisecontroloperation controller retries the errored task
			// but in this case there will be GracefulUpscaleCompletedWithError status in the kafkaCluster's status.
			// To avoid that requeue is here until brokers come up.
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}

		cruiseControlOpRef, err := r.addBrokers(ctx, instance, operationTTLSecondsAfterFinished, brokerIDs)
		if err != nil {
			return requeueWithError(log, fmt.Sprintf("creating CruiseControlOperation for upscale has failed, brokerIDs: %s", brokerIDs), err)
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

		cruiseControlOpRef, err := r.removeBroker(ctx, instance, operationTTLSecondsAfterFinished, removeTask.BrokerID)
		if err != nil {
			return requeueWithError(log, fmt.Sprintf("creating CruiseControlOperation for downscale has failed, brokerID: %s", removeTask.BrokerID), err)
		}

		removeTask.SetCruiseControlOperationRef(cruiseControlOpRef)
		removeTask.SetStateScheduled()

	case tasksAndStates.NumActiveTasksByOp(banzaiv1alpha1.OperationRemoveDisks) > 0:
		brokerLogDirsToRemove := make(map[string][]string)
		logDirsByBroker, err := scaler.LogDirsByBroker(ctx)
		if err != nil {
			return requeueWithError(log, "failed to get list of brokerIdsToLogDirs per broker from Cruise Control", err)
		}

		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRemoveDisks) {
			if task == nil {
				continue
			}

			brokerID := task.BrokerID
			volume := task.Volume
			if _, ok := brokerLogDirsToRemove[brokerID]; !ok {
				brokerLogDirsToRemove[brokerID] = []string{}
			}

			found := false
			if onlineDirs, ok := logDirsByBroker[brokerID][scale.LogDirStateOnline]; ok {
				for _, dir := range onlineDirs {
					if strings.HasPrefix(strings.TrimSpace(dir), strings.TrimSpace(volume)) {
						brokerLogDirsToRemove[brokerID] = append(brokerLogDirsToRemove[brokerID], dir)
						found = true
						break
					}
				}
			}

			if !found {
				return requeueWithError(log, fmt.Sprintf("volume %s not found for broker %s in CC online log dirs", volume, brokerID), errors.New("log dir not found"))
			}
		}

		// create the cruise control operation
		cruiseControlOpRef, err := r.removeDisks(ctx, instance, operationTTLSecondsAfterFinished, brokerLogDirsToRemove)
		if err != nil {
			return requeueWithError(log, fmt.Sprintf("creating CruiseControlOperation for disk removal has failed, brokerID and brokerIdsToLogDirs: %s", brokerLogDirsToRemove), err)
		}

		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRemoveDisks) {
			if task == nil {
				continue
			}

			task.SetCruiseControlOperationRef(cruiseControlOpRef)
			task.SetStateScheduled()
		}

	case tasksAndStates.NumActiveTasksByOp(banzaiv1alpha1.OperationRebalance) > 0:
		brokerIDs := make([]string, 0)
		for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRebalance) {
			brokerIDs = append(brokerIDs, task.BrokerID)
		}

		unavailableBrokerIDs, err := checkBrokerLogDirsAvailability(ctx, scaler, tasksAndStates)
		if err != nil {
			log.Error(err, "failed to get unavailable brokers at rebalance")
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}

		if len(unavailableBrokerIDs) > 0 {
			log.Info("requeue as there are offline broker log dirs for rebalance", "brokerIDs", unavailableBrokerIDs)
			// This requeue is not necessary because the cruisecontrloperation controller retries the errored task
			// but in this case there will be GracefulUpscaleCompletedWithError status in the kafkaCluster's status.
			// To avoid that requeue is here until brokers with the new data logs come up.
			return requeueAfter(DefaultRequeueAfterTimeInSec)
		}

		allBrokerIDs := make([]string, 0, len(instance.Spec.Brokers))
		for i := range instance.Spec.Brokers {
			allBrokerIDs = append(allBrokerIDs, fmt.Sprint(instance.Spec.Brokers[i].Id))
		}
		// we can do rebalance between the broker's disks when JBOD capacity config is used
		// this selector distinguishes the JBOD brokers from the not JBOD brokers
		// we need to search in all brokers to find out if there are any not JBOD brokers because
		// CC cannot do disk rebalance when at least one of the brokers has not JBOD capacity configuration
		_, brokersNotJBOD, err := brokersJBODSelector(allBrokerIDs, instance.Spec.CruiseControlConfig.CapacityConfig)
		if err != nil {
			return requeueWithError(log, "failed to determine which broker using JBOD or not JBOD capacity configuration at rebalance operation", err)
		}

		var cruiseControlOpRef corev1.LocalObjectReference
		// when there is at least one not JBOD broker in the kafka cluster CC cannot do the disk rebalance :(
		if len(brokersNotJBOD) > 0 {
			cruiseControlOpRef, err = r.rebalanceDisks(ctx, instance, operationTTLSecondsAfterFinished, brokerIDs, false)
			if err != nil {
				return requeueWithError(log, fmt.Sprintf("creating CruiseControlOperation for re-balancing not JBOD disks has failed, brokerIDs: %s", brokerIDs), err)
			}
		} else {
			cruiseControlOpRef, err = r.rebalanceDisks(ctx, instance, operationTTLSecondsAfterFinished, brokerIDs, true)
			if err != nil {
				return requeueWithError(log, fmt.Sprintf("creating CruiseControlOperation for re-balancing not JBOD disks has failed, brokerIDs: %s", brokerIDs), err)
			}
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
		return requeueWithError(log, "failed to update Kafka Cluster status", err)
	}

	return reconciled()
}

func checkBrokerLogDirsAvailability(ctx context.Context, scaler scale.CruiseControlScaler, tasksAndStates *CruiseControlTasksAndStates) (unavailableBrokerIDs []string, err error) {
	logDirsByBroker, err := scaler.LogDirsByBroker(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get list of volumes per broker from Cruise Control")
	}

	unavailableBrokerIDs = make([]string, 0)
	for _, task := range tasksAndStates.GetActiveTasksByOp(banzaiv1alpha1.OperationRebalance) {
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
	return unavailableBrokerIDs, nil
}

func getUnavailableBrokers(ctx context.Context, scaler scale.CruiseControlScaler, brokerIDs []string) ([]string, error) {
	states := []scale.KafkaBrokerState{scale.KafkaBrokerAlive, scale.KafkaBrokerNew}
	// This can result NullPointerException when the capacity calculation is missing for a broker in the cruisecontrol configmap
	availableBrokers, err := scaler.BrokersWithState(ctx, states...)
	if err != nil {
		return nil, errors.WrapIff(err, "failed to retrieve list of available brokers from Cruise Control")
	}

	availableBrokersMap := make(map[string]bool, len(availableBrokers))
	for _, id := range availableBrokers {
		availableBrokersMap[id] = true
	}

	unavailableBrokerIDs := make([]string, 0, len(brokerIDs))
	for _, id := range brokerIDs {
		if _, ok := availableBrokersMap[id]; !ok {
			unavailableBrokerIDs = append(unavailableBrokerIDs, id)
		}
	}

	return unavailableBrokerIDs, nil
}

func (r *CruiseControlTaskReconciler) addBrokers(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, ttlSecondsAfterFinished *int, bokerIDs []string) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, ttlSecondsAfterFinished, banzaiv1alpha1.OperationAddBroker, bokerIDs, false, nil)
}

func (r *CruiseControlTaskReconciler) removeBroker(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, ttlSecondsAfterFinished *int, brokerID string) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, ttlSecondsAfterFinished, banzaiv1alpha1.OperationRemoveBroker, []string{brokerID}, false, nil)
}

func (r *CruiseControlTaskReconciler) removeDisks(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, ttlSecondsAfterFinished *int, brokerIdsToRemovedLogDirs map[string][]string) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, ttlSecondsAfterFinished, banzaiv1alpha1.OperationRemoveDisks, nil, false, brokerIdsToRemovedLogDirs)
}

func (r *CruiseControlTaskReconciler) rebalanceDisks(ctx context.Context, kafkaCluster *banzaiv1beta1.KafkaCluster, ttlSecondsAfterFinished *int, bokerIDs []string, isJBOD bool) (corev1.LocalObjectReference, error) {
	return r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, ttlSecondsAfterFinished, banzaiv1alpha1.OperationRebalance, bokerIDs, isJBOD, nil)
}

//nolint:unparam
func (r *CruiseControlTaskReconciler) createCCOperation(
	ctx context.Context,
	kafkaCluster *banzaiv1beta1.KafkaCluster,
	errorPolicy banzaiv1alpha1.ErrorPolicyType,
	ttlSecondsAfterFinished *int,
	operationType banzaiv1alpha1.CruiseControlTaskOperation,
	brokerIDs []string,
	isJBOD bool,
	logDirsByBrokerID map[string][]string,
) (corev1.LocalObjectReference, error) {
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

	if ttlSecondsAfterFinished != nil {
		operation.Spec.TTLSecondsAfterFinished = ttlSecondsAfterFinished
	}

	if err := controllerutil.SetControllerReference(kafkaCluster, operation, r.Scheme); err != nil {
		return corev1.LocalObjectReference{}, err
	}
	if err := r.Client.Create(ctx, operation); err != nil {
		return corev1.LocalObjectReference{}, err
	}

	operation.Status.CurrentTask = &banzaiv1alpha1.CruiseControlTask{
		Operation:  operationType,
		Parameters: make(map[string]string),
	}

	if operationType != banzaiv1alpha1.OperationRemoveDisks {
		operation.Status.CurrentTask.Parameters[scale.ParamExcludeDemoted] = True
		operation.Status.CurrentTask.Parameters[scale.ParamExcludeRemoved] = True
	}

	switch {
	case operationType == banzaiv1alpha1.OperationRebalance:
		operation.Status.CurrentTask.Parameters[scale.ParamDestbrokerIDs] = strings.Join(brokerIDs, ",")
		if isJBOD {
			operation.Status.CurrentTask.Parameters[scale.ParamRebalanceDisk] = True
		}
	case operationType == banzaiv1alpha1.OperationRemoveDisks:
		pairs := make([]string, 0, len(logDirsByBrokerID))
		for brokerID, logDirs := range logDirsByBrokerID {
			for _, logDir := range logDirs {
				pair := fmt.Sprintf("%s-%s", brokerID, logDir)
				pairs = append(pairs, pair)
			}
		}
		operation.Status.CurrentTask.Parameters[scale.ParamBrokerIDAndLogDirs] = strings.Join(pairs, ",")
	default:
		operation.Status.CurrentTask.Parameters[scale.ParamBrokerID] = strings.Join(brokerIDs, ",")
	}

	if err := r.Status().Update(ctx, operation); err != nil {
		return corev1.LocalObjectReference{}, err
	}
	return corev1.LocalObjectReference{
		Name: operation.Name,
	}, nil
}

// brokersJBODSelector filters out the JBOD and not JBOD brokers from a broker list based on the capacityConfig
func brokersJBODSelector(brokerIDs []string, capacityConfigJSON string) (brokersJBOD []string, brokersNotJBOD []string, err error) {
	// JBOD is generated by default
	if capacityConfigJSON == "" {
		return brokerIDs, nil, nil
	}
	brokerIsJBOD := make(map[string]bool)
	for _, brokerID := range brokerIDs {
		brokerIsJBOD[brokerID] = true
	}

	var capacityConfig koperatorccconf.JBODInvariantCapacityConfig
	err = json.Unmarshal([]byte(capacityConfigJSON), &capacityConfig)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not unmarshal the user-provided broker capacity config")
	}
	for _, brokerCapacity := range capacityConfig.Capacities {
		brokerCapacityMap, ok := brokerCapacity.(map[string]interface{})
		if !ok {
			continue
		}

		brokerId, ok, err := unstructured.NestedString(brokerCapacityMap, banzaiv1beta1.BrokerIdLabelKey)
		if err != nil {
			return nil, nil, errors.WrapIfWithDetails(err,
				"could not retrieve broker Id from broker capacity configuration",
				"capacity configuration", brokerCapacityMap)
		}
		if !ok {
			continue
		}

		_, ok, err = unstructured.NestedMap(brokerCapacityMap, BrokerCapacity, BrokerCapacityDisk)
		// when the format is not a map[string]interface then it has been considered as not JBOD
		if err != nil {
			// brokerID -1 means all brokers get this capacity config as default
			if brokerId == "-1" {
				for brokerID := range brokerIsJBOD {
					brokerIsJBOD[brokerID] = false
				}
			}
			if _, ok := brokerIsJBOD[brokerId]; ok {
				brokerIsJBOD[brokerId] = false
			}
			continue
		}

		// this covers the case when there was a -1 default capacity config but there is an override for a specific broker
		if _, has := brokerIsJBOD[brokerId]; has && ok {
			brokerIsJBOD[brokerId] = true
		}
	}
	//
	for brokerID, isJBOD := range brokerIsJBOD {
		if isJBOD {
			brokersJBOD = append(brokersJBOD, brokerID)
		} else {
			brokersNotJBOD = append(brokersNotJBOD, brokerID)
		}
	}
	return brokersJBOD, brokersNotJBOD, nil
}

// UpdateStatus updates the Status of the provided banzaiv1beta1.KafkaCluster instance with the status of the tasks
// from a CruiseControlTasksAndStates and sends the updates to the Kubernetes API if any changes in the Status field is
// detected. Otherwise, this step is skipped.
func (r *CruiseControlTaskReconciler) UpdateStatus(ctx context.Context, instance *banzaiv1beta1.KafkaCluster,
	taskAndStates *CruiseControlTasksAndStates) error {
	log := logr.FromContextOrDiscard(ctx)

	currentStatus := instance.Status.DeepCopy()
	taskAndStates.SyncState(instance)
	if reflect.DeepEqual(*currentStatus, instance.Status) {
		log.Info("there are no updates to apply to Kafka Cluster Status")
		return nil
	}

	conflictRetryFunction := func() error {
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
	}
	if err := util.RetryOnConflict(util.DefaultBackOffForConflict, conflictRetryFunction); err != nil {
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
			return obj.CurrentTaskState() != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*banzaiv1alpha1.CruiseControlOperation)
			newObj := e.ObjectNew.(*banzaiv1alpha1.CruiseControlOperation)
			if !reflect.DeepEqual(oldObj.CurrentTask(), newObj.CurrentTask()) ||
				oldObj.IsPaused() != newObj.IsPaused() ||
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
		Named("CruiseControlTask")
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
			switch {
			case volumeState.CruiseControlVolumeState.IsDiskRebalance():
				t := &CruiseControlTask{
					BrokerID:                        brokerId,
					Volume:                          mountPath,
					VolumeState:                     volumeState.CruiseControlVolumeState,
					Operation:                       banzaiv1alpha1.OperationRebalance,
					CruiseControlOperationReference: volumeState.CruiseControlOperationReference,
				}
				tasksAndStates.Add(t)

			case volumeState.CruiseControlVolumeState.IsDiskRemoval():
				t := &CruiseControlTask{
					BrokerID:                        brokerId,
					Volume:                          mountPath,
					VolumeState:                     volumeState.CruiseControlVolumeState,
					Operation:                       banzaiv1alpha1.OperationRemoveDisks,
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
func updateActiveTasks(tasksAndStates *CruiseControlTasksAndStates, ccOperations []*banzaiv1alpha1.CruiseControlOperation) {
	ccOperationMap := make(map[string]*banzaiv1alpha1.CruiseControlOperation)
	for i := range ccOperations {
		ccOperationMap[ccOperations[i].Name] = ccOperations[i]
	}

	for _, task := range tasksAndStates.tasks {
		if task == nil || task.CruiseControlOperationReference == nil {
			continue
		}

		task.FromResult(ccOperationMap[task.CruiseControlOperationReference.Name])
	}
}
