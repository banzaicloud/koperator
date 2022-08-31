// Copyright © 2022 Banzai Cloud
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
	banzaiv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaiv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
	"github.com/banzaicloud/koperator/pkg/util"
)

const (
	defaultFailedTasksHistoryMaxLength = 50
	cruiseControlOperationFinalizer    = "finalizer.cruisecontroloperations.kafka.banzaicloud.io"
	ccOperationForFinalize             = "ccOperationStopExecution"
	ccOperationFirstExecution          = "ccOperationFirstExecution"
	ccOperationRetryExecution          = "ccOperationRetryExecution"
	ccOperationInProgress              = "ccOperationInProgress"
)

var (
	defaultRequeueIntervall                                                   = 10
	executionPriorityMap    map[banzaiv1alpha1.CruiseControlTaskOperation]int = map[banzaiv1alpha1.CruiseControlTaskOperation]int{
		banzaiv1alpha1.OperationAddBroker:    0,
		banzaiv1alpha1.OperationRemoveBroker: 1,
		banzaiv1alpha1.OperationRebalance:    2,
	}
)

// CruiseControlOperationReconciler reconciles CruiseControlOperation custom resources
type CruiseControlOperationReconciler struct {
	client.Client
	DirectClient client.Reader
	Scheme       *runtime.Scheme
	Scaler       scale.CruiseControlScaler
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperations,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperations/finalizers,verbs=update

func (r *CruiseControlOperationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("reconciling CruiseControlOperation custom resources")

	ccOperationListClusterWide := banzaiv1alpha1.CruiseControlOperationList{}
	err := r.DirectClient.List(ctx, &ccOperationListClusterWide, client.ListOption(client.InNamespace(request.Namespace)))
	if err != nil {
		return requeueWithError(log, err.Error(), err)
	}

	// Selecting the reconciling CruiseControlOperation
	var currentCCoperation *banzaiv1alpha1.CruiseControlOperation
	for i := range ccOperationListClusterWide.Items {
		ccOperation := &ccOperationListClusterWide.Items[i]
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
		log.Info("Koperator does not support this operation", "operation", currentCCoperation.GetCurrentTaskOp())
		return reconciled()
	}

	kafkaClusterRef, err := getKafkaClusterReference(currentCCoperation)
	if err != nil {
		return requeueWithError(log, "couldn't get kafka cluster reference", err)
	}

	kafkaCluster := &banzaiv1beta1.KafkaCluster{}
	err = r.Get(ctx, kafkaClusterRef, kafkaCluster)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if !currentCCoperation.ObjectMeta.DeletionTimestamp.IsZero() {
				log.Info("Cluster is gone already, remove finalizer")
				controllerutil.RemoveFinalizer(currentCCoperation, cruiseControlOperationFinalizer)
				if err := r.Update(ctx, currentCCoperation); err != nil {
					return requeueWithError(log, "failed to remove finalizer from CruiseControlOperation", err)
				}
			}
			return reconciled()
		}
		return requeueWithError(log, "failed to lookup referenced cluster", err)
	}

	//Adding finalizer
	if err := r.addFinalizer(ctx, currentCCoperation); err != nil {
		return requeueWithError(log, "finalizer could not be added", err)
	}

	// We only get scaler when we have not had mocked one for test
	if _, ok := r.Scaler.(*scale.MockCruiseControlScaler); !ok {
		r.Scaler, err = scale.NewCruiseControlScaler(ctx, scale.CruiseControlURLFromKafkaCluster(kafkaCluster))
		if err != nil {
			return requeueWithError(log, "failed to create Cruise Control Scaler instance", err)
		}
	} else {
		// In tests we requeue faster
		defaultRequeueIntervall = 1
	}
	// Checking Cruise Control availability
	if !r.Scaler.IsUp() {
		log.Info("requeue event as Cruise Control is not available (yet)")
		return requeueAfter(defaultRequeueIntervall)
	}

	// Filtering out CruiseControlOperation by kafka cluster ref
	var ccOperationsKafkaClusterFiltered []*banzaiv1alpha1.CruiseControlOperation
	for i := range ccOperationListClusterWide.Items {
		operation := &ccOperationListClusterWide.Items[i]
		ret, err := getKafkaClusterReference(operation)
		if err != nil {
			log.Info(err.Error())
		}
		if ret.Name == kafkaClusterRef.Name && ret.Namespace == kafkaClusterRef.Namespace && operation.IsCurrentTaskOperationValid() {
			ccOperationsKafkaClusterFiltered = append(ccOperationsKafkaClusterFiltered, operation)
		}
	}

	// Update currentTask states from Cruise Control
	err = r.updateCurrentTasks(ctx, ccOperationsKafkaClusterFiltered)
	if err != nil {
		log.Error(err, "requeue event as updating state of currentTask(s) failed")
		return requeueAfter(defaultRequeueIntervall)
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

	// Sorting operations into categories which are sorted by priority
	ccOperationQueueMap := sortOperations(ccOperationsKafkaClusterFiltered)
	// When there is no more job present in the cluster we reconciled.
	if len(ccOperationQueueMap[ccOperationForFinalize]) == 0 && len(ccOperationQueueMap[ccOperationFirstExecution]) == 0 &&
		len(ccOperationQueueMap[ccOperationRetryExecution]) == 0 && len(ccOperationQueueMap[ccOperationInProgress]) == 0 {
		return reconciled()
	}

	ccOperationExecution := selectOperationForExecution(ccOperationQueueMap)
	// There is nothing to be executed for now, requeue
	if ccOperationExecution == nil {
		return requeueAfter(defaultRequeueIntervall)
	}

	// Check if CruiseControl is ready as we cannot perform any operation until it is in ready state unless it is a stop execution operation
	if status := r.Scaler.Status(); status.InExecution() && ccOperationExecution.GetCurrentTaskOp() != banzaiv1alpha1.OperationStopExecution {
		// Requeue becuse we can't do more
		return requeueAfter(defaultRequeueIntervall)
	}

	log.Info("executing Cruise Control task", "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTaskParameters())
	// Executing operation
	cruseControlTaskResult, err := r.executeOperation(ccOperationExecution)

	if err != nil {
		log.Error(err, "Cruise Control task execution got error", "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTaskParameters())
	}

	// Protection to avoid re-execution task when status update is failed
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
		return requeueWithError(log, "could not update the result of the Cruise Control user task execution to the CruiseControlOperation status", errEnd)
	}

	return reconciled()
}
func (r *CruiseControlOperationReconciler) addFinalizer(ctx context.Context, currentCCoperation *banzaiv1alpha1.CruiseControlOperation) error {
	// examine DeletionTimestamp to determine if object is under deletion
	if currentCCoperation.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(currentCCoperation, cruiseControlOperationFinalizer) {
			controllerutil.AddFinalizer(currentCCoperation, cruiseControlOperationFinalizer)
			if err := r.Update(ctx, currentCCoperation); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *CruiseControlOperationReconciler) executeOperation(ccOperationExecution *banzaiv1alpha1.CruiseControlOperation) (*scale.Result, error) {
	var cruseControlTaskResult *scale.Result
	var err error
	switch ccOperationExecution.GetCurrentTaskOp() {
	case banzaiv1alpha1.OperationAddBroker:
		cruseControlTaskResult, err = r.Scaler.AddBrokersWithParams(ccOperationExecution.GetCurrentTaskParameters())
	case banzaiv1alpha1.OperationRemoveBroker:
		cruseControlTaskResult, err = r.Scaler.RemoveBrokersWithParams(ccOperationExecution.GetCurrentTaskParameters())
	case banzaiv1alpha1.OperationRebalance:
		cruseControlTaskResult, err = r.Scaler.RebalanceWithParams(ccOperationExecution.GetCurrentTaskParameters())
	case banzaiv1alpha1.OperationStopExecution:
		cruseControlTaskResult, err = r.Scaler.StopExecution()
	default:
		err = errors.NewWithDetails("Cruise Control operation not supported", "name", ccOperationExecution.GetName(), "namespace", ccOperationExecution.GetNamespace(), "operation", ccOperationExecution.GetCurrentTaskOp(), "parameters", ccOperationExecution.GetCurrentTaskParameters())
	}
	return cruseControlTaskResult, err
}

func sortOperations(ccOperations []*banzaiv1alpha1.CruiseControlOperation) map[string][]*banzaiv1alpha1.CruiseControlOperation {
	ccOperationQueueMap := make(map[string][]*banzaiv1alpha1.CruiseControlOperation)
	for _, ccOperation := range ccOperations {
		switch {
		case isWaitingforFinalize(ccOperation):
			ccOperationQueueMap[ccOperationForFinalize] = append(ccOperationQueueMap[ccOperationForFinalize], ccOperation)
		case ccOperation.IsWaitingForFirstExecution():
			ccOperationQueueMap[ccOperationFirstExecution] = append(ccOperationQueueMap[ccOperationFirstExecution], ccOperation)
		case ccOperation.IsWaitingForRetryExecution():
			ccOperationQueueMap[ccOperationRetryExecution] = append(ccOperationQueueMap[ccOperationRetryExecution], ccOperation)
		case ccOperation.IsInProgress():
			ccOperationQueueMap[ccOperationInProgress] = append(ccOperationQueueMap[ccOperationInProgress], ccOperation)
		}
	}

	// Sorting by operation type and by the k8s object creation time
	for key := range ccOperationQueueMap {
		sort.SliceStable(ccOperationQueueMap[key], func(i, j int) bool {
			ccOperationQueue := ccOperationQueueMap[key]
			if executionPriorityMap[ccOperationQueue[i].GetCurrentTaskOp()] < executionPriorityMap[ccOperationQueue[j].GetCurrentTaskOp()] {
				return true
			} else if executionPriorityMap[ccOperationQueue[i].GetCurrentTaskOp()] == executionPriorityMap[ccOperationQueue[j].GetCurrentTaskOp()] {
				return ccOperationQueue[i].CreationTimestamp.Unix() < ccOperationQueue[j].CreationTimestamp.Unix()
			}
			return true
		})
	}
	return ccOperationQueueMap
}

func selectOperationForExecution(ccOperationQueueMap map[string][]*banzaiv1alpha1.CruiseControlOperation) *banzaiv1alpha1.CruiseControlOperation {
	// SELECTING OPERATION FOR EXECUTION
	var ccOperationExecution *banzaiv1alpha1.CruiseControlOperation
	// First prio: execute the finalize task
	switch {
	case len(ccOperationQueueMap[ccOperationForFinalize]) > 0:
		ccOperationExecution = ccOperationQueueMap[ccOperationForFinalize][0]
		ccOperationExecution.GetCurrentTask().Operation = banzaiv1alpha1.OperationStopExecution
	// Second prio: execute add_broker operation
	case len(ccOperationQueueMap[ccOperationFirstExecution]) > 0 && ccOperationQueueMap[ccOperationFirstExecution][0].GetCurrentTaskOp() == banzaiv1alpha1.OperationAddBroker:
		ccOperationExecution = ccOperationQueueMap[ccOperationFirstExecution][0]
	// Third prio: execute failed task
	case len(ccOperationQueueMap[ccOperationRetryExecution]) > 0:
		// When default backoff duration elapsed we retry
		if ccOperationQueueMap[ccOperationRetryExecution][0].IsReadyForRetryExecution() {
			ccOperationExecution = ccOperationQueueMap[ccOperationRetryExecution][0]
		}
	// Forth prio: execute the first element in the FirstExecutionQueue which is ordered by operation type and k8s creation timestamp
	case len(ccOperationQueueMap[ccOperationFirstExecution]) > 0:
		ccOperationExecution = ccOperationQueueMap[ccOperationFirstExecution][0]
	}

	return ccOperationExecution
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
				if !reflect.DeepEqual(oldObj.GetCurrentTask(), newObj.GetCurrentTask()) ||
					oldObj.GetDeletionTimestamp() != newObj.GetDeletionTimestamp() ||
					oldObj.GetGeneration() != newObj.GetGeneration() {
					return true
				}
				return false
			},
		})

	return builder
}

