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

	"github.com/banzaicloud/k8s-objectmatcher/patch"

	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	certsigningreqv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/pki"
	"github.com/banzaicloud/koperator/pkg/util"
	kafkautil "github.com/banzaicloud/koperator/pkg/util/kafka"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"

	"sigs.k8s.io/controller-runtime/pkg/source"
)

var userFinalizer = "finalizer.kafkausers.kafka.banzaicloud.io"

// SetupKafkaUserWithManager registers KafkaUser controller to the manager
func SetupKafkaUserWithManager(mgr ctrl.Manager, certSigningEnabled bool, certManagerNamespace bool, log logr.Logger) *ctrl.Builder {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KafkaUser{}).Named("KafkaUser")
	if certSigningEnabled {
		builder.Watches(
			&source.Kind{Type: &certsigningreqv1.CertificateSigningRequest{}},
			handler.EnqueueRequestsFromMapFunc(certificateSigningRequestMapper),
			ctrlBuilder.WithPredicates(certificateSigningRequestFilter(log)))
	}
	if certManagerNamespace {
		builder.Owns(&certv1.Certificate{})
	}
	return builder
}

func certificateSigningRequestFilter(log logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			patchResult, err := patch.DefaultPatchMaker.Calculate(e.ObjectOld, e.ObjectNew)
			if err != nil {
				log.Error(err, "could not match objects", "kind", e.ObjectOld.GetObjectKind())
			} else if patchResult.IsEmpty() {
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
	}
}

func certificateSigningRequestMapper(obj client.Object) []ctrl.Request {
	certSigningReqAnnotations := obj.GetAnnotations()
	kafkaUserResourceNamespacedName, ok := certSigningReqAnnotations[pkicommon.KafkaUserAnnotationName]
	if !ok {
		return []ctrl.Request{}
	}
	namespaceWithName := strings.Split(kafkaUserResourceNamespacedName, string(types.Separator))
	if len(namespaceWithName) != 2 {
		return []ctrl.Request{}
	}
	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: namespaceWithName[0],
			Name:      namespaceWithName[1],
		},
	}}
}

// blank assignment to verify that KafkaUserReconciler implements reconcile.kafkaUserReconciler
var _ reconcile.Reconciler = &KafkaUserReconciler{}

// KafkaUserReconciler reconciles a KafkaUser object
type KafkaUserReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkausers,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=kafkausers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=clusterissuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a KafkaUser object and makes changes based on the state read
// and what is in the KafkaUser.Spec
func (r *KafkaUserReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("kafkauser", request.NamespacedName, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KafkaUser")
	var err error

	// Fetch the KafkaUser instance
	instance := &v1alpha1.KafkaUser{}
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
			reqLogger.Info("Cluster is gone already, there is nothing we can do")
			if err = r.removeFinalizer(ctx, instance); err != nil {
				return requeueWithError(reqLogger, "failed to remove finalizer from kafkauser", err)
			}
			return reconciled()
		}
		return requeueWithError(reqLogger, "failed to lookup referenced cluster", err)
	}

	var kafkaUser string

	if instance.Spec.GetIfCertShouldBeCreated() {
		// Avoid panic if the user wants to create a kafka user but the cluster is in plaintext mode
		// TODO: refactor this and use webhook to validate if the cluster is eligible to create a kafka user
		if cluster.Spec.ListenersConfig.SSLSecrets == nil && instance.Spec.PKIBackendSpec == nil {
			return requeueWithError(reqLogger, "could not create kafka user since user specific PKI not configured", errors.New("failed to create kafka user"))
		}
		var backend v1beta1.PKIBackend
		if instance.Spec.PKIBackendSpec != nil {
			backend = v1beta1.PKIBackend(instance.Spec.PKIBackendSpec.PKIBackend)
		} else {
			backend = v1beta1.PKIBackendProvided
		}

		pkiManager := pki.GetPKIManager(r.Client, cluster, backend, r.Log)

		// Reconcile no matter what to get a user certificate instance for ACL management
		// TODO (tinyzimmer): This can go wrong if the user made a mistake in their secret path
		// using the vault backend, then tried to delete and fix it. Should probably
		// have the PKIManager export a GetUserCertificate specifically for deletions
		// that will allow the error to fall through if the certificate doesn't exist.
		user, err := pkiManager.ReconcileUserCertificate(ctx, instance, r.Scheme, cluster.Spec.GetKubernetesClusterDomain())
		if err != nil {
			switch errors.Cause(err).(type) {
			case errorfactory.ResourceNotReady:
				reqLogger.Info("generated secret not found, may not be ready")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(5) * time.Second,
				}, nil
			case errorfactory.FatalReconcileError:
				// TODO: (tinyzimmer) - Sleep for longer for now to give user time to see the error
				// But really we should catch these kinds of issues in a pre-admission hook in a future PR
				// The user can fix while this is looping and it will pick it up next reconcile attempt
				reqLogger.Error(err, "Fatal error attempting to reconcile the user certificate. If using vault perhaps a permissions issue or improperly configured PKI?")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(15) * time.Second,
				}, nil
			case errorfactory.VaultAPIFailure:
				// Same as above in terms of things that could be checked pre-flight on the cluster
				reqLogger.Error(err, "Vault API error attempting to reconcile the user certificate. If using vault perhaps a permissions issue or improperly configured PKI?")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Duration(15) * time.Second,
				}, nil
			default:
				if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
					reqLogger.Info("Kafka user marked for deletion before creating certificates")
					if err = r.removeFinalizer(ctx, instance); err != nil {
						return requeueWithError(reqLogger, "failed to remove finalizer from kafkauser", err)
					}
					return reconciled()
				}
				return requeueWithError(reqLogger, "failed to reconcile user secret", err)
			}
		}
		kafkaUser = user.DN()
		// check if marked for deletion and remove created certs
		if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
			reqLogger.Info("Kafka user is marked for deletion, revoking certificates")
			if err = pkiManager.FinalizeUserCertificate(ctx, instance); err != nil {
				return requeueWithError(reqLogger, "failed to finalize user certificate", err)
			}
		}
	} else {
		kafkaUser = fmt.Sprintf("CN=%s", instance.Name)
	}

	// check if marked for deletion and remove kafka ACLs
	if k8sutil.IsMarkedForDeletion(instance.ObjectMeta) {
		return r.checkFinalizers(ctx, reqLogger, cluster, instance, kafkaUser)
	}

	// ensure a kafkaCluster label
	if instance, err = r.ensureClusterLabel(ctx, cluster, instance); err != nil {
		return requeueWithError(reqLogger, "failed to ensure kafkacluster label on user", err)
	}

	// If topic grants supplied, grab a broker connection and set ACLs
	if len(instance.Spec.TopicGrants) > 0 {
		broker, close, err := newKafkaFromCluster(r.Client, cluster)
		if err != nil {
			return checkBrokerConnectionError(reqLogger, err)
		}
		defer close()

		// TODO (tinyzimmer): Should probably take this opportunity to see if we are removing any ACLs
		for _, grant := range instance.Spec.TopicGrants {
			reqLogger.Info(fmt.Sprintf("Ensuring %s ACLs for User: %s -> Topic: %s", grant.AccessType, kafkaUser, grant.TopicName))
			// CreateUserACLs returns no error if the ACLs already exist
			if err = broker.CreateUserACLs(grant.AccessType, grant.PatternType, kafkaUser, grant.TopicName); err != nil {
				return requeueWithError(reqLogger, "failed to ensure ACLs for kafkauser", err)
			}
		}
	}

	// ensure a finalizer for cleanup on deletion
	if !util.StringSliceContains(instance.GetFinalizers(), userFinalizer) {
		r.addFinalizer(reqLogger, instance)
		if instance, err = r.updateAndFetchLatest(ctx, instance); err != nil {
			return requeueWithError(reqLogger, "failed to update kafkauser with finalizer", err)
		}
	}

	// set user status
	instance.Status = v1alpha1.KafkaUserStatus{
		State: v1alpha1.UserStateCreated,
	}
	if len(instance.Spec.TopicGrants) > 0 {
		instance.Status.ACLs = kafkautil.GrantsToACLStrings(kafkaUser, instance.Spec.TopicGrants)
	}
	if err := r.Client.Status().Update(ctx, instance); err != nil {
		return requeueWithError(reqLogger, "failed to update kafkauser status", err)
	}

	return reconciled()
}

