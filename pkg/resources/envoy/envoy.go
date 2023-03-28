// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

package envoy

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiutil "github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources"
	"github.com/banzaicloud/koperator/pkg/util"
	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
)

// labelsForEnvoyIngress returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForEnvoyIngress(crName, eLName string) map[string]string {
	return apiutil.MergeLabels(labelsForEnvoyIngressWithoutEListenerName(crName), map[string]string{util.ExternalListenerLabelNameKey: eLName})
}

func labelsForEnvoyIngressWithoutEListenerName(crName string) map[string]string {
	return map[string]string{v1beta1.AppLabelKey: "envoyingress", v1beta1.KafkaCRLabelKey: crName}
}

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for Envoy
func New(client client.Client, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for Envoy
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", envoyutils.ComponentName)

	log.V(1).Info("Reconciling")
	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		if r.KafkaCluster.Spec.GetIngressController() == envoyutils.IngressControllerName && eListener.GetAccessMethod() == corev1.ServiceTypeLoadBalancer {
			ingressConfigs, defaultControllerName, err := util.GetIngressConfigs(r.KafkaCluster.Spec, eListener)
			if err != nil {
				return err
			}
			var externalListenerResources []resources.ResourceWithLogAndExternalListenerSpecificInfos
			externalListenerResources = append(externalListenerResources,
				r.service,
				r.configMap,
				r.deployment,
			)

			if r.KafkaCluster.Spec.EnvoyConfig.GetDistruptionBudget().DisruptionBudget.Create {
				externalListenerResources = append(externalListenerResources, r.podDisruptionBudget)
			}
			for name, ingressConfig := range ingressConfigs {
				if !util.IsIngressConfigInUse(name, defaultControllerName, r.KafkaCluster, log) {
					continue
				}

				for _, res := range externalListenerResources {
					o := res(log, eListener, ingressConfig, name, defaultControllerName)
					err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
					if err != nil {
						return err
					}
				}
			}
		} else {
			// Cleaning up unused envoy resources when ingress controller is not envoy or externalListener access method is not LoadBalancer
			deletionCounter := 0
			ctx := context.Background()
			envoyResourcesGVK := []schema.GroupVersionKind{
				{
					Version: corev1.SchemeGroupVersion.Version,
					Group:   corev1.SchemeGroupVersion.Group,
					Kind:    reflect.TypeOf(corev1.Service{}).Name(),
				},
				{
					Version: corev1.SchemeGroupVersion.Version,
					Group:   corev1.SchemeGroupVersion.Group,
					Kind:    reflect.TypeOf(corev1.ConfigMap{}).Name(),
				},
				{
					Version: appsv1.SchemeGroupVersion.Version,
					Group:   appsv1.SchemeGroupVersion.Group,
					Kind:    reflect.TypeOf(appsv1.Deployment{}).Name(),
				},
				{
					Version: policyv1.SchemeGroupVersion.Version,
					Group:   policyv1.SchemeGroupVersion.Group,
					Kind:    reflect.TypeOf(policyv1.PodDisruptionBudget{}).Name(),
				},
			}
			var envoyResources unstructured.UnstructuredList
			for _, gvk := range envoyResourcesGVK {
				envoyResources.SetGroupVersionKind(gvk)

				if err := r.List(ctx, &envoyResources, client.InNamespace(r.KafkaCluster.GetNamespace()),
					client.MatchingLabels(labelsForEnvoyIngressWithoutEListenerName(r.KafkaCluster.Name))); err != nil {
					return errors.Wrap(err, "error when getting list of envoy ingress resources for deletion")
				}

				for _, removeObject := range envoyResources.Items {
					if !strings.Contains(removeObject.GetLabels()[util.ExternalListenerLabelNameKey], eListener.Name) ||
						util.ObjectManagedByClusterRegistry(&removeObject) ||
						!removeObject.GetDeletionTimestamp().IsZero() {
						continue
					}
					if err := r.Delete(ctx, &removeObject); client.IgnoreNotFound(err) != nil {
						return errors.Wrap(err, "error when removing envoy ingress resources")
					}
					deletionCounter++
				}
			}
			if deletionCounter > 0 {
				log.Info(fmt.Sprintf("Removed '%d' resources for envoy ingress", deletionCounter))
			}
		}
	}
	log.V(1).Info("Reconciled")

	return nil
}
