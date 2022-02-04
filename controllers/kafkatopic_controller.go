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
	"errors"
	"reflect"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/util"
)

var topicFinalizer = "finalizer.kafkatopics.kafka.banzaicloud.io"

// SetupKafkaTopicWithManager registers kafka topic controller with manager
func SetupKafkaTopicWithManager(mgr ctrl.Manager, maxConcurrentReconciles int) *ctrl.Builder {
	builder := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.KafkaTopic{}).Named("KafkaTopic")
	builder.WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles})

	builder.WithEventFilter(
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				object, err := meta.Accessor(e.Object)
				if err != nil {
					return false
				}
				// Skip object if v1alpha1.OwnershipAnnotation is set as it is owned by other system.
				if ok := util.ObjectManagedByClusterRegistry(object); ok {
					return false
				}
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				object, err := meta.Accessor(e.Object)
				if err != nil {
					return false
				}
				// Skip object if v1alpha1.OwnershipAnnotation is set as it is owned by other system.
				if ok := util.ObjectManagedByClusterRegistry(object); ok {
					return false
				}
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				object, err := meta.Accessor(e.ObjectNew)
				if err != nil {
					return false
				}
				// Skip object if v1alpha1.OwnershipAnnotation is set as it is owned by other system.
				if ok := util.ObjectManagedByClusterRegistry(object); ok {
					return false
				}
				return true
			},
		})
	return builder
}

// blank assignment to verify that KafkaTopicReconciler implements reconcile.KafkaTopicReconciler
var _ reconcile.Reconciler = &KafkaTopicReconciler{}

// KafkaTopicReconciler reconciles a KafkaTopic object
type KafkaTopicReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkatopics,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkatopics/status,verbs=get;update;patch

// Reconcile reconciles the kafka topic
func (r *KafkaTopicReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logr.FromContextOrDiscard(ctx).WithValues("kafkatopic", request.NamespacedName, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KafkaTopic")
	var err error

	// Fetch the KafkaTopic instance
	instance := &v1alpha1.KafkaTopic{}
	if err = r.Client.Get(ctx, request.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconciled()
		}
		// Error reading the object - requeue the request.
		return requeueWithError(reqLogger, err.Error(), err)
	}

	// Get the referenced kafkacluster
	clusterNamespace := getClusterRefNamespace(instance.Namespace, instance.Spec.ClusterRef)
	var cluster *v1beta1.KafkaCluster
	if cluster, err = k8sutil.LookupKafkaCluster(r.Client, instance.Spec.ClusterRef.Name, clusterNamespace); err != nil {
		// This shouldn't trigger anymore, but leaving it here as a safetybelt
		if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
			reqLogger.Info("Cluster is already gone, there is nothing we can do")
			if err = r.removeFinalizer(ctx, instance); err != nil {
				return requeueWithError(reqLogger, "failed to remove finalizer", err)
			}
			return reconciled()
		}

		// the cluster does not exist - should have been caught pre-flight
		return requeueWithError(reqLogger, "failed to lookup referenced cluster", err)
	}

	// Get a kafka connection
	broker, close, err := newKafkaFromCluster(r.Client, cluster)
	if err != nil {
		return checkBrokerConnectionError(reqLogger, err)
	}
	defer close()

	// Check if marked for deletion and if so run finalizers
	if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
		return r.checkFinalizers(ctx, broker, instance)
	}

	// Check if the topic already exists
	existing, err := broker.GetTopic(instance.Spec.Name)
	if err != nil {
		return requeueWithError(reqLogger, "failure checking for existing topic", err)
	}

	// It may take several seconds after topic is created successfully for all the brokers
	// to become aware that the topic has been created
	if instance.Status.State == v1alpha1.TopicStateCreated && existing == nil {
		return requeueWithError(reqLogger, instance.Spec.Name, errors.New("topic is still creating"))
	}

	// we got a topic back
	if existing != nil {
		reqLogger.Info("Topic already exists, verifying configuration")
		// Ensure partition count
		if changed, err := broker.EnsurePartitionCount(instance.Spec.Name, instance.Spec.Partitions); err != nil {
			return requeueWithError(reqLogger, "failed to ensure topic partition count", err)
		} else if changed {
			reqLogger.Info("Increased partition count for topic")
		}
		// Ensure topic configurations
		if err = broker.EnsureTopicConfig(instance.Spec.Name, util.MapStringStringPointer(instance.Spec.Config)); err != nil {
			return requeueWithError(reqLogger, "failure to ensure topic config", err)
		}
		reqLogger.Info("Verified partitions and configuration for topic")
	} else if err = broker.CreateTopic(&kafkaclient.CreateTopicOptions{
		// Create the topic
		Name:              instance.Spec.Name,
		Partitions:        instance.Spec.Partitions,
		ReplicationFactor: int16(instance.Spec.ReplicationFactor),
		Config:            util.MapStringStringPointer(instance.Spec.Config),
	}); err != nil {
		return requeueWithError(reqLogger, "failed to create kafka topic", err)
	}

	// ensure kafkaCluster label
	if instance, err = r.ensureClusterLabel(ctx, cluster, instance); err != nil {
		return requeueWithError(reqLogger, "failed to ensure kafkacluster label on topic", err)
	}

	// ensure a finalizer for cleanup on deletion
	if !util.StringSliceContains(instance.GetFinalizers(), topicFinalizer) {
		reqLogger.Info("Adding Finalizer for the KafkaTopic")
		instance.SetFinalizers(append(instance.GetFinalizers(), topicFinalizer))
		if instance, err = r.updateAndFetchLatest(ctx, instance); err != nil {
			return requeueWithError(reqLogger, "failed to add Finalizer to KafkaTopic", err)
		}
	}

	// set topic status as created
	if instance.Status.State != v1alpha1.TopicStateCreated {
		instance.Status = v1alpha1.KafkaTopicStatus{State: v1alpha1.TopicStateCreated}
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return requeueWithError(reqLogger, "failed to update kafkatopic status", err)
		}
	}

	reqLogger.Info("Ensured topic")

	return reconciled()
}

