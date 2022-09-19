// Copyright © 2022 Banzai Cloud
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

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	banzaiv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
)

const (
	defaultRequeueCruiseControlOperatonTTL = 30
)

// CruiseControlOperationTTLReconciler reconciles CruiseControlOperation custom resources
type CruiseControlOperationTTLReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperations,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=kafka.banzaicloud.io,resources=cruisecontroloperations/status,verbs=get

//nolint:gocyclo
func (r *CruiseControlOperationTTLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("reconciling CruiseControlOperation custom resources")

	ccOperation := &banzaiv1alpha1.CruiseControlOperation{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, ccOperation); err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconciled()
		}
		// Error reading the object - requeue the request.
		return requeueWithError(log, err.Error(), err)
	}

	if ccOperation.GetTTLSecondsAfterFinished() == nil || ccOperation.GetCurrentTask().Finished == nil {
		return reconciled()
	}

	operationTTL := time.Duration(*ccOperation.GetTTLSecondsAfterFinished() * int(time.Second))
	finishedAt := ccOperation.GetCurrentTask().Finished

	if IsExpired(operationTTL, finishedAt.Time) {
		log.Info("cleaning up finished CruiseControlOperation", "name", ccOperation.GetName(), "namespace", ccOperation.GetNamespace(), "finished", finishedAt.Time, "ttl+now", finishedAt.Time.Add(operationTTL))
		return r.delete(ctx, ccOperation)
	} else {
		return requeueAfter(int(finishedAt.Time.Add(operationTTL).Sub(time.Now())))
	}
}

// SetupCruiseControlWithManager registers cruise control controller to the manager
func SetupCruiseControlOperationTTLWithManager(mgr ctrl.Manager) *ctrl.Builder {
	cruiseControlOperationTTLPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newObj := e.ObjectNew.(*banzaiv1alpha1.CruiseControlOperation)
			if newObj.IsFinished() && newObj.GetTTLSecondsAfterFinished() != nil && newObj.GetDeletionTimestamp().IsZero() {
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&banzaiv1alpha1.CruiseControlOperation{}).
		WithEventFilter(SkipClusterRegistryOwnedResourcePredicate{}).
		WithEventFilter(cruiseControlOperationTTLPredicate).
		Named("CruiseControlOperationTTL")

	return builder
}

func IsExpired(ttl time.Duration, finishedAt time.Time) bool {
	return finishedAt.Add(ttl).Before(time.Now())
}

func (r *CruiseControlOperationTTLReconciler) delete(ctx context.Context, ccOperation *v1alpha1.CruiseControlOperation) (reconcile.Result, error) {
	log := logr.FromContextOrDiscard(ctx)
	err := r.Delete(ctx, ccOperation)
	if err != nil && !apierrors.IsNotFound(err) {
		return requeueWithError(log, "error is occured when deleting finished CruiseControlOperation ", err)
	}

	return reconcile.Result{}, nil
}
