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
	"time"

	"emperror.dev/errors"
	"github.com/Shopify/sarama"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrol"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrolmonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/externalaccess"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafkamonitoring"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
)

// KafkaClusterReconciler reconciles a KafkaCluster object
type KafkaClusterReconciler struct {
	client.Client
	Log logr.Logger
}

// Reconcile reads that state of the cluster for a KafkaCluster object and makes changes based on the state read
// and what is in the KafkaCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
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
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	reconcilers := []resources.ComponentReconciler{
		externalaccess.New(r.Client, instance),
		kafkamonitoring.New(r.Client, instance),
		cruisecontrolmonitoring.New(r.Client, instance),
		kafka.New(r.Client, instance),
		cruisecontrol.New(r.Client, instance),
	}

	for _, rec := range reconcilers {
		err = rec.Reconcile(log)
		if err != nil {
			var tError *sarama.TopicError
			if errors.Is(err, sarama.ErrOutOfBrokers) || errors.As(err, &tError) {
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(20) * time.Second,
				}, nil
			}
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
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
