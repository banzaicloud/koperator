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
	"errors"
	"reflect"

	"emperror.dev/emperror"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
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
			return emperror.With(err, "kind", desiredType)
		}
		log = log.WithValues("kind", desiredType, "name", key.Name)

		err = client.Get(context.TODO(), key, current)
		if err != nil && !apierrors.IsNotFound(err) {
			return emperror.WrapWith(err, "getting resource failed", "kind", desiredType, "name", key.Name)
		}
		if apierrors.IsNotFound(err) {
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return emperror.WrapWith(err, "creating resource failed", "kind", desiredType, "name", key.Name)
			}
			log.Info("resource created")
			return nil
		}
		// TODO check if this ClusterIssuer part here is necessary or can be handled in default (baluchicken)
	case *certv1.ClusterIssuer:
		var key runtimeClient.ObjectKey
		key, err = runtimeClient.ObjectKeyFromObject(current)
		if err != nil {
			return emperror.With(err, "kind", desiredType)
		}
		err = client.Get(context.TODO(), types.NamespacedName{Namespace: metav1.NamespaceAll, Name: key.Name}, current)
		if err != nil && !apierrors.IsNotFound(err) {
			return emperror.WrapWith(err, "getting resource failed", "kind", desiredType, "name", key.Name)
		}
		if apierrors.IsNotFound(err) {
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return emperror.WrapWith(err, "creating resource failed", "kind", desiredType, "name", key.Name)
			}
			log.Info("resource created")
			return nil
		}
	case *corev1.PersistentVolumeClaim:
		log = log.WithValues("kind", desiredType)
		log.V(1).Info("searching with label because name is empty")

		pvcList := &corev1.PersistentVolumeClaimList{}
		matchingLabels := runtimeClient.MatchingLabels{
			"kafka_cr": cr.Name,
			"brokerId": desired.(*corev1.PersistentVolumeClaim).Labels["brokerId"],
		}
		err = client.List(context.TODO(), pvcList,
			runtimeClient.InNamespace(current.(*corev1.PersistentVolumeClaim).Namespace), matchingLabels)
		if err != nil && len(pvcList.Items) == 0 {
			return emperror.WrapWith(err, "getting resource failed", "kind", desiredType)
		}
		mountPath := current.(*corev1.PersistentVolumeClaim).Annotations["mountPath"]

		// Creating the first PersistentVolume For Pod
		if len(pvcList.Items) == 0 {
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return emperror.WrapWith(err, "creating resource failed", "kind", desiredType)
			}
			log.Info("resource created")
			return nil
		}
		alreadyCreated := false
		for _, pvc := range pvcList.Items {
			if mountPath == pvc.Annotations["mountPath"] {
				current = pvc.DeepCopyObject()
				alreadyCreated = true
				break
			}
		}
		if !alreadyCreated {
			// Creating the 2+ PersistentVolumes for Pod
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return emperror.WrapWith(err, "creating resource failed", "kind", desiredType)
			}
			return nil
		}
	case *corev1.Pod:
		log = log.WithValues("kind", desiredType)
		log.V(1).Info("searching with label because name is empty")

		podList := &corev1.PodList{}
		matchingLabels := runtimeClient.MatchingLabels{
			"kafka_cr": cr.Name,
			"brokerId": desired.(*corev1.Pod).Labels["brokerId"],
		}
		err = client.List(context.TODO(), podList, runtimeClient.InNamespace(current.(*corev1.Pod).Namespace), matchingLabels)
		if err != nil && len(podList.Items) == 0 {
			return emperror.WrapWith(err, "getting resource failed", "kind", desiredType)
		}
		if len(podList.Items) == 0 {
			patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)
			if err := client.Create(context.TODO(), desired); err != nil {
				return emperror.WrapWith(err, "creating resource failed", "kind", desiredType)
			}
			// Update status to Config InSync because broker is configured to go
			statusErr := updateBrokerStatus(client, desired.(*corev1.Pod).Labels["brokerId"], cr, banzaicloudv1alpha1.ConfigInSync, log)
			if statusErr != nil {
				return emperror.WrapWith(err, "updating status for resource failed", "kind", desiredType)
			}
			log.Info("resource created")
			return nil
		} else if len(podList.Items) == 1 {
			current = podList.Items[0].DeepCopyObject()
			brokerId := current.(*corev1.Pod).Labels["brokerId"]
			if brokerState, ok := cr.Status.BrokersState[brokerId]; ok {
				if cr.Spec.RackAwareness != nil && (brokerState.RackAwarenessState == banzaicloudv1alpha1.WaitingForRackAwareness || brokerState.RackAwarenessState == "") {
					err := updateCrWithRackAwarenessConfig(current.(*corev1.Pod), cr, client)
					if err != nil {
						return emperror.Wrap(err, "updating cr with rack awareness info failed")
					}
					statusErr := updateBrokerStatus(client, brokerId, cr, banzaicloudv1alpha1.Configured, log)
					if statusErr != nil {
						return emperror.WrapWith(err, "updating status for resource failed", "kind", desiredType)
					}
				}
				if current.(*corev1.Pod).Status.Phase == corev1.PodRunning && brokerState.GracefulActionState.CruiseControlState == banzaicloudv1alpha1.GracefulUpdateRequired {
					scaleErr := scale.UpScaleCluster(desired.(*corev1.Pod).Labels["brokerId"], desired.(*corev1.Pod).Namespace, cr.Spec.CruiseControlConfig.CruiseControlEndpoint, cr.Name)
					if scaleErr != nil {
						log.Error(err, "graceful upscale failed, or cluster just started")
						statusErr := updateBrokerStatus(client, brokerId, cr,
							banzaicloudv1alpha1.GracefulActionState{ErrorMessage: scaleErr.Error(), CruiseControlState: banzaicloudv1alpha1.GracefulUpdateFailed}, log)
						if statusErr != nil {
							return emperror.Wrap(statusErr, "could not update broker graceful action state")
						}
					} else {
						statusErr := updateBrokerStatus(client, brokerId, cr,
							banzaicloudv1alpha1.GracefulActionState{ErrorMessage: "", CruiseControlState: banzaicloudv1alpha1.GracefulUpdateSucceeded}, log)
						if statusErr != nil {
							return emperror.Wrap(statusErr, "could not update broker graceful action state")
						}
					}
				}
			} else {
				statusErr := updateBrokerStatus(client, brokerId, cr,
					banzaicloudv1alpha1.GracefulActionState{ErrorMessage: "", CruiseControlState: banzaicloudv1alpha1.GracefulUpdateRequired}, log)
				if statusErr != nil {
					return emperror.Wrap(statusErr, "could not update broker graceful action state")
				}
				if cr.Spec.RackAwareness != nil {
					statusErr := updateBrokerStatus(client, brokerId, cr, banzaicloudv1alpha1.WaitingForRackAwareness, log)
					if statusErr != nil {
						return emperror.Wrap(statusErr, "could not update broker rack state")
					}
				}
			}

		} else {
			return emperror.WrapWith(errors.New("reconcile failed"), "more then one matching pod found", "labels", matchingLabels)
		}
	}
	if err == nil {
		patchResult, err := patch.DefaultPatchMaker.Calculate(current, desired)
		if err != nil {
			log.Error(err, "could not match objects", "kind", desiredType)
		} else if patchResult.IsEmpty() {
			switch current.(type) {
			case *corev1.Pod:
				{
					if current.(*corev1.Pod).Status.Phase != corev1.PodFailed && cr.Status.BrokersState[current.(*corev1.Pod).Labels["brokerId"]].ConfigurationState == banzaicloudv1alpha1.ConfigInSync {
						log.V(1).Info("resource is in sync")
						return nil
					}
				}
			default:
				log.V(1).Info("resource is in sync")
				return nil
			}
		} else {
			log.V(1).Info("resource diffs",
				"patch", string(patchResult.Patch),
				"current", string(patchResult.Current),
				"modified", string(patchResult.Modified),
				"original", string(patchResult.Original))
		}

		patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)

		switch desired.(type) {
		default:
			return emperror.With(errors.New("unexpected resource type"), "kind", desiredType)
		case *certv1.ClusterIssuer:
			cm := desired.(*certv1.ClusterIssuer)
			cm.ResourceVersion = current.(*certv1.ClusterIssuer).ResourceVersion
			desired = cm
		case *certv1.Issuer:
			cm := desired.(*certv1.Issuer)
			cm.ResourceVersion = current.(*certv1.Issuer).ResourceVersion
			desired = cm
		case *certv1.Certificate:
			cm := desired.(*certv1.Certificate)
			cm.ResourceVersion = current.(*certv1.Certificate).ResourceVersion
			desired = cm
		case *corev1.ConfigMap:
			cm := desired.(*corev1.ConfigMap)
			cm.ResourceVersion = current.(*corev1.ConfigMap).ResourceVersion
			desired = cm
		case *corev1.Secret:
			cm := desired.(*corev1.Secret)
			cm.ResourceVersion = current.(*corev1.Secret).ResourceVersion
			desired = cm
		case *corev1.Service:
			svc := desired.(*corev1.Service)
			svc.ResourceVersion = current.(*corev1.Service).ResourceVersion
			svc.Spec.ClusterIP = current.(*corev1.Service).Spec.ClusterIP
			desired = svc
		case *corev1.Pod:
			err := updateCrWithNodeAffinity(current.(*corev1.Pod), cr, client)
			if err != nil {
				return emperror.WrapWith(err, "updating cr failed")
			}
			err = client.Delete(context.TODO(), current)
			if err != nil {
				return emperror.WrapWith(err, "deleting resource failed", "kind", desiredType)
			}
			return nil

		case *corev1.PersistentVolumeClaim:
			//TODO
			desired = current
		case *appsv1.Deployment:
			deploy := desired.(*appsv1.Deployment)
			deploy.ResourceVersion = current.(*appsv1.Deployment).ResourceVersion
			desired = deploy
		case *appsv1.StatefulSet:
			deploy := desired.(*appsv1.StatefulSet)
			deploy.ResourceVersion = current.(*appsv1.StatefulSet).ResourceVersion
			desired = deploy
		}

		if err := client.Update(context.TODO(), desired); err != nil {
			return emperror.WrapWith(err, "updating resource failed", "kind", desiredType)
		}
		switch desired.(type) {
		case *corev1.ConfigMap:
			// Only update status when configmap belongs to broker
			if id, ok := desired.(*corev1.ConfigMap).Labels["brokerId"]; ok {
				statusErr := updateBrokerStatus(client, id, cr, banzaicloudv1alpha1.ConfigOutOfSync, log)
				if statusErr != nil {
					return emperror.WrapWith(err, "updating status for resource failed", "kind", desiredType)
				}
			}
		}
		log.Info("resource updated")
	}
	return nil
}
