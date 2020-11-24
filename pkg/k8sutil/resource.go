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
	"strings"

	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	banzaicloudv1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconcile reconciles K8S resources
func Reconcile(log logr.Logger, client runtimeClient.Client, desired runtime.Object, cr *v1beta1.KafkaCluster) error {
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
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desired); err != nil {
				return errors.WrapIf(err, "could not apply last state to annotation")
			}
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
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desired); err != nil {
				return errors.WrapIf(err, "could not apply last state to annotation")
			}
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

			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desired); err != nil {
				return errors.WrapIf(err, "could not apply last state to annotation")
			}

			switch d := desired.(type) {
			default:
				d.(metav1.ObjectMetaAccessor).GetObjectMeta().SetResourceVersion(current.(metav1.ObjectMetaAccessor).GetObjectMeta().GetResourceVersion())
			case *corev1.Service:
				svc := desired.(*corev1.Service)
				svc.ResourceVersion = current.(*corev1.Service).ResourceVersion
				svc.Spec.ClusterIP = current.(*corev1.Service).Spec.ClusterIP
				desired = svc
			}

			if err := client.Update(context.TODO(), desired); err != nil {
				return errorfactory.New(errorfactory.APIFailure{}, err, "updating resource failed", "kind", desiredType)
			}
			switch desired.(type) {
			case *corev1.ConfigMap:
				// Only update status when configmap belongs to broker
				if id, ok := desired.(*corev1.ConfigMap).Labels["brokerId"]; ok {
					touchedConfigs := collectTouchedConfigs(current.(*corev1.ConfigMap), desired.(*corev1.ConfigMap), log)

					var statusErr error
					// if only per broker configs are changed, do not trigger rolling upgrade by setting ConfigOutOfSync status
					if containsOnlyPerBrokerConfigs(touchedConfigs) {
						log.V(1).Info("setting per broker config status to out of sync")
						statusErr = UpdateBrokerStatus(client, []string{id}, cr, banzaicloudv1beta1.PerBrokerConfigOutOfSync, log)
					} else {
						statusErr = UpdateBrokerStatus(client, []string{id}, cr, banzaicloudv1beta1.ConfigOutOfSync, log)
					}
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

// CheckIfObjectUpdated checks if the given object is updated using K8sObjectMatcher
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

func GetBrokerConfigsFromConfigMap(configMap *corev1.ConfigMap) map[string]string {
	brokerConfig := configMap.Data["broker-config"]
	configs := strings.Split(brokerConfig, "\n")
	m := make(map[string]string)
	for _, config := range configs {
		elements := strings.Split(config, "=")
		if len(elements) != 2 {
			continue
		}
		m[strings.TrimSpace(elements[0])] = strings.TrimSpace(elements[1])
	}
	return m
}

// collects are the config keys that are either added, removed or updated
// between the current and the desired ConfigMap
func collectTouchedConfigs(current, desired *corev1.ConfigMap, log logr.Logger) []string {
	touchedConfigs := make([]string, 0)
	currentConfigs := GetBrokerConfigsFromConfigMap(current)
	desiredConfigs := GetBrokerConfigsFromConfigMap(desired)

	for configName, desiredValue := range desiredConfigs {
		if currentValue, ok := currentConfigs[configName]; !ok || currentValue != desiredValue {
			// new or updated config
			touchedConfigs = append(touchedConfigs, configName)
		}
		delete(currentConfigs, configName)
	}

	for configName, _ := range currentConfigs {
		// deleted config
		touchedConfigs = append(touchedConfigs, configName)
	}

	log.V(1).Info("configs have been changed", "configs", touchedConfigs)
	return touchedConfigs
}

func IsPodContainsTerminatedContainer(pod *corev1.Pod) bool {
	for _, initContainerState := range pod.Status.InitContainerStatuses {
		if initContainerState.State.Terminated != nil &&
			strings.Contains(initContainerState.State.Terminated.Reason, "Error") {
			return true
		}
	}
	for _, containerState := range pod.Status.ContainerStatuses {
		if containerState.State.Terminated != nil {
			return true
		}
	}
	return false
}

// IsPodContainsEvictedContainer returns true if pod status has an evicted reason false otherwise
func IsPodContainsEvictedContainer(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed && strings.Contains(pod.Status.Reason, "Evicted") {
		return true
	}
	return false
}

func IsPodContainsPendingContainer(pod *corev1.Pod) bool {
	for _, containerState := range pod.Status.ContainerStatuses {
		if containerState.State.Waiting != nil {
			return true
		}
	}
	return false
}