func (r *KafkaUserReconciler) ensureClusterLabel(ctx context.Context, cluster *v1beta1.KafkaCluster, user *v1alpha1.KafkaUser) (*v1alpha1.KafkaUser, error) {
	labels := applyClusterRefLabel(cluster, user.GetLabels())
	if !reflect.DeepEqual(labels, user.GetLabels()) {
		user.SetLabels(labels)
		return r.updateAndFetchLatest(ctx, user)
	}
	return user, nil
}

func (r *KafkaUserReconciler) updateAndFetchLatest(ctx context.Context, user *v1alpha1.KafkaUser) (*v1alpha1.KafkaUser, error) {
	typeMeta := user.TypeMeta
	err := r.Client.Update(ctx, user)
	if err != nil {
		return nil, err
	}
	user.TypeMeta = typeMeta
	return user, nil
}

func (r *KafkaUserReconciler) checkFinalizers(ctx context.Context, reqLogger logr.Logger, cluster *v1beta1.KafkaCluster, instance *v1alpha1.KafkaUser, user string) (reconcile.Result, error) {
	// run finalizers
	var err error
	if util.StringSliceContains(instance.GetFinalizers(), userFinalizer) {
		if len(instance.Spec.TopicGrants) > 0 {
			if err = r.finalizeKafkaUserACLs(reqLogger, cluster, user); err != nil {
				return requeueWithError(reqLogger, "failed to finalize kafkauser", err)
			}
		}
		// remove finalizer
		if err = r.removeFinalizer(ctx, instance); err != nil {
			return requeueWithError(reqLogger, "failed to remove finalizer from kafkauser", err)
		}
	}
	return reconciled()
}

func (r *KafkaUserReconciler) removeFinalizer(ctx context.Context, user *v1alpha1.KafkaUser) error {
	user.SetFinalizers(util.StringSliceRemove(user.GetFinalizers(), userFinalizer))
	_, err := r.updateAndFetchLatest(ctx, user)
	return err
}

func (r *KafkaUserReconciler) finalizeKafkaUserACLs(reqLogger logr.Logger, cluster *v1beta1.KafkaCluster, user string) error {
	if k8sutil.IsMarkedForDeletion(cluster.ObjectMeta) {
		reqLogger.Info("Cluster is being deleted, skipping ACL deletion")
		return nil
	}
	var err error
	reqLogger.Info("Deleting user ACLs from kafka")
	broker, close, err := newKafkaFromCluster(r.Client, cluster)
	if err != nil {
		return err
	}
	defer close()
	if err = broker.DeleteUserACLs(user); err != nil {
		return err
	}
	return nil
}

func (r *KafkaUserReconciler) addFinalizer(reqLogger logr.Logger, user *v1alpha1.KafkaUser) {
	reqLogger.Info("Adding Finalizer for the KafkaUser")
	user.SetFinalizers(append(user.GetFinalizers(), userFinalizer))
}
