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

package k8sutil

import (
	"context"
	"reflect"

	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconcile reconciles K8S resources
func Reconcile(log logr.Logger, client runtimeClient.Client, desired runtime.Object, cr *banzaicloudv1alpha1.KafkaCluster) error {
	desiredType := reflect.TypeOf(desired)
	var current = desired.DeepCopyObject()
	var err error

	switch desired.(type) {
	default:
		var key runtimeClient.ObjectKey
		key, err = runtimeClient.ObjectKeyFromObject(current)
		if err != nil {
			return errors.WithDetails(err, "kind", desiredType)
		}
		log = log.WithValues("kind", desiredType, "name", key.Name)

		err = client.Get(context.TODO(), key, current)
		if err != nil && !apierrors.IsNotFound(err) {
			return errorfactory.New(
				errorfactory.APIFailure{},
				err,
				"getting resource failed",
				"kind", desiredType, "name", key.Name,
			)
		}
		if apierrors.IsNotFound(err) {
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return errorfactory.New(
					errorfactory.APIFailure{},
					err,
					"creating resource failed",
					"kind", desiredType, "name", key.Name,
				)
			}
			log.Info("resource created")
			return nil
		}
		// TODO check if this ClusterIssuer part here is necessary or can be handled in default (baluchicken)
	case *certv1.ClusterIssuer:
		var key runtimeClient.ObjectKey
		key, err = runtimeClient.ObjectKeyFromObject(current)
		if err != nil {
			return errors.WithDetails(err, "kind", desiredType)
		}
		err = client.Get(context.TODO(), types.NamespacedName{Namespace: metav1.NamespaceAll, Name: key.Name}, current)
		if err != nil && !apierrors.IsNotFound(err) {
			return errorfactory.New(
				errorfactory.APIFailure{},
				err,
				"getting resource failed",
				"kind", desiredType, "name", key.Name,
			)
		}
		if apierrors.IsNotFound(err) {
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return errorfactory.New(
					errorfactory.APIFailure{},
					err,
					"creating resource failed",
					"kind", desiredType, "name", key.Name,
				)
			}
			log.Info("resource created")
			return nil
		}
	}
	if err == nil {
		if CheckIfObjectUpdated(log, desiredType, current, desired) {

			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)

			switch d := desired.(type) {
			default:
				d.(metav1.ObjectMetaAccessor).GetObjectMeta().SetResourceVersion(current.(metav1.ObjectMetaAccessor).GetObjectMeta().GetResourceVersion())
			}

			if err := client.Update(context.TODO(), desired); err != nil {
				return errorfactory.New(errorfactory.APIFailure{}, err, "updating resource failed", "kind", desiredType)
			}
			switch desired.(type) {
			case *corev1.ConfigMap:
				// Only update status when configmap belongs to broker
				if id, ok := desired.(*corev1.ConfigMap).Labels["brokerId"]; ok {
					statusErr := UpdateBrokerStatus(client, id, cr, banzaicloudv1alpha1.ConfigOutOfSync, log)
					if statusErr != nil {
						return errors.WrapIfWithDetails(err, "updating status for resource failed", "kind", desiredType)
					}
				}
			}
			log.Info("resource updated")
		}
	}
	return nil
}

func CheckIfObjectUpdated(log logr.Logger, desiredType reflect.Type, current, desired runtime.Object) bool {
	patchResult, err := patch.DefaultPatchMaker.Calculate(current, desired)
	if err != nil {
		log.Error(err, "could not match objects", "kind", desiredType)
		return true
	} else if patchResult.IsEmpty() {
		log.V(1).Info("resource is in sync")
		return false
	} else {
		log.V(1).Info("resource diffs",
			"patch", string(patchResult.Patch),
			"current", string(patchResult.Current),
			"modified", string(patchResult.Modified),
			"original", string(patchResult.Original))
		return true
	}
}
