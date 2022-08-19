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
	"sort"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/banzaicloud/go-cruise-control/pkg/types"
	"github.com/banzaicloud/koperator/api/v1alpha1"
	banzaiv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	banzaiv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
	"github.com/banzaicloud/koperator/pkg/util"
)

const (
	defaultFailedTasksHistoryMaxLength = 50
	cruiseControlOperationFinalizer    = "finalizer.cruisecontroloperations.kafka.banzaicloud.io"
	executeNewWhenHaveFailedTask       = false
)

var (
	defaultTaskStatusUpdateIntervall                                                   = 10
	executionOrderMap                map[banzaiv1alpha1.CruiseControlTaskOperation]int = map[banzaiv1alpha1.CruiseControlTaskOperation]int{
		banzaiv1alpha1.OperationAddBroker:    0,
		banzaiv1alpha1.OperationRemoveBroker: 1,
		banzaiv1alpha1.OperationRebalance:    2,
	}
)

// CruiseControlOperationReconciler reconciles CruiseControlOperation custom resoruces
type CruiseControlOperationReconciler struct {
	client.Client
	DirectClient client.Reader
	Scheme       *runtime.Scheme
	Scaler       scale.CruiseControlScaler
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperation,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperation/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperation/finalizers,verbs=update

func (r *CruiseControlOperationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("reconciling CruiseControlOperation custom resources")

	ccOperationListClusterWide := banzaiv1alpha1.CruiseControlOperationList{}
	//ccOperation := banzaiv1alpha1.CruiseControlOperation{}
	//r.Get(ctx, client.ObjectKey{Name: request.Name, Namespace: request.Namespace}, &ccOperation)
	err := r.DirectClient.List(ctx, &ccOperationListClusterWide, client.ListOption(client.InNamespace(request.Namespace)))
	if err != nil {
		return reconciledWithError(log, err.Error(), err)
	}

	var currentCCoperation *banzaiv1alpha1.CruiseControlOperation
	for i := range ccOperationListClusterWide.Items {
		ccOperation := &ccOperationListClusterWide.Items[i]
		// if ccOperation.Status.CurrentTask.State != kafkav1beta1.CruiseControlTaskCompleted {

		// }
		if ccOperation.GetName() == request.Name && ccOperation.GetNamespace() == request.Namespace {
			currentCCoperation = ccOperation
		}
	}

	// Object is deleted...
	if currentCCoperation == nil {
		return reconciled()
	}
	// CruiseControlOperation has invalid operation type, reconciled
	if !currentCCoperation.IsCurrentTaskOperationValid() {
		log.Info("Operation is invalid")
		return reconciled()
	}

	if currentCCoperation.GetLabels()[banzaiv1beta1.KafkaCRLabelKey] == "" {
		requeueWithError(log, "couldn't get kafka cluster reference", fmt.Errorf("missing cluster reference label: %s", banzaiv1beta1.KafkaCRLabelKey))
	}
	kafkaClusterRef := client.ObjectKey{Name: currentCCoperation.GetLabels()[banzaiv1beta1.KafkaCRLabelKey], Namespace: currentCCoperation.GetNamespace()}

	kafkaCluster := &banzaiv1beta1.KafkaCluster{}
	err = r.Get(ctx, kafkaClusterRef, kafkaCluster)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if !currentCCoperation.ObjectMeta.DeletionTimestamp.IsZero() {
				log.Info("Cluster is gone already, remove finalizer")
				controllerutil.RemoveFinalizer(currentCCoperation, cruiseControlOperationFinalizer)
				if err := r.Update(ctx, currentCCoperation); err != nil {
					return requeueWithError(log, "failed to remove finalizer from kafkauser", err)
				}
				return reconciled()
			}
		}
		return requeueWithError(log, "failed to lookup referenced cluster", err)
	}

	//Adding finalizer
	// examine DeletionTimestamp to determine if object is under deletion
	if currentCCoperation.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(currentCCoperation, cruiseControlOperationFinalizer) {
			controllerutil.AddFinalizer(currentCCoperation, cruiseControlOperationFinalizer)
			if err := r.Update(ctx, currentCCoperation); err != nil {
				return requeueWithError(log, err.Error(), err)
			}
		}
	}

	// We only get scaler for cruise control operation when we dont have mocked one for test
	if _, ok := r.Scaler.(*scale.MockCruiseControlScaler); !ok {
		r.Scaler, err = scale.NewCruiseControlScaler(ctx, scale.CruiseControlURLFromKafkaCluster(kafkaCluster))
		if err != nil {
			return requeueWithError(log, "failed to create Cruise Control Scaler instance", err)
		}
	} else {
		// In tests we requeue faster
		defaultTaskStatusUpdateIntervall = 1
	}

	if !r.Scaler.IsUp() {
		log.Info("requeue event as Cruise Control is not available (yet)")
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	// Filtering out CruiseControlOperation by Cluster Ref
	var ccOperationsKafkaClusterFiltered []*banzaiv1alpha1.CruiseControlOperation
	for i := range ccOperationListClusterWide.Items {
		isBelongs := isCCoperationBelongsToKafkaCluster(ccOperationListClusterWide.Items[i], kafkaClusterRef)
		if isBelongs != nil && *isBelongs {
			ccOperationsKafkaClusterFiltered = append(ccOperationsKafkaClusterFiltered, &ccOperationListClusterWide.Items[i])
		}
	}

	var ccOperationsKafkaClusterFilteredCopy []*banzaiv1alpha1.CruiseControlOperation
	for _, ccOperation := range ccOperationsKafkaClusterFiltered {
		copy := ccOperation.DeepCopy()
		ccOperationsKafkaClusterFilteredCopy = append(ccOperationsKafkaClusterFilteredCopy, copy)
	}
	// Update task states with information from Cruise Control
	err = updateCurrentTasks(r.Scaler, ccOperationsKafkaClusterFiltered)
	if err != nil {
		log.Error(err, "requeue event as updating state of currentTask(s) failed")
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	for i := range ccOperationsKafkaClusterFiltered {
		if !reflect.DeepEqual(ccOperationsKafkaClusterFiltered[i].Status, ccOperationsKafkaClusterFilteredCopy[i].Status) {
			if err := r.Status().Update(ctx, currentCCoperation); err != nil {
				return requeueWithError(log, err.Error(), err)
			}
		}
	}

	//When the task is not in execution we can remove the finalizer
	if controllerutil.ContainsFinalizer(currentCCoperation, cruiseControlOperationFinalizer) && !currentCCoperation.ObjectMeta.DeletionTimestamp.IsZero() &&
		!currentCCoperation.IsCurrentTaskRunning() {
		controllerutil.RemoveFinalizer(currentCCoperation, cruiseControlOperationFinalizer)
		if err := r.Update(ctx, currentCCoperation); err != nil {
			return requeueWithError(log, err.Error(), err)
		}
		return reconciled()
	}

	// Check if CruiseControl is ready as we cannot perform any operation until it is in ready state
	if status := r.Scaler.Status(); status.InExecution() {
		log.Info("updating status of Kafka Cluster and requeue event as Cruise Control is in execution")
		if err := r.k8sUpdateforCCoperationStatuses(ctx, ccOperationsKafkaClusterFilteredCopy, ccOperationsKafkaClusterFiltered); err != nil {
			return requeueWithError(log, err.Error(), err)
		}
		// Requeue becuse we can't do more
		return requeueAfter(DefaultRequeueAfterTimeInSec)
	}

	// Searching for task which is running and it has finalizer and
	// collecting tasks which are waiting for execution
	ccOperationQueueMap := make(map[string][]*banzaiv1alpha1.CruiseControlOperation)
	for _, ccOperation := range ccOperationsKafkaClusterFiltered {
		// Collecting CC operations to finalize
		if isWaitingforFinalize(ccOperation) {
			ccOperationQueueMap["ccOperationFinalizeExecutionQueue"] = append(ccOperationQueueMap["ccOperationFinalizeExecutionQueue"], ccOperation)
		} else if ccOperation.IsWaitingForFirstExecution() {
			ccOperationQueueMap["ccOperationFirstExecutionQueue"] = append(ccOperationQueueMap["ccOperationFirstExecutionQueue"], ccOperation)
		} else if ccOperation.IsWaitingForRetryExecution() {
			ccOperationQueueMap["ccOperationRetryExecutionQueue"] = append(ccOperationQueueMap["ccOperationRetryExecutionQueue"], ccOperation)
		}
	}

	// When there is no more job present in the cluster we reconciled.
	if len(ccOperationQueueMap["ccOperationFinalizeExecutionQueue"]) == 0 && len(ccOperationQueueMap["ccOperationFirstExecutionQueue"]) == 0 && len(ccOperationQueueMap["ccOperationRetryExecutionQueue"]) == 0 {
		return reconciled()
	}

	// Sorting by operation type and by the k8s object creation time
	for key := range ccOperationQueueMap {
		sort.SliceStable(ccOperationQueueMap[key], func(i, j int) bool {
			ccOperationQueue := ccOperationQueueMap[key]
			if executionOrderMap[ccOperationQueue[i].GetCurrentTaskOp()] < executionOrderMap[ccOperationQueue[j].GetCurrentTaskOp()] {
				return true
			} else if executionOrderMap[ccOperationQueue[i].GetCurrentTaskOp()] == executionOrderMap[ccOperationQueue[j].GetCurrentTaskOp()] {
				return ccOperationQueue[i].CreationTimestamp.Unix() < ccOperationQueue[j].CreationTimestamp.Unix()

			}
			return true
		})
	}
	// SELECTING OPERATION FOR EXECUTION
	var ccOperationExecution *banzaiv1alpha1.CruiseControlOperation
	// First prio: execute the finalize task
	if len(ccOperationQueueMap["ccOperationFinalizeExecutionQueue"]) > 0 {
		ccOperationExecution = ccOperationQueueMap["ccOperationFinalizeExecutionQueue"][0]
		ccOperationExecution.Status.CurrentTask.Operation = v1alpha1.OperationStopExecution
		// Second prio: execute add_broker operation
	} else if len(ccOperationQueueMap["ccOperationFirstExecutionQueue"]) > 0 && ccOperationQueueMap["ccOperationFirstExecutionQueue"][0].GetCurrentTaskOp() == v1alpha1.OperationAddBroker {
		ccOperationExecution = ccOperationQueueMap["ccOperationFirstExecutionQueue"][0]
		// Third prio: execute failed task
	} else if len(ccOperationQueueMap["ccOperationRetryExecutionQueue"]) > 0 {
		// When default backoff duration elapsed we retry
		if ccOperationQueueMap["ccOperationRetryExecutionQueue"][0].IsReadyForRetryExecution() {
			ccOperationExecution = ccOperationQueueMap["ccOperationRetryExecutionQueue"][0]
		}
		// Forth prio: execute the first in the unexecuted list which ordered by operation type and k8s creation timestamp
	} else if len(ccOperationQueueMap["ccOperationFirstExecutionQueue"]) > 0 {
		ccOperationExecution = ccOperationQueueMap["ccOperationFirstExecutionQueue"][0]
	}

	// There is nothing to be executed for now, requeue
	if ccOperationExecution == nil {
		return requeueAfter(defaultTaskStatusUpdateIntervall)
	}

	var cruseControlTaskResult *scale.Result
	switch ccOperationExecution.GetCurrentTaskOp() {
	case banzaiv1alpha1.OperationAddBroker:
		log.Info("executing Cruise Control task", "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTask().Parameters)
		cruseControlTaskResult, err = r.Scaler.AddBrokersWithParams(ccOperationExecution.GetCurrentTask().Parameters)
	case banzaiv1alpha1.OperationRemoveBroker:
		log.Info("executing Cruise Control task", "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTask().Parameters)
		cruseControlTaskResult, err = r.Scaler.RemoveBrokersWithParams(ccOperationExecution.GetCurrentTask().Parameters)
	case banzaiv1alpha1.OperationRebalance:
		//https://github.com/banzaicloud/koperator/blob/9b28b80945c0fc8f59f133a7eb4a3923640a59c1/controllers/cruisecontroltask_controller.go#L137
		log.Info("executing Cruise Control task", "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTask().Parameters)
		cruseControlTaskResult, err = r.Scaler.RebalanceWithParams(ccOperationExecution.GetCurrentTask().Parameters)
	case v1alpha1.OperationStopExecution:
		cruseControlTaskResult, err = r.Scaler.StopExecution()
	default:
		log.Error(err, "Cruise Control operation not supported", "name", ccOperationExecution.GetName(), "namespace", ccOperationExecution.GetNamespace(), "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTask().Parameters)
		return reconciled()
	}

	if err != nil {
		log.Error(err, "Cruise Control task execution failed", "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTask().Parameters)
	}

	// Protection to avoid re-execution task when status update failed
	if errEnd := util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
		if err = updateResult(cruseControlTaskResult, ccOperationExecution, true); err != nil {
			return err
		}
		err = r.Status().Update(ctx, ccOperationExecution)
		if apiErrors.IsConflict(err) {
			err = r.Get(ctx, client.ObjectKey{Name: ccOperationExecution.GetName(), Namespace: ccOperationExecution.GetNamespace()}, ccOperationExecution)
		}
		return err
	}); errEnd != nil {
		return requeueWithError(log, errEnd.Error(), errEnd)
	}

	// Requeue after execution
	return requeueAfter(defaultTaskStatusUpdateIntervall)
}

func isWaitingforFinalize(ccOperation *banzaiv1alpha1.CruiseControlOperation) bool {
	if !ccOperation.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(ccOperation, cruiseControlOperationFinalizer) {
		return true
	}
	return false
}

func (r *CruiseControlOperationReconciler) k8sUpdateforCCoperationStatuses(ctx context.Context, operationsOld, operationsNew []*banzaiv1alpha1.CruiseControlOperation) error {
	log := logr.FromContextOrDiscard(ctx)
	log.Info("Updating CruiseControlOperation Statuses...")
	for i := range operationsOld {
		operationOld := operationsOld[i]
		operationNew := operationsNew[i]
		if !reflect.DeepEqual(operationOld.Status, operationNew.Status) {
			if err := r.Update(ctx, operationNew); err != nil {
				return errors.WrapIfWithDetails(err, "failed to update CruiseControlOperation status", "name", operationNew.GetName(), "namespace", operationNew.GetNamespace())
			}
		}
	}
	return nil
}

// SetupCruiseControlWithManager registers cruise control controller to the manager
func SetupCruiseControlOperationWithManager(mgr ctrl.Manager) *ctrl.Builder {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&banzaiv1alpha1.CruiseControlOperation{}).
		WithEventFilter(SkipClusterRegistryOwnedResourcePredicate{}).
		Named("CruiseControlOperation")

	builder.WithEventFilter(
		predicate.Funcs{
			// We dont reconcile when there is no operation definied
			CreateFunc: func(e event.CreateEvent) bool {
				obj := e.Object.(*banzaiv1alpha1.CruiseControlOperation)
				return obj.GetCurrentTaskOp() != ""
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObj := e.ObjectOld.(*banzaiv1alpha1.CruiseControlOperation)
				newObj := e.ObjectNew.(*banzaiv1alpha1.CruiseControlOperation)
				// We only reconcile when spec or operation or deletiontimestamp is changed
				if oldObj.GetCurrentTaskOp() != newObj.GetCurrentTaskOp() ||
					oldObj.GetDeletionTimestamp() != newObj.GetDeletionTimestamp() ||
					oldObj.GetGeneration() != newObj.GetGeneration() {
					return true
				}
				return false
			},
		})

	return builder
}

func isCCoperationBelongsToKafkaCluster(operation banzaiv1alpha1.CruiseControlOperation, clusterRef client.ObjectKey) *bool {
	if operation.GetLabels()[banzaiv1beta1.KafkaCRLabelKey] == "" {
		return nil
	} else if operation.GetLabels()[banzaiv1beta1.KafkaCRLabelKey] == clusterRef.Name && operation.GetNamespace() == clusterRef.Namespace {
		return util.BoolPointer(true)
	}
	return util.BoolPointer(false)
}

func updateResult(res *scale.Result, operation *banzaiv1alpha1.CruiseControlOperation, isAfterExecution bool) error {
	if res == nil || operation.GetCurrentTask() == nil {
		return nil
	}
	task := operation.GetCurrentTask()
	// When we got instant error after execution then we set the state
	if res.State == banzaiv1beta1.CruiseControlTaskCompletedWithError {
		task.State = res.State
	}
	// When we got instant result after execution then we set the state
	if (task.State == v1beta1.CruiseControlTaskCompleted || task.State == v1beta1.CruiseControlTaskCompletedWithError) && task.Finished == nil {
		task.Finished = &v1.Time{Time: time.Now()}
	}

	// Add the failed task into the status failedTasks slice only when the update is after the task execution
	if isAfterExecution && task.Finished != nil && task.State == v1beta1.CruiseControlTaskCompletedWithError {
		operation.Status.FailedTasks = append(operation.Status.FailedTasks, *task)
		operation.Status.NumberOfRetries += 1
		operation.Status.ErrorPolicy = v1alpha1.ErrorPolicyRetry
		task.Finished = nil
	}

	task.ID = res.TaskID
	if task.Started == nil {
		startTime, err := time.Parse(time.RFC3339, res.StartedAt)
		if err != nil {
			return errors.WrapIff(err, "could not parse user task start time from Cruise Control API")
		}
		task.Started = &v1.Time{Time: startTime}
	}
	task.Summary = parseSummary(res.Result)
	task.State = res.State
	task.ErrorMessage = res.Err
	return nil
}

// updateActiveTasks updates the state of the tasks from the CruiseControlTasksAndStates instance by getting their
// status from CruiseControl using the provided scale.CruiseControlScaler.
func updateCurrentTasks(scaler scale.CruiseControlScaler, ccOperations []*banzaiv1alpha1.CruiseControlOperation) error {
	tasks, err := scaler.GetUserTasks()
	if err != nil {
		return errors.WrapIff(err, "could not get user tasks from Cruise Control API")
	}

	taskResultsByID := make(map[string]*scale.Result, len(tasks))
	for _, task := range tasks {
		taskResultsByID[task.TaskID] = task
	}

	for i := range ccOperations {
		ccOperation := ccOperations[i]
		if err := updateResult(taskResultsByID[ccOperation.GetCurrentTaskID()], ccOperation, false); err != nil {
			return errors.WrapWithDetails(err, "could not set Cruise Control user task result to CruiseControlOperation CurrentTask", "ID", ccOperation.GetCurrentTaskID())
		}
	}

	return nil
}

func parseSummary(res *types.OptimizationResult) map[string]string {
	if res == nil {
		return nil
	}
	return map[string]string{
		"Data to move":                             fmt.Sprintf("%d", res.Summary.DataToMoveMB),
		"Number of replica movements":              fmt.Sprintf("%d", res.Summary.NumReplicaMovements),
		"Intra broker data to move":                fmt.Sprintf("%d", res.Summary.IntraBrokerDataToMoveMB),
		"Number of intra broker replica movements": fmt.Sprintf("%d", res.Summary.NumIntraBrokerReplicaMovements),
		"Number of leader movements":               fmt.Sprintf("%d", res.Summary.NumLeaderMovements),
		"Recent windows":                           fmt.Sprintf("%d", res.Summary.RecentWindows),
		"Provision recommendation":                 res.Summary.ProvisionRecommendation,
	}
}
