// Copyright © 2020 Banzai Cloud
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
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/banzaicloud/kafka-operator/pkg/util/istioingress"

	"fmt"

	corev1 "k8s.io/api/core/v1"

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

var annotationName string

// labelsForIstioIngress returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForIstioIngress(crName, eLName string) map[string]string {
	return map[string]string{"app": "istioingress", "eListenerName": eLName, "kafka_cr": crName}
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
	if r.KafkaCluster.Spec.GetIngressController() == istioingress.IngressControllerName {

		for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
			if eListener.GetAccessMethod() == corev1.ServiceTypeLoadBalancer {

				ingressConfigs, defaultControllerName, err := util.GetIngressConfigs(r.KafkaCluster.Spec, eListener)
				if err != nil {
					return err
				}
				for name, ingressConfig := range ingressConfigs {
					if !util.IsIngressConfigInUse(name, defaultControllerName, r.KafkaCluster, log) {
						continue
					}
					if name == util.IngressConfigGlobalName {
						annotationName = eListener.Name
					} else {
						annotationName = fmt.Sprintf("%s-%s", eListener.Name, name)
					}
					for _, res := range []resources.ResourceWithLogAndExternalListenerSpecificInfos{
						r.meshgateway,
						r.gateway,
						r.virtualService,
					} {
						o := res(log, eListener, ingressConfig, name, defaultControllerName)
						err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}