func getKafkaClusterReference(operation *banzaiv1alpha1.CruiseControlOperation) (client.ObjectKey, error) {
	if operation.GetClusterRef() == "" {
		return client.ObjectKey{}, errors.NewWithDetails("could not find kafka cluster reference label for CruiseControlOperation", "missing label", banzaiv1beta1.KafkaCRLabelKey, "name", operation.GetName(), "namespace", operation.GetNamespace())
	}
	return client.ObjectKey{
		Name:      operation.GetClusterRef(),
		Namespace: operation.GetNamespace(),
	}, nil
}

func updateResult(res *scale.Result, operation *banzaiv1alpha1.CruiseControlOperation, isAfterExecution bool) error {
	if res == nil || operation.GetCurrentTask() == nil {
		return nil
	}
	task := operation.GetCurrentTask()

	if (task.State == banzaiv1beta1.CruiseControlTaskCompleted || task.State == banzaiv1beta1.CruiseControlTaskCompletedWithError) && task.Finished == nil {
		task.Finished = &v1.Time{Time: time.Now()}
	}

	// Add the failed task into the status.failedTasks slice only when the update is happened after executing task
	if isAfterExecution && task.Finished != nil && task.State == banzaiv1beta1.CruiseControlTaskCompletedWithError {
		if len(operation.Status.FailedTasks) >= defaultFailedTasksHistoryMaxLength {
			operation.Status.FailedTasks = append(operation.Status.FailedTasks[1:], *task)
		} else {
			operation.Status.FailedTasks = append(operation.Status.FailedTasks, *task)
		}
		operation.Status.NumberOfRetries += 1
		operation.Status.ErrorPolicy = banzaiv1alpha1.ErrorPolicyRetry
		task.Finished = nil
		task.State = ""
		task.Started = nil
	}

	task.ID = res.TaskID
	if task.Started == nil {
		//Sat, 27 Aug 2022 12:22:21 GMT
		startTime, err := time.Parse(time.RFC1123, res.StartedAt)
		if err != nil {
			return errors.WrapIff(err, "could not parse user task start time from Cruise Control API")
		}
		task.Started = &v1.Time{Time: startTime}
	}
	// We only have summary after execution
	if isAfterExecution {
		task.Summary = parseSummary(res.Result)
		task.ErrorMessage = res.Err
	}
	// This is necessary because of a bug in CC (when a user task resulted in error later when get usertasks status from CC the corresponding task state is Active and not CompletedWithError...)
	//if task.State != banzaiv1beta1.CruiseControlTaskCompletedWithError && task.State != banzaiv1beta1.CruiseControlTaskCompleted {
	task.State = res.State
	//}
	return nil
}

