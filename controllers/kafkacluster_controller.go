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
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/pki"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrol"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrolmonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafkamonitoring"
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
)

var clusterFinalizer = "finalizer.kafkaclusters.kafka.banzaicloud.io"
var clusterTopicsFinalizer = "topics.kafkaclusters.kafka.banzaicloud.io"
var clusterUsersFinalizer = "users.kafkaclusters.kafka.banzaicloud.io"

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
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch

func (r *KafkaClusterReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("kafkacluster", request.NamespacedName, "Request.Name", request.Name)
	log.Info("Reconciling KafkaCluster")
	// Fetch the KafkaCluster instance
	instance := &v1beta1.KafkaCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconciled()
		}
		// Error reading the object - requeue the request.
		return requeueWithError(log, err.Error(), err)
	}

	// Check if marked for deletion and run finalizers
	if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
		return r.checkFinalizers(log, instance)
	}

	if instance.Status.State != v1beta1.KafkaClusterRollingUpgrading {
		if err := k8sutil.UpdateCRStatus(r.Client, instance, v1beta1.KafkaClusterReconciling, log); err != nil {
			return requeueWithError(log, err.Error(), err)
		}
	}

	reconcilers := []resources.ComponentReconciler{
		envoy.New(r.Client, instance),
		kafkamonitoring.New(r.Client, instance),
		cruisecontrolmonitoring.New(r.Client, instance),
		kafka.New(r.Client, r.Scheme, instance),
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
				log.Info("A new resource was not found or may not be ready")
				log.Info(err.Error())
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(7) * time.Second,
				}, nil
			case errorfactory.ReconcileRollingUpgrade:
				log.Info("Rolling Upgrade in Progress")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(15) * time.Second,
				}, nil
			default:
				return requeueWithError(log, err.Error(), err)
			}
		}
	}

	// Fetch the most recent cluster instance and ensure finalizer
	if instance, err = r.fetchMostRecent(instance); err != nil {
		return requeueWithError(log, "failed to fetch latest kafkacluster instance", err)
	}

	log.Info("ensuring finalizers on kafkacluster")
	if instance, err = r.ensureFinalizers(instance); err != nil {
		return requeueWithError(log, "failed to ensure finalizers on kafkacluster instance", err)
	}

	//Update rolling upgrade last successful state
	if instance.Status.State == v1beta1.KafkaClusterRollingUpgrading {
		if err := k8sutil.UpdateRollingUpgradeState(r.Client, instance, time.Now(), log); err != nil {
			return requeueWithError(log, err.Error(), err)
		}
	}

	if err := k8sutil.UpdateCRStatus(r.Client, instance, v1beta1.KafkaClusterRunning, log); err != nil {
		return requeueWithError(log, err.Error(), err)
	}

	return reconciled()
}

