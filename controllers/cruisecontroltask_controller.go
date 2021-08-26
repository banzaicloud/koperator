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
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	ccutils "github.com/banzaicloud/kafka-operator/pkg/util/cruisecontrol"

	kafkav1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
)

// CruiseControlTaskReconciler reconciles a kafka cluster object
type CruiseControlTaskReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Log logr.Logger
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch

func (r *CruiseControlTaskReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("clusterName", request.Name, "clusterNamespace", request.Namespace)

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
		return requeueWithError(r.Log, err.Error(), err)
	}

	log.V(1).Info("Reconciling")

	brokersWithRunningCCTask := make(map[string]kafkav1beta1.BrokerState)
	brokerVolumesWithRunningCCTask := make(map[string]map[string]kafkav1beta1.VolumeState)
	for brokerId, brokerStatus := range instance.Status.BrokersState {
		if brokerStatus.GracefulActionState.CruiseControlState.IsRunningState() {
			brokersWithRunningCCTask[brokerId] = brokerStatus
		}

		volumesState := make(map[string]kafkav1beta1.VolumeState)
		for mountPath, volumeState := range brokerStatus.GracefulActionState.VolumeStates {
			if volumeState.CruiseControlVolumeState == kafkav1beta1.GracefulDiskRebalanceRunning {
				volumesState[mountPath] = volumeState
			}
		}
		if len(volumesState) > 0 {
			brokerVolumesWithRunningCCTask[brokerId] = volumesState
		}
	}
	if len(brokersWithRunningCCTask) > 0 {
		err = r.checkCCTaskState(instance, brokersWithRunningCCTask, log)
	}

	if err == nil && len(brokerVolumesWithRunningCCTask) > 0 {
		err = r.checkVolumeCCTaskState(instance, brokerVolumesWithRunningCCTask, log)
	}

	if err != nil {
		switch errors.Cause(err).(type) {
		case errorfactory.CruiseControlNotReady, errorfactory.ResourceNotReady:
			return ctrl.Result{
				RequeueAfter: time.Duration(15) * time.Second,
			}, nil
		case errorfactory.CruiseControlTaskRunning:
			return ctrl.Result{
				RequeueAfter: time.Duration(20) * time.Second,
			}, nil
		case errorfactory.CruiseControlTaskTimeout, errorfactory.CruiseControlTaskFailure:
			return ctrl.Result{
				RequeueAfter: time.Duration(20) * time.Second,
			}, nil
		default:
			return requeueWithError(log, err.Error(), err)
		}
	}

	var brokersWithDownscaleRequired []string
	var brokersWithUpscaleRequired []string
	brokersWithDiskRebalanceRequired := make(map[string][]string)

	for brokerId, brokerStatus := range instance.Status.BrokersState {
		if brokerStatus.GracefulActionState.CruiseControlState == kafkav1beta1.GracefulUpscaleRequired {
			brokersWithUpscaleRequired = append(brokersWithUpscaleRequired, brokerId)
		} else if brokerStatus.GracefulActionState.CruiseControlState == kafkav1beta1.GracefulDownscaleRequired {
			brokersWithDownscaleRequired = append(brokersWithDownscaleRequired, brokerId)
		}

		for mountPath, volumeState := range brokerStatus.GracefulActionState.VolumeStates {
			if volumeState.CruiseControlVolumeState == kafkav1beta1.GracefulDiskRebalanceRequired {
				brokersWithDiskRebalanceRequired[brokerId] = append(brokersWithDiskRebalanceRequired[brokerId], mountPath)
			}
		}
	}

	var taskId, startTime string
	switch {
	case len(brokersWithUpscaleRequired) > 0:
		err = r.handlePodAddCCTask(instance, brokersWithUpscaleRequired, log)
	case len(brokersWithDownscaleRequired) > 0:
		err = r.handlePodDeleteCCTask(instance, brokersWithDownscaleRequired, log)
	case len(brokersWithDiskRebalanceRequired) > 0:
		// create new cc task, set status to running
		cc := scale.NewCruiseControlScaler(instance.Namespace, instance.Spec.GetKubernetesClusterDomain(), instance.Spec.CruiseControlConfig.CruiseControlEndpoint, instance.Name)
		taskId, startTime, err = cc.RebalanceDisks(brokersWithDiskRebalanceRequired)
		if err != nil {
			log.Error(err, "executing disk rebalance cc task failed")
		} else {
			var brokerIds []string
			brokersVolumeStates := make(map[string]map[string]kafkav1beta1.VolumeState, len(brokersWithDiskRebalanceRequired))
			for brokerId, mountPaths := range brokersWithDiskRebalanceRequired {
				brokerVolumeState := make(map[string]kafkav1beta1.VolumeState, len(mountPaths))
				for _, mountPath := range mountPaths {
					brokerVolumeState[mountPath] = kafkav1beta1.VolumeState{
						CruiseControlTaskId:      taskId,
						TaskStarted:              startTime,
						CruiseControlVolumeState: kafkav1beta1.GracefulDiskRebalanceRunning,
					}
				}
				if len(brokerVolumeState) > 0 {
					brokersVolumeStates[brokerId] = brokerVolumeState
					brokerIds = append(brokerIds, brokerId)
				}
			}
			if len(brokersVolumeStates) > 0 {
				err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, instance, brokersVolumeStates, log)
			}
		}
	}

	if err != nil {
		switch errors.Cause(err).(type) {
		case errorfactory.CruiseControlNotReady:
			return ctrl.Result{
				RequeueAfter: time.Duration(15) * time.Second,
			}, nil
		case errorfactory.CruiseControlTaskRunning:
			return ctrl.Result{
				RequeueAfter: time.Duration(20) * time.Second,
			}, nil
		case errorfactory.CruiseControlTaskTimeout, errorfactory.CruiseControlTaskFailure:
			return ctrl.Result{
				RequeueAfter: time.Duration(20) * time.Second,
			}, nil
		default:
			return requeueWithError(log, err.Error(), err)
		}
	}

	return reconciled()
}
func (r *CruiseControlTaskReconciler) handlePodAddCCTask(kafkaCluster *kafkav1beta1.KafkaCluster, brokerIds []string, log logr.Logger) error {
	cc := scale.NewCruiseControlScaler(kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	uTaskId, taskStartTime, scaleErr := cc.UpScaleCluster(brokerIds)
	if scaleErr != nil {
		log.Info("Cannot upscale broker(s)", "brokerId(s)", brokerIds, "error", scaleErr.Error())
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, scaleErr, fmt.Sprintf("broker id(s): %s", brokerIds))
	}
	statusErr := k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
		kafkav1beta1.GracefulActionState{CruiseControlTaskId: uTaskId, CruiseControlState: kafkav1beta1.GracefulUpscaleRunning,
			TaskStarted: taskStartTime}, log)
	if statusErr != nil {
		return errors.WrapIfWithDetails(statusErr, "could not update status for broker", "id(s)", brokerIds)
	}
	return nil
}
func (r *CruiseControlTaskReconciler) handlePodDeleteCCTask(kafkaCluster *kafkav1beta1.KafkaCluster, brokerIds []string, log logr.Logger) error {
	cc := scale.NewCruiseControlScaler(kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	uTaskId, taskStartTime, err := cc.DownsizeCluster(brokerIds)
	if err != nil {
		log.Info("cruise control communication error during downscaling broker(s)", "id(s)", brokerIds)
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, fmt.Sprintf("broker(s) id(s): %s", brokerIds))
	}
	err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
		kafkav1beta1.GracefulActionState{CruiseControlTaskId: uTaskId, CruiseControlState: kafkav1beta1.GracefulDownscaleRunning,
			TaskStarted: taskStartTime}, log)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", brokerIds)
	}

	return nil
}

