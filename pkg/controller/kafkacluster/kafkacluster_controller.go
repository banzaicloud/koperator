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
	"fmt"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/kafka"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
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

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by KafkaCluster - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &banzaicloudv1alpha1.KafkaCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &banzaicloudv1alpha1.KafkaCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &banzaicloudv1alpha1.KafkaCluster{},
	})
	if err != nil {
		return err
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
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkaclusters/status,verbs=get;update;patch
func (r *ReconcileKafkaCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	lBService := envoy.LoadBalancerForEnvoy(instance)
	if err := controllerutil.SetControllerReference(instance, lBService, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundLBService := &corev1.Service{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: lBService.Name, Namespace: lBService.Namespace}, foundLBService)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating LoadBalancerService", "namespace", lBService.Namespace, "name", lBService.Name)
		err = r.Create(context.TODO(), lBService)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if len(foundLBService.Status.LoadBalancer.Ingress) == 0 {
		return reconcile.Result{}, fmt.Errorf("loadbalancer is not created waiting")
	}

	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" && foundLBService.Status.LoadBalancer.Ingress[0].IP == "" {
		time.Sleep(20 * time.Second)
		return reconcile.Result{}, fmt.Errorf("loadbalancer is not ready waiting")
	}
	var loadBalancerExternalAddress string
	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" {
		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].IP
	} else {
		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].Hostname
	}

	eDeployment := envoy.DeploymentForEnvoy(instance)
	if err := controllerutil.SetControllerReference(instance, eDeployment, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundEDeployment := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: eDeployment.Name, Namespace: eDeployment.Namespace}, foundEDeployment)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Deployment", "namespace", eDeployment.Namespace, "name", eDeployment.Name)
		err = r.Create(context.TODO(), eDeployment)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	bConfigMap := kafka.ConfigMapForKafka(instance)
	if err := controllerutil.SetControllerReference(instance, bConfigMap, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundBConfigMap := &corev1.ConfigMap{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: bConfigMap.Name, Namespace: bConfigMap.Namespace}, foundBConfigMap)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating ConfigMap for Brokers", "namespace", bConfigMap.Namespace, "name", bConfigMap.Name)
		err = r.Create(context.TODO(), bConfigMap)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(bConfigMap.Data, foundBConfigMap.Data) {
		foundBConfigMap.Data = bConfigMap.Data
		log.Info("Updating ConfigMap", "namespace", bConfigMap.Namespace, "name", bConfigMap.Name)
		err = r.Update(context.TODO(), foundBConfigMap)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	hService := kafka.HeadlessServiceForKafka(instance)
	if err := controllerutil.SetControllerReference(instance, hService, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundHService := &corev1.Service{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: hService.Name, Namespace: hService.Namespace}, foundHService)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating HeadlessService", "namespace", hService.Namespace, "name", hService.Name)
		err = r.Create(context.TODO(), hService)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	sSet := kafka.StatefulSetForKafka(instance, loadBalancerExternalAddress)
	if err := controllerutil.SetControllerReference(instance, sSet, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundSSet := &appsv1.StatefulSet{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: sSet.Name, Namespace: sSet.Namespace}, foundSSet)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating StatefulSet", "namespace", sSet.Namespace, "name", sSet.Name)
		err = r.Create(context.TODO(), sSet)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	//if !reflect.DeepEqual(sSet.Spec, foundSSet.Spec) {
	//	foundSSet.Spec = sSet.Spec
	//	log.Info("Updating StatefulSet", "namespace", sSet.Namespace, "name", sSet.Name)
	//	err = r.Update(context.TODO(), foundSSet)
	//	if err != nil {
	//		return reconcile.Result{}, err
	//	}
	//}

	return reconcile.Result{}, nil
}