func (r *KafkaClusterReconciler) checkFinalizers(log logr.Logger, cluster *v1beta1.KafkaCluster) (ctrl.Result, error) {
	log.Info("KafkaCluster is marked for deletion, checking for children")

	// If the main finalizer is gone then we've already finished up
	if !util.StringSliceContains(cluster.GetFinalizers(), clusterFinalizer) {
		return reconciled()
	}

	var err error

	var namespaces corev1.NamespaceList
	if err := r.Client.List(context.TODO(), &namespaces); err != nil {
		return requeueWithError(log, "failed to get namespace list", err)
	}

	// If we haven't deleted all kafkatopics yet, iterate namespaces and delete all kafkatopics
	// with the matching label.
	if util.StringSliceContains(cluster.GetFinalizers(), clusterTopicsFinalizer) {
		log.Info(fmt.Sprintf("Sending delete kafkatopics request to all namespaces for cluster %s/%s", cluster.Namespace, cluster.Name))
		for _, ns := range namespaces.Items {
			if err := r.Client.DeleteAllOf(
				context.TODO(),
				&v1alpha1.KafkaTopic{},
				client.InNamespace(ns.Name),
				client.MatchingLabels{clusterRefLabel: clusterLabelString(cluster)},
			); err != nil {
				if client.IgnoreNotFound(err) != nil {
					return requeueWithError(log, "failed to send delete request for children kafkatopics", err)
				}
				log.Info(fmt.Sprintf("No matching kafkatopics in namespace: %s", ns.Name))
			}

		}
		if cluster, err = r.removeFinalizer(cluster, clusterTopicsFinalizer); err != nil {
			return requeueWithError(log, "failed to remove topics finalizer from kafkacluster", err)
		}
	}

	// If any of the topics still exist, it means their finalizer is still running.
	// Wait to make sure we have fully cleaned up zookeeper. Also if we delete
	// our kafkausers before all topics are finished cleaning up, we will lose
	// our controller certificate.
	log.Info("Ensuring all topics have finished cleaning up")
	var childTopics v1alpha1.KafkaTopicList
	if err = r.Client.List(context.TODO(), &childTopics); err != nil {
		return requeueWithError(log, "failed to list kafkatopics", err)
	}
	for _, topic := range childTopics.Items {
		if belongsToCluster(topic.Spec.ClusterRef, cluster) {
			log.Info(fmt.Sprintf("Still waiting for topic %s/%s to be deleted", topic.Namespace, topic.Name))
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Duration(3) * time.Second,
			}, nil
		}
	}

	// If we haven't deleted all kafkausers yet, iterate namespaces and delete all kafkausers
	// with the matching label.
	if util.StringSliceContains(cluster.GetFinalizers(), clusterUsersFinalizer) {
		log.Info(fmt.Sprintf("Sending delete kafkausers request to all namespaces for cluster %s/%s", cluster.Namespace, cluster.Name))
		for _, ns := range namespaces.Items {
			if err := r.Client.DeleteAllOf(
				context.TODO(),
				&v1alpha1.KafkaUser{},
				client.InNamespace(ns.Name),
				client.MatchingLabels{clusterRefLabel: clusterLabelString(cluster)},
			); err != nil {
				if client.IgnoreNotFound(err) != nil {
					return requeueWithError(log, "failed to send delete request for children kafkausers", err)
				}
				log.Info(fmt.Sprintf("No matching kafkausers in namespace: %s", ns.Name))
			}
		}
		if cluster, err = r.removeFinalizer(cluster, clusterUsersFinalizer); err != nil {
			return requeueWithError(log, "failed to remove users finalizer from kafkacluster", err)
		}
	}

	// Do any necessary PKI cleanup - a PKI backend should make sure any
	// user finalizations are done before it does its final cleanup
	log.Info("Tearing down any PKI resources for the kafkacluster")
	if err = pki.GetPKIManager(r.Client, cluster).FinalizePKI(log); err != nil {
		switch err.(type) {
		case errorfactory.ResourceNotReady:
			log.Info("The PKI is not ready to be torn down")
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Duration(5) * time.Second,
			}, nil
		default:
			return requeueWithError(log, "failed to finalize PKI", err)
		}
	}

	log.Info("Finalizing deletion of kafkacluster instance")
	if _, err = r.removeFinalizer(cluster, clusterFinalizer); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// We may have been a requeue from earlier with all conditions met - but with
			// the state of the finalizer not yet reflected in the response we got.
			return reconciled()
		}
		return requeueWithError(log, "failed to remove main finalizer", err)
	}

	return ctrl.Result{}, nil
}

func (r *KafkaClusterReconciler) ensureFinalizers(cluster *v1beta1.KafkaCluster) (updated *v1beta1.KafkaCluster, err error) {
	for _, finalizer := range []string{clusterFinalizer, clusterTopicsFinalizer, clusterUsersFinalizer} {
		if util.StringSliceContains(cluster.GetFinalizers(), finalizer) {
			continue
		}
		cluster.SetFinalizers(append(cluster.GetFinalizers(), finalizer))
	}
	return r.updateAndFetchLatest(cluster)
}

func (r *KafkaClusterReconciler) removeFinalizer(cluster *v1beta1.KafkaCluster, finalizer string) (updated *v1beta1.KafkaCluster, err error) {
	cluster.SetFinalizers(util.StringSliceRemove(cluster.GetFinalizers(), finalizer))
	return r.updateAndFetchLatest(cluster)
}

func (r *KafkaClusterReconciler) updateAndFetchLatest(cluster *v1beta1.KafkaCluster) (*v1beta1.KafkaCluster, error) {
	if err := r.Client.Update(context.TODO(), cluster); err != nil {
		return nil, err
	}
	return r.fetchMostRecent(cluster)
}

func (r *KafkaClusterReconciler) fetchMostRecent(cluster *v1beta1.KafkaCluster) (*v1beta1.KafkaCluster, error) {
	updated := &v1beta1.KafkaCluster{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, updated)
	return updated, err
}

func belongsToCluster(ref v1alpha1.ClusterReference, cluster *v1beta1.KafkaCluster) bool {
	if ref.Name == cluster.Name && ref.Namespace == cluster.Namespace {
		return true
	}
	return false
}

// SetupKafkaClusterWithManager registers kafka cluster controller to the manager
func SetupKafkaClusterWithManager(mgr ctrl.Manager, log logr.Logger) *ctrl.Builder {

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.KafkaCluster{})

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
				if _, ok := object.(*v1beta1.KafkaCluster); ok {
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
				case *v1beta1.KafkaCluster:
					old := e.ObjectOld.(*v1beta1.KafkaCluster)
					new := e.ObjectNew.(*v1beta1.KafkaCluster)
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
