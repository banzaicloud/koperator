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

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var topicFinalizer = "finalizer.kafkatopics.kafka.banzaicloud.io"

// TODO (tinyzimmer): Should this maybe just be one master sync routine
// checking ALL known topics at an interval - or is it better to just have one
// per topic
var syncRoutines = make(map[types.UID]struct{}, 0)

// SetupKafkaTopicWithManager registers kafka topic controller with manager
func SetupKafkaTopicWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	r := &KafkaTopicReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("KafkaTopic"),
	}

	c, err := controller.New("kafkatopic", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KafkaTopic
	err = c.Watch(&source.Kind{Type: &v1alpha1.KafkaTopic{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that KafkaTopicReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &KafkaTopicReconciler{}

// KafkaTopicReconciler reconciles a KafkaTopic object
type KafkaTopicReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkatopics,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkatopics/status,verbs=get;update;patch

// Reconcile reconciles the kafka topic
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *KafkaTopicReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("kafkatopic", request.NamespacedName, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KafkaTopic")
	var err error

	// Fetch the KafkaTopic instance
	instance := &v1alpha1.KafkaTopic{}
	if err = r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
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
			if err = r.removeFinalizer(instance); err != nil {
				return requeueWithError(reqLogger, "failed to remove finalizer", err)
			}
			return reconciled()
		}

		// the cluster does not exist - should have been caught pre-flight
		return requeueWithError(reqLogger, "failed to lookup referenced cluster", err)
	}

	// Get a kafka connection
	broker, close, err := newBrokerConnection(reqLogger, r.Client, cluster)
	if err != nil {
		return checkBrokerConnectionError(reqLogger, err)
	}
	defer close()

	// Check if marked for deletion and if so run finalizers
	if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
		return r.checkFinalizers(reqLogger, broker, instance)
	}

	// Check if the topic already exists
	existing, err := broker.GetTopic(instance.Spec.Name)
	if err != nil {
		return requeueWithError(reqLogger, "failure checking for existing topic", err)
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

	} else {

		// Create the topic
		if err = broker.CreateTopic(&kafkaclient.CreateTopicOptions{
			Name:              instance.Spec.Name,
			Partitions:        instance.Spec.Partitions,
			ReplicationFactor: int16(instance.Spec.ReplicationFactor),
			Config:            util.MapStringStringPointer(instance.Spec.Config),
		}); err != nil {
			return requeueWithError(reqLogger, "failed to create kafka topic", err)
		}

	}

	// set controller reference to parent cluster
	if instance, err = r.ensureControllerReference(cluster, instance); err != nil {
		return requeueWithError(reqLogger, "failed to ensure controller reference", err)
	}

	// ensure kafkaCluster label
	if instance, err = r.ensureClusterLabel(cluster, instance); err != nil {
		return requeueWithError(reqLogger, "failed to ensure kafkacluster label on topic", err)
	}

	// ensure a finalizer for cleanup on deletion
	if !util.StringSliceContains(instance.GetFinalizers(), topicFinalizer) {
		reqLogger.Info("Adding Finalizer for the KafkaTopic")
		instance.SetFinalizers(append(instance.GetFinalizers(), topicFinalizer))
	}

	// push any changes
	// TODO (tinyzimmer): This is sometimes failing the first reconcile attempt
	// still with an "object already modified" error. It's benign, but should dig
	// deeper at some point.
	if err = r.Client.Update(context.TODO(), instance); err != nil {
		return requeueWithError(reqLogger, "failed to update KafkaTopic", err)
	}

	// Fetch the updated object
	instance = &v1alpha1.KafkaTopic{}
	if err = r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return requeueWithError(reqLogger, "failed to retrieve updated topic instance", err)
	}

	// Do an initial topic status sync
	if _, err = r.doTopicStatusSync(reqLogger, cluster, instance); err != nil {
		return requeueWithError(reqLogger, "failed to update KafkaTopic status", err)
	}

	// Kick off a goroutine to sync topic status every 5 minutes
	uid := instance.GetUID()
	if _, ok := syncRoutines[uid]; !ok {
		reqLogger.Info("Starting status sync routine for topic")
		syncRoutines[uid] = struct{}{}
		go r.syncTopicStatus(cluster, instance, uid)
	}

	reqLogger.Info("Ensured topic")

	return reconciled()
}