func (r *CruiseControlTaskReconciler) checkCCTaskState(kafkaCluster *kafkav1beta1.KafkaCluster, brokersState map[string]kafkav1beta1.BrokerState, log logr.Logger) error {
	if len(brokersState) == 0 {
		return nil
	}

	// a CC task may run for one or multiple brokers (e.g add_broker for multiple brokers)
	// check that all brokers that we check the CC task status for have the same task id
	var ccTaskId string
	for _, brokerState := range brokersState {
		if ccTaskId == "" {
			ccTaskId = brokerState.GracefulActionState.CruiseControlTaskId
		} else if ccTaskId != brokerState.GracefulActionState.CruiseControlTaskId {
			return errors.New("multiple CC task ids found")
		}
	}

	if ccTaskId == "" {
		return errors.New("no CC task id provided to be checked")
	}

	// check cc task status
	cc := scale.NewCruiseControlScaler(kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	status, err := cc.GetCCTaskState(ccTaskId)
	if err != nil {
		log.Info("Cruise control communication error checking running task", "taskId", ccTaskId)
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}

	if status == kafkav1beta1.CruiseControlTaskNotFound || status == kafkav1beta1.CruiseControlTaskCompletedWithError {
		// CC task failed or not found in CC,
		// reschedule it by marking broker CruiseControlState= GracefulUpscaleRequired or GracefulDownscaleRequired
		var brokerIds []string
		requiredBrokerCCState := make(map[string]kafkav1beta1.GracefulActionState, len(brokersState))
		for brokerId, brokerState := range brokersState {
			requiredCCState, err := r.getCorrectRequiredCCState(brokerState.GracefulActionState.CruiseControlState)
			if err != nil {
				return err
			}

			brokerIds = append(brokerIds, brokerId)
			requiredBrokerCCState[brokerId] = kafkav1beta1.GracefulActionState{
				CruiseControlState:  requiredCCState,
				ErrorMessage:        "Previous cc task status invalid",
				CruiseControlTaskId: ccTaskId,
			}
		}

		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster, requiredBrokerCCState, log)

		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return errorfactory.New(errorfactory.CruiseControlTaskFailure{}, err, "CC task failed", fmt.Sprintf("cc task id: %s", ccTaskId))
	}

	if status == kafkav1beta1.CruiseControlTaskCompleted {
		// cc task completed successfully
		var brokerIds []string
		completedBrokerCCState := make(map[string]kafkav1beta1.GracefulActionState, len(brokersState))
		for brokerId, brokerState := range brokersState {
			brokerIds = append(brokerIds, brokerId)

			completedBrokerCCState[brokerId] = kafkav1beta1.GracefulActionState{
				CruiseControlState:  brokerState.GracefulActionState.CruiseControlState.Complete(),
				TaskStarted:         brokerState.GracefulActionState.TaskStarted,
				CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
			}
		}

		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster, completedBrokerCCState, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return nil
	}
	var brokersWithTimedOutCCTask []string
	timedOutBrokerCCState := make(map[string]kafkav1beta1.GracefulActionState)
	for brokerId, brokerState := range brokersState {
		if brokerState.GracefulActionState.CruiseControlState.IsRunningState() {
			parsedTime, err := ccutils.ParseTimeStampToUnixTime(brokerState.GracefulActionState.TaskStarted)
			if err != nil {
				return errors.WrapIf(err, "could not parse timestamp")
			}
			if time.Since(parsedTime).Minutes() > kafkaCluster.Spec.CruiseControlConfig.CruiseControlTaskSpec.GetDurationMinutes() {
				brokersWithTimedOutCCTask = append(brokersWithTimedOutCCTask, brokerId)
				requiredCCState, err := r.getCorrectRequiredCCState(brokerState.GracefulActionState.CruiseControlState)
				if err != nil {
					return err
				}

				timedOutBrokerCCState[brokerId] = kafkav1beta1.GracefulActionState{
					CruiseControlState:  requiredCCState,
					CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
					ErrorMessage:        "Timed out waiting for the task to complete",
					TaskStarted:         brokerState.GracefulActionState.TaskStarted,
				}
			}
		}
	}

	// task timed out
	if len(brokersWithTimedOutCCTask) > 0 {
		log.Info("Killing Cruise control task", "taskId", ccTaskId)
		cc := scale.NewCruiseControlScaler(kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
		err = cc.KillCCTask()

		if err != nil {
			return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
		}

		err = k8sutil.UpdateBrokerStatus(r.Client, brokersWithTimedOutCCTask, kafkaCluster, timedOutBrokerCCState, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokersWithTimedOutCCTask, ","))
		}
		return errorfactory.New(errorfactory.CruiseControlTaskTimeout{}, errors.New("cc task timed out"), fmt.Sprintf("cc task id: %s", ccTaskId))
	}

	// cc task still in progress
	log.Info("Cruise control task is still running", "taskId", ccTaskId)
	return errorfactory.New(errorfactory.CruiseControlTaskRunning{}, errors.New("cc task is still running"), fmt.Sprintf("cc task id: %s", ccTaskId))
}

