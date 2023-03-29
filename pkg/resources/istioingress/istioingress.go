// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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

package istioingress

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"

	istioclientv1beta1 "github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"
	istioOperatorApi "github.com/banzaicloud/istio-operator/api/v2/v1alpha1"
	"github.com/banzaicloud/operator-tools/pkg/utils"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/util/istioingress"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName                   = "istioingress"
	gatewayNameTemplate             = "%s-%s-gateway"
	gatewayNameTemplateWithScope    = "%s-%s-%s-gateway"
	virtualServiceTemplate          = "%s-%s-virtualservice"
	virtualServiceTemplateWithScope = "%s-%s-%s-virtualservice"
)

// labelsForIstioIngress returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForIstioIngress(crName, eLName, istioRevision string) map[string]string {
	return utils.MergeLabels(labelsForIstioIngressWithoutEListenerName(crName, istioRevision), map[string]string{util.ExternalListenerLabelNameKey: eLName})
}

func labelsForIstioIngressWithoutEListenerName(crName, istioRevision string) map[string]string {
	labels := map[string]string{v1beta1.AppLabelKey: "istioingress", v1beta1.KafkaCRLabelKey: crName}
	if istioRevision != "" {
		labels["istio.io/rev"] = istioRevision
	}
	return labels
}

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for IstioIngress
func New(client client.Client, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for IstioIngress
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)
	log.V(1).Info("Reconciling")

	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		if r.KafkaCluster.Spec.GetIngressController() == istioingress.IngressControllerName && eListener.GetAccessMethod() == corev1.ServiceTypeLoadBalancer {
			if r.KafkaCluster.Spec.IstioControlPlane == nil {
				log.Error(errors.NewPlain("reference to Istio Control Plane is missing"), "skip external listener reconciliation", "external listener", eListener.Name)
				continue
			}

			istioRevision := istioOperatorApi.NamespacedRevision(
				strings.ReplaceAll(r.KafkaCluster.Spec.IstioControlPlane.Name, ".", "-"),
				r.KafkaCluster.Spec.IstioControlPlane.Namespace)
			ingressConfigs, defaultControllerName, err := util.GetIngressConfigs(r.KafkaCluster.Spec, eListener)
			if err != nil {
				return err
			}
			for name, ingressConfig := range ingressConfigs {
				if !util.IsIngressConfigInUse(name, defaultControllerName, r.KafkaCluster, log) {
					continue
				}
				for _, res := range []resources.ResourceWithLogAndExternalListenerSpecificInfosAndIstioRevision{
					r.meshgateway,
					r.gateway,
					r.virtualService,
				} {
					o := res(log, eListener, ingressConfig, name, defaultControllerName, istioRevision)
					err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
					if err != nil {
						return err
					}
				}
			}
		} else if r.KafkaCluster.Spec.RemoveUnusedIngressResources {
			// Cleaning up unused istio resources when ingress controller is not istioingress or externalListener access method is not LoadBalancer
			deletionCounter := 0
			ctx := context.Background()
			istioResourcesGVK := []schema.GroupVersionKind{
				{
					Version: istioOperatorApi.GroupVersion.Version,
					Group:   istioOperatorApi.GroupVersion.Group,
					Kind:    reflect.TypeOf(istioOperatorApi.IstioMeshGateway{}).Name(),
				},
				{
					Version: istioclientv1beta1.SchemeGroupVersion.Version,
					Group:   istioclientv1beta1.SchemeGroupVersion.Group,
					Kind:    reflect.TypeOf(istioclientv1beta1.Gateway{}).Name(),
				},
				{
					Version: istioclientv1beta1.SchemeGroupVersion.Version,
					Group:   istioclientv1beta1.SchemeGroupVersion.Group,
					Kind:    reflect.TypeOf(istioclientv1beta1.VirtualService{}).Name(),
				},
			}
			var istioResources unstructured.UnstructuredList
			for _, gvk := range istioResourcesGVK {
				istioResources.SetGroupVersionKind(gvk)

				if err := r.List(ctx, &istioResources, client.InNamespace(r.KafkaCluster.GetNamespace()),
					client.MatchingLabels(labelsForIstioIngressWithoutEListenerName(r.KafkaCluster.Name, ""))); err != nil && !apimeta.IsNoMatchError(err) {
					return errors.Wrap(err, "error when getting list of istio ingress resources for deletion")
				}

				for _, removeObject := range istioResources.Items {
					if !strings.Contains(removeObject.GetLabels()[util.ExternalListenerLabelNameKey], eListener.Name) ||
						util.ObjectManagedByClusterRegistry(&removeObject) ||
						!removeObject.GetDeletionTimestamp().IsZero() {
						continue
					}
					if err := r.Delete(ctx, &removeObject); client.IgnoreNotFound(err) != nil {
						return errors.Wrap(err, "error when removing istio ingress resources")
					}
					log.V(1).Info(fmt.Sprintf("Deleted istio ingress '%s' resource '%s' for externalListener '%s'", gvk.Kind, removeObject.GetName(), eListener.Name))
					deletionCounter++
				}
			}
			if deletionCounter > 0 {
				log.Info(fmt.Sprintf("Removed '%d' resources for istio ingress", deletionCounter))
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}
