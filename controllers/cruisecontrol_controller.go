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
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	ccutils "github.com/banzaicloud/kafka-operator/pkg/util/cruisecontrol"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	kafkav1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
)

// CruiseControlReconciler reconciles a kafka cluster object
type CruiseControlReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Log logr.Logger
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaCluster,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaCluster/status,verbs=get;update;patch

func (r *CruiseControlReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	log := r.Log.WithValues("clusterName", request.Name, "clusterNamespace", request.Namespace)

	// Fetch the KafkaCluster instance
	instance := &v1beta1.KafkaCluster{}
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

	for brokerId, brokerStatus := range instance.Status.BrokersState {

		var err error
		// add and delete broker logic, create new cc task, etc.
		if brokerStatus.GracefulActionState.CruiseControlState.IsUpscale() {
			err = r.handlePodAddCCTask(instance, brokerId, brokerStatus, log)
		} else if brokerStatus.GracefulActionState.CruiseControlState.IsDownscale() {
			err = r.handlePodDeleteCCTask(instance, brokerId, brokerStatus, log)
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

		// Set volume states
		for _, volumeState := range brokerStatus.GracefulActionState.VolumeStates {
			switch volumeState.CruiseControlVolumeState {
			case v1beta1.GracefulDiskRebalanceRunning:
				// if succeeded set status to succeeded, if running don't do anything, if failed set status to failed
				err = r.checkVolumeCCTaskState(instance, []string{brokerId}, volumeState, v1beta1.GracefulDiskRebalanceSucceeded, log)
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

			case v1beta1.GracefulDiskRebalanceRequired:
				// create new cc task, set status to running
				taskId, startTime, err := scale.RebalanceDisks(brokerId, volumeState.MountPath, instance.Namespace, instance.Spec.CruiseControlConfig.CruiseControlEndpoint, instance.Name)
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
				err = k8sutil.UpdateBrokerStatus(r.Client, []string{brokerId}, instance, kafkav1beta1.VolumeState{
					CruiseControlTaskId: taskId,
					TaskStarted:         startTime,
				}, log)
				if err != nil {
					return requeueWithError(log, err.Error(), err)
				}
			}
		}
	}

	return reconciled()
}
func (r *CruiseControlReconciler) handlePodAddCCTask(kafkaCluster *v1beta1.KafkaCluster, brokerId string, brokerState kafkav1beta1.BrokerState, log logr.Logger) error {
	podList := &corev1.PodList{}

	matchingLabels := client.MatchingLabels(
		util.MergeLabels(
			kafka.LabelsForKafka(kafkaCluster.Name),
			map[string]string{"brokerId": brokerId},
		),
	)
	err := r.Client.List(context.TODO(), podList, client.InNamespace(kafkaCluster.Namespace), matchingLabels)
	if err != nil && len(podList.Items) == 0 {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed")
	}
	if len(podList.Items) == 1 {

		podStatus := podList.Items[0].DeepCopy().Status.Phase
		if podStatus == corev1.PodPending {
			return errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("broker pod is in pending state"), fmt.Sprintf("broker id: %s", brokerId))
		}
		if podStatus == corev1.PodRunning &&
			brokerState.GracefulActionState.CruiseControlState == v1beta1.GracefulUpscaleRequired {
			//trigger add broker in CC
			uTaskId, taskStartTime, scaleErr := scale.UpScaleCluster(brokerId, kafkaCluster.Namespace, kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
			if scaleErr != nil {
				log.Info("cruise control communication error during upscaling broker", "brokerId", brokerId)
				return errorfactory.New(errorfactory.CruiseControlNotReady{}, scaleErr, fmt.Sprintf("broker id: %s", brokerId))
			}
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{brokerId}, kafkaCluster,
				v1beta1.GracefulActionState{CruiseControlTaskId: uTaskId, CruiseControlState: v1beta1.GracefulUpscaleRunning,
					TaskStarted: taskStartTime}, log)
			if statusErr != nil {
				return errors.WrapIfWithDetails(err, "could not update status for broker", "id", brokerId)
			}
		}
		if brokerState.GracefulActionState.CruiseControlState == v1beta1.GracefulUpscaleRunning {
			err = r.checkCCTaskState(kafkaCluster, []string{brokerId}, brokerState, v1beta1.GracefulUpscaleSucceeded, log)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
func (r *CruiseControlReconciler) handlePodDeleteCCTask(kafkaCluster *v1beta1.KafkaCluster, brokerId string, brokerState kafkav1beta1.BrokerState, log logr.Logger) error {
	ccState := brokerState.GracefulActionState.CruiseControlState
	if ccState == v1beta1.GracefulDownscaleRequired {
		uTaskId, taskStartTime, err := scale.DownsizeCluster([]string{brokerId},
			kafkaCluster.Namespace, kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
		if err != nil {
			log.Info("cruise control communication error during downscaling broker(s)", "id(s)", brokerId)
			return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, fmt.Sprintf("broker(s) id(s): %s", brokerId))
		}
		err = k8sutil.UpdateBrokerStatus(r.Client, []string{brokerId}, kafkaCluster,
			v1beta1.GracefulActionState{CruiseControlTaskId: uTaskId, CruiseControlState: v1beta1.GracefulDownscaleRunning,
				TaskStarted: taskStartTime}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", brokerId)
		}
	}

	if ccState == v1beta1.GracefulDownscaleRunning {
		err := r.checkCCTaskState(kafkaCluster, []string{brokerId}, brokerState, v1beta1.GracefulDownscaleSucceeded, log)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *CruiseControlReconciler) checkCCTaskState(kafkaCluster *v1beta1.KafkaCluster, brokerIds []string, brokerState v1beta1.BrokerState, cruiseControlState v1beta1.CruiseControlState, log logr.Logger) error {

	// check cc task status
	status, err := scale.GetCCTaskState(brokerState.GracefulActionState.CruiseControlTaskId,
		kafkaCluster.Namespace, kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	if err != nil {
		log.Info("Cruise control communication error checking running task", "taskId", brokerState.GracefulActionState.CruiseControlTaskId)
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}

	if status == v1beta1.CruiseControlTaskNotFound || status == v1beta1.CruiseControlTaskCompletedWithError {
		// CC task failed or not found in CC,
		// reschedule it by marking broker CruiseControlState= GracefulUpscaleRequired or GracefulDownscaleRequired
		requiredCCState, err := r.getCorrectRequiredCCState(brokerState.GracefulActionState.CruiseControlState)
		if err != nil {
			return err
		}

		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
			v1beta1.GracefulActionState{CruiseControlState: requiredCCState,
				ErrorMessage: "Previous cc task status invalid",
			}, log)

		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return errorfactory.New(errorfactory.CruiseControlTaskFailure{}, err, "CC task failed", fmt.Sprintf("cc task id: %s", brokerState.GracefulActionState.CruiseControlTaskId))
	}

	if status == v1beta1.CruiseControlTaskCompleted {
		// cc task completed successfully
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
			v1beta1.GracefulActionState{CruiseControlState: cruiseControlState,
				TaskStarted:         brokerState.GracefulActionState.TaskStarted,
				CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return nil
	}
	// cc task still in progress
	parsedTime, err := ccutils.ParseTimeStampToUnixTime(brokerState.GracefulActionState.TaskStarted)
	if err != nil {
		return errors.WrapIf(err, "could not parse timestamp")
	}
	if time.Now().Sub(parsedTime).Minutes() < kafkaCluster.Spec.CruiseControlConfig.CruiseControlTaskSpec.GetDurationMinutes() {
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
			v1beta1.GracefulActionState{TaskStarted: brokerState.GracefulActionState.TaskStarted,
				CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
				CruiseControlState:  brokerState.GracefulActionState.CruiseControlState,
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		log.Info("Cruise control task is still running", "taskId", brokerState.GracefulActionState.CruiseControlTaskId)
		return errorfactory.New(errorfactory.CruiseControlTaskRunning{}, errors.New("cc task is still running"), fmt.Sprintf("cc task id: %s", brokerState.GracefulActionState.CruiseControlTaskId))
	}
	// task timed out
	log.Info("Killing Cruise control task", "taskId", brokerState.GracefulActionState.CruiseControlTaskId)
	err = scale.KillCCTask(kafkaCluster.Namespace, kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	if err != nil {
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}
	err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
		v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRequired,
			CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
			ErrorMessage:        "Timed out waiting for the task to complete",
			TaskStarted:         brokerState.GracefulActionState.TaskStarted,
		}, log)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
	}
	return errorfactory.New(errorfactory.CruiseControlTaskTimeout{}, errors.New("cc task timed out"), fmt.Sprintf("cc task id: %s", brokerState.GracefulActionState.CruiseControlTaskId))
}

// getCorrectRequiredCCState returns the correct Required CC state based on that we upscale or downscale
func (r *CruiseControlReconciler) getCorrectRequiredCCState(ccState kafkav1beta1.CruiseControlState) (kafkav1beta1.CruiseControlState, error) {
	if ccState.IsDownscale() {
		return kafkav1beta1.GracefulDownscaleRequired, nil
	} else if ccState.IsUpscale() {
		return kafkav1beta1.GracefulUpscaleRequired, nil
	}

	return ccState, errors.NewWithDetails("could not determine if cruise control state is upscale or downscale", "ccState", ccState)
}

//TODO merge with checkCCTaskState into one func (hi-im-aren)
func (r *CruiseControlReconciler) checkVolumeCCTaskState(kafkaCluster *v1beta1.KafkaCluster, brokerIds []string, volumeState v1beta1.VolumeState, cruiseControlVolumeState v1beta1.CruiseControlVolumeState, log logr.Logger) error {

	// check cc task status
	status, err := scale.GetCCTaskState(volumeState.CruiseControlTaskId,
		kafkaCluster.Namespace, kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	if err != nil {
		log.Info("Cruise control communication error checking running task", "taskId", volumeState.CruiseControlTaskId)
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}

	if status == v1beta1.CruiseControlTaskNotFound || status == v1beta1.CruiseControlTaskCompletedWithError {
		// CC task failed or not found in CC,
		// reschedule it by marking volume CruiseControlVolumeState=GracefulDiskRebalanceRequired
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
			kafkav1beta1.VolumeState{
				CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired,
				ErrorMessage:             "Previous cc task status invalid",
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker volume(s)", "id(s)", strings.Join(brokerIds, ","), "mountPath:", volumeState.MountPath)
		}
		return errorfactory.New(errorfactory.CruiseControlTaskFailure{}, err, "CC task failed", fmt.Sprintf("cc task id: %s", volumeState.CruiseControlTaskId))
	}

	if status == v1beta1.CruiseControlTaskCompleted {
		// cc task completed successfully
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
			kafkav1beta1.VolumeState{CruiseControlVolumeState: cruiseControlVolumeState,
				TaskStarted:         volumeState.TaskStarted,
				CruiseControlTaskId: volumeState.CruiseControlTaskId,
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return nil
	}
	// cc task still in progress
	parsedTime, err := ccutils.ParseTimeStampToUnixTime(volumeState.TaskStarted)
	if err != nil {
		return errors.WrapIf(err, "could not parse timestamp")
	}
	if time.Now().Sub(parsedTime).Minutes() < kafkaCluster.Spec.CruiseControlConfig.CruiseControlTaskSpec.GetDurationMinutes() {
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
			kafkav1beta1.VolumeState{TaskStarted: volumeState.TaskStarted,
				CruiseControlTaskId:      volumeState.CruiseControlTaskId,
				CruiseControlVolumeState: volumeState.CruiseControlVolumeState,
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		log.Info("Cruise control task is still running", "taskId", volumeState.CruiseControlTaskId)
		return errorfactory.New(errorfactory.CruiseControlTaskRunning{}, errors.New("cc task is still running"), fmt.Sprintf("cc task id: %s", volumeState.CruiseControlTaskId))
	}
	// task timed out
	log.Info("Killing Cruise control task", "taskId", volumeState.CruiseControlTaskId)
	err = scale.KillCCTask(kafkaCluster.Namespace, kafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, kafkaCluster.Name)
	if err != nil {
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}
	err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, kafkaCluster,
		kafkav1beta1.VolumeState{CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired,
			CruiseControlTaskId: volumeState.CruiseControlTaskId,
			ErrorMessage:        "Timed out waiting for the task to complete",
			TaskStarted:         volumeState.TaskStarted,
		}, log)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
	}
	return errorfactory.New(errorfactory.CruiseControlTaskTimeout{}, errors.New("cc task timed out"), fmt.Sprintf("cc task id: %s", volumeState.CruiseControlTaskId))
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
				if _, ok := object.(*v1beta1.KafkaCluster); ok {
					old := e.ObjectOld.(*v1beta1.KafkaCluster)
					new := e.ObjectNew.(*v1beta1.KafkaCluster)
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