// updateActiveTasks updates the state of the CruiseControlOperation from the CruiseControlTasksAndStates instance by getting their
// status from Cruise Control.
func (r *CruiseControlOperationReconciler) updateCurrentTasks(ctx context.Context, ccOperations []*banzaiv1alpha1.CruiseControlOperation) error {
	log := logr.FromContextOrDiscard(ctx)
	tasks, err := r.Scaler.GetUserTasks()
	if err != nil {
		return errors.WrapIff(err, "could not get user tasks from Cruise Control API")
	}

	var ccOperationsCopy []*banzaiv1alpha1.CruiseControlOperation
	for _, ccOperation := range ccOperations {
		copy := ccOperation.DeepCopy()
		ccOperationsCopy = append(ccOperationsCopy, copy)
	}

	taskResultsByID := make(map[string]*scale.Result, len(tasks))
	for _, task := range tasks {
		taskResultsByID[task.TaskID] = task
	}
	for i := range ccOperations {
		ccOperation := ccOperations[i]
		log.Info("taskres current", "res", taskResultsByID[ccOperation.GetCurrentTaskID()])
		if err := updateResult(taskResultsByID[ccOperation.GetCurrentTaskID()], ccOperation, false); err != nil {
			return errors.WrapWithDetails(err, "could not set Cruise Control user task result to CruiseControlOperation CurrentTask", "name", ccOperations[i].GetName(), "namespace", ccOperations[i].GetNamespace())
		}
	}

	for i := range ccOperations {
		if !reflect.DeepEqual(ccOperations[i].Status, ccOperationsCopy[i].Status) {
			if err := r.Status().Update(ctx, ccOperations[i]); err != nil {
				return errors.WrapIfWithDetails(err, "could not update CruiseControlOperation status", "name", ccOperations[i].GetName(), "namespace", ccOperations[i].GetNamespace())
			}
		}
	}
	return nil
}

func isWaitingforFinalize(ccOperation *banzaiv1alpha1.CruiseControlOperation) bool {
	if ccOperation.IsCurrentTaskRunning() && !ccOperation.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(ccOperation, cruiseControlOperationFinalizer) {
		return true
	}
	return false
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