func (r *KafkaTopicReconciler) ensureClusterLabel(cluster *v1beta1.KafkaCluster, topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopic, error) {
	labelValue := clusterLabelString(cluster)
	var labels map[string]string
	if labels = topic.GetLabels(); labels == nil {
		labels = make(map[string]string, 0)
	}
	if label, ok := labels[clusterRefLabel]; ok {
		if label != labelValue {
			labels[clusterRefLabel] = labelValue
		}
	} else {
		labels[clusterRefLabel] = labelValue
	}
	if !reflect.DeepEqual(labels, topic.GetLabels()) {
		topic.SetLabels(labels)
		return r.updateAndFetchLatest(topic)
	}
	return topic, nil
}

func (r *KafkaTopicReconciler) ensureControllerReference(cluster *v1beta1.KafkaCluster, topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopic, error) {
	if err := controllerutil.SetControllerReference(cluster, topic, r.Scheme); err != nil {
		if !k8sutil.IsAlreadyOwnedError(err) {
			return nil, err
		}
	} else {
		return r.updateAndFetchLatest(topic)
	}
	return topic, nil
}

func (r *KafkaTopicReconciler) updateAndFetchLatest(topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopic, error) {
	if err := r.Client.Update(context.TODO(), topic); err != nil {
		return nil, err
	}
	return r.fetchMostRecent(topic)
}

func (r *KafkaTopicReconciler) fetchMostRecent(topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopic, error) {
	updated := &v1alpha1.KafkaTopic{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: topic.Name, Namespace: topic.Namespace}, updated)
	return updated, err
}

func (r *KafkaTopicReconciler) syncTopicStatus(cluster *v1beta1.KafkaCluster, instance *v1alpha1.KafkaTopic, uid types.UID) {
	syncLogger := r.Log.WithName(fmt.Sprintf("%s/%s_sync", instance.Namespace, instance.Name))
	ticker := time.NewTicker(time.Duration(5) * time.Minute)
	for range ticker.C {
		syncLogger.Info("Syncing topic status")
		if cont, _ := r.doTopicStatusSync(syncLogger, cluster, instance); !cont {
			if _, ok := syncRoutines[uid]; ok {
				delete(syncRoutines, uid)
			}
			return
		}
	}
}

func (r *KafkaTopicReconciler) doTopicStatusSync(syncLogger logr.Logger, cluster *v1beta1.KafkaCluster, instance *v1alpha1.KafkaTopic) (bool, error) {

	// check if the topic still exists
	topic := &v1alpha1.KafkaTopic{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, topic); err != nil {
		if apierrors.IsNotFound(err) {
			syncLogger.Info("Topic has been deleted, stopping sync routine")
			// return false to stop calling goroutine
			return false, nil
		}
		return true, err
	}

	// get topic status
	status, err := r.getKafkaTopicStatus(syncLogger, cluster, topic)
	if err != nil {
		syncLogger.Error(err, "Failed to get current kafka topic status")
		return true, err
	}

	// update status
	updated := topic.DeepCopy()
	updated.Status = *status
	if err = r.Client.Status().Update(context.TODO(), updated); err != nil {
		syncLogger.Error(err, "Failed to update KafkaTopic status")
		return true, err
	}
	return true, nil
}

func (r *KafkaTopicReconciler) getKafkaTopicStatus(log logr.Logger, cluster *v1beta1.KafkaCluster, topic *v1alpha1.KafkaTopic) (*v1alpha1.KafkaTopicStatus, error) {
	// grab a connection to kafka
	k, close, err := newBrokerConnection(log, r.Client, cluster)
	if err != nil {
		return nil, err
	}
	defer close()

	// get topic metadata
	meta, err := k.DescribeTopic(topic.Spec.Name)
	if err != nil {
		return nil, err
	}

	return kafkaclient.TopicMetaToStatus(k.Brokers(), meta), nil
}

func (r *KafkaTopicReconciler) checkFinalizers(reqLogger logr.Logger, broker kafkaclient.KafkaClient, topic *v1alpha1.KafkaTopic) (reconcile.Result, error) {
	reqLogger.Info("Kafka topic is marked for deletion")
	var err error
	if util.StringSliceContains(topic.GetFinalizers(), topicFinalizer) {
		if err = r.finalizeKafkaTopic(reqLogger, broker, topic); err != nil {
			return requeueWithError(reqLogger, "failed to finalize kafkatopic", err)
		}
		if err = r.removeFinalizer(topic); err != nil {
			return requeueWithError(reqLogger, "failed to remove finalizer from kafkatopic", err)
		}
	}
	return reconciled()
}

func (r *KafkaTopicReconciler) removeFinalizer(topic *v1alpha1.KafkaTopic) error {
	topic.SetFinalizers(util.StringSliceRemove(topic.GetFinalizers(), topicFinalizer))
	return r.Client.Update(context.TODO(), topic)
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