func (r *KafkaTopicReconciler) ensureClusterLabel(ctx context.Context, cluster *v1beta1.KafkaCluster, topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopic, error) {
	labels := applyClusterRefLabel(cluster, topic.GetLabels())
	if !reflect.DeepEqual(labels, topic.GetLabels()) {
		topic.SetLabels(labels)
		return r.updateAndFetchLatest(ctx, topic)
	}
	return topic, nil
}

func (r *KafkaTopicReconciler) updateAndFetchLatest(ctx context.Context, topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopic, error) {
	typeMeta := topic.TypeMeta
	err := r.Client.Update(ctx, topic)
	if err != nil {
		return nil, err
	}
	topic.TypeMeta = typeMeta
	return topic, nil
}

func (r *KafkaTopicReconciler) checkFinalizers(ctx context.Context, broker kafkaclient.KafkaClient, topic *v1alpha1.KafkaTopic) (reconcile.Result, error) {
	reqLogger := logr.FromContextOrDiscard(ctx)
	reqLogger.Info("Kafka topic is marked for deletion")
	var err error
	if util.StringSliceContains(topic.GetFinalizers(), topicFinalizer) {
		if err = r.finalizeKafkaTopic(reqLogger, broker, topic); err != nil {
			return requeueWithError(reqLogger, "failed to finalize kafkatopic", err)
		}
		if err = r.removeFinalizer(ctx, topic); err != nil {
			return requeueWithError(reqLogger, "failed to remove finalizer from kafkatopic", err)
		}
	}
	return reconciled()
}

func (r *KafkaTopicReconciler) removeFinalizer(ctx context.Context, topic *v1alpha1.KafkaTopic) error {
	topic.SetFinalizers(util.StringSliceRemove(topic.GetFinalizers(), topicFinalizer))
	_, err := r.updateAndFetchLatest(ctx, topic)
	return err
}

func (r *KafkaTopicReconciler) finalizeKafkaTopic(reqLogger logr.Logger, broker kafkaclient.KafkaClient, topic *v1alpha1.KafkaTopic) error {
	exists, err := broker.GetTopic(topic.Spec.Name)
	if err != nil {
		return err
	}
	if exists != nil {
		// DeleteTopic with wait to make sure it goes down fully in case of cluster
		// deletion.
		// TODO (tinyzimmer): Perhaps this should only wait when it's the cluster
		// being deleted, and use false when it's just the topic itself. Also,
		// if delete.topic.enable=false this may hang forever, so should maybe
		// check if that's the case during a wait.
		if err = broker.DeleteTopic(topic.Spec.Name, true); err != nil {
			return err
		}
		reqLogger.Info("Deleted topic")
	}
	return nil
}