// getCorrectRequiredCCState returns the correct Required CC state based on that we upscale or downscale
func (r *CruiseControlTaskReconciler) getCorrectRequiredCCState(ccState kafkav1beta1.CruiseControlState) (kafkav1beta1.CruiseControlState, error) {
	if ccState.IsDownscale() {
		return kafkav1beta1.GracefulDownscaleRequired, nil
	} else if ccState.IsUpscale() {
		return kafkav1beta1.GracefulUpscaleRequired, nil
	}

	return ccState, errors.NewWithDetails("could not determine if cruise control state is upscale or downscale", "ccState", ccState)
}

//TODO merge with checkCCTaskState into one func (hi-im-aren)
func (r *CruiseControlTaskReconciler) checkVolumeCCTaskState(kafkaCluster *kafkav1beta1.KafkaCluster, brokersVolumesState map[string]map[string]kafkav1beta1.VolumeState, log logr.Logger) error {
	if len(brokersVolumesState) == 0 {
		return nil
	}

	var ccTaskId string
	for _, brokerVolumesState := range brokersVolumesState {
		for _, volumeState := range brokerVolumesState {
			if ccTaskId == "" {
				ccTaskId = volumeState.CruiseControlTaskId
			} else if ccTaskId != volumeState.CruiseControlTaskId {
				return errors.New("multiple rebalance disk CC task ids found")
			}
		}
	}

	if ccTaskId == "" {
		return errors.New("no CC task id provided to be checked")
	}

	// check cc task status
	cc := scale.NewCruiseControlScaler(kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	status, err := cc.GetCCTaskState(ccTaskId)
	if err != nil {
		log.Info("Cruise control communication error checking running task", "taskId", ccTaskId)
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}

	if status == kafkav1beta1.CruiseControlTaskNotFound || status == kafkav1beta1.CruiseControlTaskCompletedWithError {
		// CC task failed or not found in CC,
		// reschedule it by marking volume CruiseControlVolumeState=GracefulDiskRebalanceRequired
		var brokerIds []string
		requiredBrokerVolumesCCState := make(map[string]map[string]kafkav1beta1.VolumeState, len(brokersVolumesState))
		for brokerId, volumesState := range brokersVolumesState {
			brokerIds = append(brokerIds, brokerId)

			volumesState := make(map[string]kafkav1beta1.VolumeState, len(volumesState))
			for mountPath := range volumesState {
				volumesState[mountPath] = kafkav1beta1.VolumeState{
					CruiseControlVolumeState: kafkav1beta1.GracefulDiskRebalanceRequired,
					ErrorMessage:             "Previous disk rebalance cc task status invalid",
				}
			}

			requiredBrokerVolumesCCState[brokerId] = volumesState
		}
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster, requiredBrokerVolumesCCState, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker volume(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return errorfactory.New(errorfactory.CruiseControlTaskFailure{}, err, "CC task failed", fmt.Sprintf("cc task id: %s", ccTaskId))
	}

	if status == kafkav1beta1.CruiseControlTaskCompleted {
		// cc task completed successfully
		var brokerIds []string
		rebalanceCompletedVolumesState := make(map[string]map[string]kafkav1beta1.VolumeState, len(brokersVolumesState))
		for brokerId, volumesState := range brokersVolumesState {
			brokerIds = append(brokerIds, brokerId)

			volumesStateSucceeded := make(map[string]kafkav1beta1.VolumeState, len(volumesState))
			for mountPath, volumeState := range volumesState {
				volumesStateSucceeded[mountPath] = kafkav1beta1.VolumeState{
					CruiseControlVolumeState: kafkav1beta1.GracefulDiskRebalanceSucceeded,
					TaskStarted:              volumeState.TaskStarted,
					CruiseControlTaskId:      volumeState.CruiseControlTaskId,
				}
			}

			rebalanceCompletedVolumesState[brokerId] = volumesStateSucceeded
		}

		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster, rebalanceCompletedVolumesState, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return nil
	}

	var brokersWithTimedOutCCTask []string
	brokersVolumesStateWithTimedOutDiskCCTask := make(map[string]map[string]kafkav1beta1.VolumeState)

	for brokerId, volumesState := range brokersVolumesState {
		volumesStateWithTimedOutDiskCCTask := make(map[string]kafkav1beta1.VolumeState)

		for mountPath, volumeState := range volumesState {
			if volumeState.CruiseControlVolumeState == kafkav1beta1.GracefulDiskRebalanceRunning {
				parsedTime, err := ccutils.ParseTimeStampToUnixTime(volumeState.TaskStarted)
				if err != nil {
					return errors.WrapIf(err, "could not parse timestamp")
				}

				if time.Since(parsedTime).Minutes() > kafkaCluster.Spec.CruiseControlConfig.CruiseControlTaskSpec.GetDurationMinutes() {
					volumesStateWithTimedOutDiskCCTask[mountPath] = kafkav1beta1.VolumeState{
						CruiseControlVolumeState: kafkav1beta1.GracefulDiskRebalanceRequired,
						CruiseControlTaskId:      volumeState.CruiseControlTaskId,
						ErrorMessage:             "Timed out waiting for the disk rebalance cc task to complete",
						TaskStarted:              volumeState.TaskStarted,
					}
				}
			}
		}
		if len(volumesStateWithTimedOutDiskCCTask) > 0 {
			brokersWithTimedOutCCTask = append(brokersWithTimedOutCCTask, brokerId)
			brokersVolumesStateWithTimedOutDiskCCTask[brokerId] = volumesStateWithTimedOutDiskCCTask
		}
	}

	// task timed out
	if len(brokersWithTimedOutCCTask) > 0 {
		log.Info("Killing Cruise control task", "taskId", ccTaskId)
		cc := scale.NewCruiseControlScaler(kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain(), kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)

		err = cc.KillCCTask()
		if err != nil {
			return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
		}
		err = k8sutil.UpdateBrokerStatus(r.Client, brokersWithTimedOutCCTask, kafkaCluster, brokersVolumesStateWithTimedOutDiskCCTask, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokersWithTimedOutCCTask, ","))
		}
		return errorfactory.New(errorfactory.CruiseControlTaskTimeout{}, errors.New("cc task timed out"), fmt.Sprintf("cc task id: %s", ccTaskId))
	}

	// cc task still in progress
	log.Info("Cruise control task is still running", "taskId", ccTaskId)
	return errorfactory.New(errorfactory.CruiseControlTaskRunning{}, errors.New("cc task is still running"), fmt.Sprintf("cc task id: %s", ccTaskId))
}

// SetupCruiseControlWithManager registers cruise control controller to the manager
func SetupCruiseControlWithManager(mgr ctrl.Manager) *ctrl.Builder {
	builder := ctrl.NewControllerManagedBy(mgr).For(&kafkav1beta1.KafkaCluster{}).Named("CruiseControl")

	builder.WithEventFilter(
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				object, err := meta.Accessor(e.ObjectNew)
				if err != nil {
					return false
				}
				if _, ok := object.(*kafkav1beta1.KafkaCluster); ok {
					old := e.ObjectOld.(*kafkav1beta1.KafkaCluster)
					new := e.ObjectNew.(*kafkav1beta1.KafkaCluster)
					if !reflect.DeepEqual(old.Status.BrokersState, new.Status.BrokersState) ||
						old.GetDeletionTimestamp() != new.GetDeletionTimestamp() ||
						old.GetGeneration() != new.GetGeneration() {
						return true
					}
					return false
				}
				return true
			},
		})

	return builder
}
