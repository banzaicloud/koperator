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
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrol"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrolmonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafkamonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/pki"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
)

var clusterFinalizer = "finalizer.kafkaclusters.banzaicloud.banzaicloud.io"

// KafkaClusterReconciler reconciles a KafkaCluster object
type KafkaClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a KafkaCluster object and makes changes based on the state read
// and what is in the KafkaCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch

func (r *KafkaClusterReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("kafkacluster", request.NamespacedName, "Request.Name", request.Name)
	log.Info("Reconciling KafkaCluster")
	// Fetch the KafkaCluster instance
	instance := &banzaicloudv1alpha1.KafkaCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconciled()
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Check if marked for deletion and run finalizers
	if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
		return r.checkFinalizers(log, instance)
	}

	if err := k8sutil.UpdateCRStatus(r.Client, instance, banzaicloudv1alpha1.KafkaClusterReconciling, log); err != nil {
		return ctrl.Result{}, err
	}

	reconcilers := []resources.ComponentReconciler{
		pki.New(r.Client, r.Scheme, instance),
		envoy.New(r.Client, instance),
		kafkamonitoring.New(r.Client, instance),
		cruisecontrolmonitoring.New(r.Client, instance),
		kafka.New(r.Client, instance),
		cruisecontrol.New(r.Client, instance),
	}

	for _, rec := range reconcilers {
		err = rec.Reconcile(log)
		if err != nil {
			switch err.(type) {
			case errorfactory.BrokersUnreachable:
				log.Info("Brokers unreachable, may still be starting up")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(15) * time.Second,
				}, nil
			case errorfactory.BrokersNotReady:
				log.Info("Brokers not ready, may still be starting up")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(15) * time.Second,
				}, nil
			case errorfactory.ResourceNotReady:
				log.Info("A new resource was not found, may not be ready")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(5) * time.Second,
				}, nil
			case errorfactory.CreateTopicError:
				log.Info("Could not create CC topic, either less than 3 brokers or not all are ready")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(15) * time.Second,
				}, nil
			default:
				return requeueWithError(log, err.Error(), err)
			}
		}
	}

	log.Info("ensuring finalizer on kafkacluster")
	if err = r.ensureFinalizer(instance); err != nil {
		return requeueWithError(log, "failed to ensure finalizers on kafkacluster instance", err)
	}

	if err := k8sutil.UpdateCRStatus(r.Client, instance, banzaicloudv1alpha1.KafkaClusterRunning, log); err != nil {
		return requeueWithError(log, err.Error(), err)
	}

	return reconciled()
}

func (r *KafkaClusterReconciler) checkFinalizers(log logr.Logger, cluster *v1alpha1.KafkaCluster) (ctrl.Result, error) {
	log.Info("KafkaCluster is marked for deletion, checking for children")
	if !util.StringSliceContains(cluster.GetFinalizers(), clusterFinalizer) {
		return reconciled()
	}

	var err error

	// Delete all kafkatopics associated with us
	var childTopics v1alpha1.KafkaTopicList
	if err = r.Client.List(context.TODO(), &childTopics); err != nil {
		return requeueWithError(log, "failed to list kafkatopics", err)
	}
	for _, topic := range childTopics.Items {
		if belongsToCluster(topic.Spec.ClusterRef, cluster) {
			log.Info(fmt.Sprintf("Deleting associated topic %s", topic.Name))
			if err = r.Client.Delete(context.TODO(), &topic); err != nil {
				return requeueWithError(log, "failed to delete kafkatopic", err)
			}
		}
	}

	// Delete all kafkausers associated with us
	var childUsers v1alpha1.KafkaUserList
	if err = r.Client.List(context.TODO(), &childUsers); err != nil {
		return requeueWithError(log, "failed to list kafkausers", err)
	}
	for _, user := range childUsers.Items {
		if belongsToCluster(user.Spec.ClusterRef, cluster) {
			log.Info(fmt.Sprintf("Deleting associated user %s", user.Name))
			if err = r.Client.Delete(context.TODO(), &user); err != nil {
				return requeueWithError(log, "failed to delete kafkauser", err)
			}
		}
	}

	log.Info("Removing finalizers from kafkacluster instance")
	if err := r.removeFinalizer(cluster); err != nil {
		return requeueWithError(log, "failed to remove finalizer", err)
	}

	return ctrl.Result{}, nil
}

func (r *KafkaClusterReconciler) ensureFinalizer(cluster *v1alpha1.KafkaCluster) (err error) {
	if util.StringSliceContains(cluster.GetFinalizers(), clusterFinalizer) {
		return
	}
	cluster.SetFinalizers(append(cluster.GetFinalizers(), clusterFinalizer))
	err = r.Client.Update(context.TODO(), cluster)
	return
}

func (r *KafkaClusterReconciler) removeFinalizer(cluster *v1alpha1.KafkaCluster) error {
	cluster.SetFinalizers(util.StringSliceRemove(cluster.GetFinalizers(), clusterFinalizer))
	return r.Client.Update(context.TODO(), cluster)
}

func belongsToCluster(ref v1alpha1.ClusterReference, cluster *v1alpha1.KafkaCluster) bool {
	if ref.Name == cluster.Name && ref.Namespace == cluster.Namespace {
		return true
	}
	return false
}

func SetupKafkaClusterWithManager(mgr ctrl.Manager, log logr.Logger) *ctrl.Builder {

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&banzaicloudv1alpha1.KafkaCluster{})

	kafkaWatches(builder)
	envoyWatches(builder)
	cruiseControlWatches(builder)

	builder.WithEventFilter(
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				object, err := meta.Accessor(e.Object)
				if err != nil {
					return false
				}
				if _, ok := object.(*banzaicloudv1alpha1.KafkaCluster); ok {
					return true
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				object, err := meta.Accessor(e.ObjectNew)
				if err != nil {
					return false
				}
				switch object.(type) {
				case *corev1.Pod, *corev1.ConfigMap, *corev1.PersistentVolumeClaim:
					patchResult, err := patch.DefaultPatchMaker.Calculate(e.ObjectOld, e.ObjectNew)
					if err != nil {
						log.Error(err, "could not match objects", "kind", e.ObjectOld.GetObjectKind())
					} else if patchResult.IsEmpty() {
						return false
					}
				case *banzaicloudv1alpha1.KafkaCluster:
					old := e.ObjectOld.(*banzaicloudv1alpha1.KafkaCluster)
					new := e.ObjectNew.(*banzaicloudv1alpha1.KafkaCluster)
					if !reflect.DeepEqual(old.Spec, new.Spec) ||
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

func kafkaWatches(builder *ctrl.Builder) *ctrl.Builder {
	return builder.
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Pod{})
}

func envoyWatches(builder *ctrl.Builder) *ctrl.Builder {
	return builder.
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{})
}

func cruiseControlWatches(builder *ctrl.Builder) *ctrl.Builder {
	return builder.
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{})
}
