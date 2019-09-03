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

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/certutil"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/kafkautil"

	logr "github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var userFinalizer = "finalizer.kafkausers.banzaicloud.banzaicloud.io"

func SetupKafkaUserWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	r := &KafkaUserReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("KafkaUser"),
	}

	// Create a new controller
	c, err := controller.New("kafkauser", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KafkaUser
	err = c.Watch(&source.Kind{Type: &v1alpha1.KafkaUser{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary certificates and requeue the owner KafkaUser
	err = c.Watch(&source.Kind{Type: &certv1.Certificate{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.KafkaUser{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that KafkaUserReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &KafkaUserReconciler{}

// KafkaUserReconciler reconciles a KafkaUser object
type KafkaUserReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkausers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banzaicloud.banzaicloud.io,resources=kafkausers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=clusterissuers,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a KafkaUser object and makes changes based on the state read
// and what is in the KafkaUser.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *KafkaUserReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KafkaUser")
	var err error
	// Fetch the KafkaUser instance
	instance := &v1alpha1.KafkaUser{}
	if err = r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get the referenced kafkacluster
	if instance.Spec.ClusterRef.Namespace == "" {
		instance.Spec.ClusterRef.Namespace = instance.Namespace
	}
	var cluster *v1alpha1.KafkaCluster
	if cluster, err = k8sutil.LookupKafkaCluster(r.Client, instance.Spec.ClusterRef); err != nil {
		reqLogger.Error(err, "Failed to lookup referenced cluster")
		return reconcile.Result{}, err
	}

	// Get a kafka connection
	reqLogger.Info("Retrieving kafka admin client")
	broker, err := kafkautil.NewFromCluster(r.Client, cluster)
	if err != nil {
		reqLogger.Error(err, "Error connecting to kafka")
		return reconcile.Result{}, err
	}
	defer broker.Close()

	// check if marked for deletion
	isMarkedForDeletion := instance.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		return r.checkFinalizers(reqLogger, broker, instance)
	}

	// See if we have an existing certificate for this user already
	_, err = r.getUserCertificate(instance)
	if err != nil && errors.IsNotFound(err) {
		// the certificate does not exist, let's make one
		cert := r.clusterCertificateForUser(broker, instance)
		reqLogger.Info("Creating new certificate for user")
		if err = r.Client.Create(context.TODO(), cert); err != nil {
			reqLogger.Error(err, "Failed to create certificate for user")
			return reconcile.Result{}, err
		}

	} else if err != nil {
		// API failure, requeue
		reqLogger.Error(err, "Failed to get certificate")
		return reconcile.Result{}, err

	} else {
		// certificate exists
		reqLogger.Info("User certificate already exists, verifying finalizers and grants")
	}

	// do topic grants

	// get the user's distinguished name for kafka acls
	userName, secret, err := r.getUserX509NameAndCredentials(reqLogger, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Spec.IncludeJKS {
		reqLogger.Info("Injecting JKS format into user secret")
		if secret, err = certutil.InjectJKS(reqLogger, secret); err != nil {
			return reconcile.Result{}, err
		}
		if err = r.Client.Update(context.TODO(), secret); err != nil {
			return reconcile.Result{}, err
		}
	}

	// ensure ACLs - CreateUserACLs returns no error if the ACLs already exist
	// TODO: Should probably take this opportunity to see if we are removing any ACLs
	for _, grant := range instance.Spec.TopicGrants {
		reqLogger.Info(fmt.Sprintf("Ensuring %s ACLs for User: %s -> Topic: %s", grant.AccessType, userName, grant.TopicName))
		if err = broker.CreateUserACLs(grant.AccessType, userName, grant.TopicName); err != nil {
			return reconcile.Result{}, err
		}
	}

	// ensure a finalizer for cleanup on deletion
	if !contains(instance.GetFinalizers(), userFinalizer) {
		if err = r.addFinalizer(reqLogger, instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *KafkaUserReconciler) clusterCertificateForUser(broker kafkautil.KafkaClient, user *v1alpha1.KafkaUser) *certv1.Certificate {
	caName, caKind := broker.GetCA()
	cert := &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: user.Namespace,
		},
		Spec: certv1.CertificateSpec{
			SecretName:  user.Spec.SecretName,
			KeyEncoding: certv1.PKCS8,
			CommonName:  user.Name,
			IssuerRef: certv1.ObjectReference{
				Name: caName,
				Kind: caKind,
			},
		},
	}
	controllerutil.SetControllerReference(user, cert, r.Scheme)
	return cert
}

func (r *KafkaUserReconciler) checkFinalizers(reqLogger logr.Logger, broker kafkautil.KafkaClient, user *v1alpha1.KafkaUser) (reconcile.Result, error) {
	// run finalizers
	reqLogger.Info("Kafka user is marked for deletion")
	if contains(user.GetFinalizers(), userFinalizer) {
		if err := r.finalizeKafkaUser(reqLogger, broker, user); err != nil {
			reqLogger.Error(err, "Failed to finalize kafka user")
			return reconcile.Result{}, err
		}
		// remove finalizer
		user.SetFinalizers(remove(user.GetFinalizers(), userFinalizer))
		if err := r.Client.Update(context.TODO(), user); err != nil {
			reqLogger.Error(err, "Failed to update finalizers for kafka user")
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *KafkaUserReconciler) finalizeKafkaUser(reqLogger logr.Logger, broker kafkautil.KafkaClient, user *v1alpha1.KafkaUser) error {
	var err error

	// get the user's distinguished name to delete matching kafka acls
	userName, secret, err := r.getUserX509NameAndCredentials(reqLogger, user)
	if err != nil {
		return err
	}
	reqLogger.Info("Deleting user ACLs from kafka")
	if err = broker.DeleteUserACLs(userName); err != nil {
		return err
	}

	// cleanup certificate
	cert, err := r.getUserCertificate(user)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		reqLogger.Info("Deleting certificate for user")
		err = r.Client.Delete(context.TODO(), cert)
		if err != nil {
			return err
		}
	}

	// cleanup the secret
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		reqLogger.Info("Deleting secret for user")
		err = r.Client.Delete(context.TODO(), secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KafkaUserReconciler) addFinalizer(reqLogger logr.Logger, user *v1alpha1.KafkaUser) error {
	reqLogger.Info("Adding Finalizer for the KafkaUser")
	user.SetFinalizers(append(user.GetFinalizers(), userFinalizer))

	// Update CR
	err := r.Client.Update(context.TODO(), user)
	if err != nil {
		reqLogger.Error(err, "Failed to update KafkaUser with finalizer")
		return err
	}
	return nil
}

func (r *KafkaUserReconciler) getUserX509NameAndCredentials(reqLogger logr.Logger, user *v1alpha1.KafkaUser) (dn string, secret *corev1.Secret, err error) {
	// retrieve user secret to get common name
	secret, err = r.getUserSecret(user)
	if err != nil {
		reqLogger.Error(err, "Failed to lookup client secret")
		return
	}

	certData, err := certutil.DecodeCertificate(secret.Data[corev1.TLSCertKey])
	if err != nil {
		reqLogger.Error(err, "Failed decoding client certificate")
		return
	}

	dn = certData.Subject.String()
	return
}

func (r *KafkaUserReconciler) getUserSecret(user *v1alpha1.KafkaUser) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: user.Spec.SecretName, Namespace: user.Namespace}, secret)
	return secret, err
}

func (r *KafkaUserReconciler) getUserCertificate(user *v1alpha1.KafkaUser) (*certv1.Certificate, error) {
	cert := &certv1.Certificate{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, cert)
	return cert, err
}
