/*
Copyright 2019 Banzai Cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kafkacluster

import (
	"context"

	objectmatch "github.com/banzaicloud/k8s-objectmatcher"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrol"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/monitoring"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")

// Add creates a new KafkaCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKafkaCluster{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("kafkacluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to KafkaCluster
	err = c.Watch(&source.Kind{Type: &banzaicloudv1alpha1.KafkaCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Initialize object matcher
	//objectMatcher := objectmatch.New(logf.NewDelegatingLogger(logf.NullLogger{}))
	objectMatcher := objectmatch.New(log)

	// Initialize owner matcher
	ownerMatcher := k8sutil.NewOwnerReferenceMatcher(&banzaicloudv1alpha1.KafkaCluster{}, true,  mgr.GetScheme())

	// Watch for changes to resources managed by the operator
	for _, t := range []runtime.Object{
		&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}},
		&corev1.ConfigMap{TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"}},
		&corev1.PersistentVolumeClaim{TypeMeta: metav1.TypeMeta{Kind: "PersistentVolumeClaim", APIVersion: "v1"}},
	} {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &banzaicloudv1alpha1.KafkaCluster{},
		}, predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				related, object, err := ownerMatcher.Match(e.Object)
				if err != nil {
					log.Error(err, "could not determine relation", "kind", e.Object.GetObjectKind())
				}
				if related {
					log.Info("related object deleted", "trigger", object.GetName())
					return true
				}
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				related, object, err := ownerMatcher.Match(e.ObjectNew)
				if err != nil {
					log.Error(err, "could not determine relation", "kind", e.ObjectNew.GetObjectKind())
				}
				if related {
					log.Info("related object changed", "trigger", object.GetName())
					objectsEquals, err := objectMatcher.Match(e.ObjectOld, e.ObjectNew)
					if err != nil {
						log.Error(err, "could not match objects", "kind", e.ObjectOld.GetObjectKind())
					} else if objectsEquals {
						return false
					}
					return true
				}
				return false
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var _ reconcile.Reconciler = &ReconcileKafkaCluster{}

// ReconcileKafkaCluster reconciles a KafkaCluster object
type ReconcileKafkaCluster struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a KafkaCluster object and makes changes based on the state read
// and what is in the KafkaCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch
func (r *ReconcileKafkaCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KafkaCluster")
	// Fetch the KafkaCluster instance
	instance := &banzaicloudv1alpha1.KafkaCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	reconcilers := []resources.ComponentReconciler{
		//envoy.New(r.Client, instance),
		monitoring.New(r.Client, instance),
		kafka.New(r.Client, instance),
		cruisecontrol.New(r.Client, instance),
		//restproxy.New(r.Client, instance),
	}

	for _, rec := range reconcilers {
		err = rec.Reconcile(reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

//func (r *ReconcileKafkaCluster) updateStatus(cluster *banzaicloudv1alpha1.KafkaCluster, status banzaicloudv1alpha1.BrokerState, brokerId int32, logger logr.Logger) error {
//	typeMeta := cluster.TypeMeta
//	cluster.Status.BrokersState[brokerId] = status
//	err := r.Status().Update(context.Background(), cluster)
//	if k8serrors.IsNotFound(err) {
//		err = r.Update(context.Background(), cluster)
//	}
//	if err != nil {
//		if !k8serrors.IsConflict(err) {
//			return emperror.Wrapf(err, "could not update broker %d state to '%s'", brokerId, status)
//		}
//		err := r.Get(context.TODO(), types.NamespacedName{
//			Namespace: cluster.Namespace,
//			Name:      cluster.Name,
//		}, cluster)
//		if err != nil {
//			return emperror.Wrap(err, "could not get config for updating status")
//		}
//		cluster.Status.BrokersState[brokerId] = status
//		err = r.Status().Update(context.Background(), cluster)
//		if k8serrors.IsNotFound(err) {
//			err = r.Update(context.Background(), cluster)
//		}
//		if err != nil {
//			return emperror.Wrapf(err, "could not update Broker state to '%s'", status)
//		}
//	}
//	// update loses the typeMeta of the config that's used later when setting ownerrefs
//	cluster.TypeMeta = typeMeta
//	logger.Info("Broker state updated", "status", status)
//	return nil
//}
