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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
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

	lBService := loadBalancerForKafka(instance)
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

	bConfigMap := configMapForKafka(instance)
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

	hService := headlessServiceForKafka(instance)
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

	sSet := statefulSetForKafka(instance)
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

	if !reflect.DeepEqual(sSet.Spec, foundSSet.Spec) {
		foundSSet.Spec = sSet.Spec
		log.Info("Updating StatefulSet", "namespace", sSet.Namespace, "name", sSet.Name)
		err = r.Update(context.TODO(), foundSSet)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// labelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}

// headlessServiceForKafka return a HeadLess service for Kafka
func headlessServiceForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.Service {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-headless", kc.Name),
			Namespace: kc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector:  labelsForKafka(kc.Name),
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Name: "broker",
					Port: 9092,
				},
			},
		},
	}
	return service
}

func loadBalancerForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.Service {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-loadbalancer", kc.Name),
			Namespace: kc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labelsForKafka(kc.Name),
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name: "broker",
					Port: 9093,
				},
			},
		},
	}
	return service
}

func configMapForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", kc.Name),
			Namespace: kc.Namespace,
			Labels:    labelsForKafka(kc.Name),
		},
		Data: kc.Spec.BrokerConfig.Config,
	}
	return configMap
}

// statefulSetForKafka returns a Kafka StatefulSet object
func statefulSetForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *appsv1.StatefulSet {
	ls := labelsForKafka(kc.Name)
	replicas := kc.Spec.Brokers

	volumes := []corev1.Volume{
		{
			Name: "broker-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: kc.Name + "-config"},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "broker-config",
			MountPath: "/kafka/config",
		},
	}

	//owner := asOwner(v)
	//ownerJSON, err := json.Marshal(owner)
	//if err != nil {
	//	return nil, err
	//}

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kc.Name,
			Namespace: kc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kafka-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(kc.Spec.StorageSize),
							},
						},
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      ls,
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: kc.Spec.GetServiceAccount(),
					Containers: []corev1.Container{
						{
							Image:           kc.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "kafka",
							Args:            []string{""},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9092,
									Name:          "broker-port",
								},
							},
							//Env: {},
							//LivenessProbe: &corev1.Probe{
							//	Handler: corev1.Handler{
							//		HTTPGet: &corev1.HTTPGetAction{
							//			Port:   intstr.FromString("api-port"),
							//			Path:   "/v1/sys/init",
							//		}},
							//},
							//ReadinessProbe: &corev1.Probe{
							//	Handler: corev1.Handler{
							//		HTTPGet: &corev1.HTTPGetAction{
							//			Port:   intstr.FromString("api-port"),
							//			Path:   "/v1/sys/health",
							//		}},
							//	PeriodSeconds:    5,
							//	FailureThreshold: 2,
							//},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	return statefulSet
}
