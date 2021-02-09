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

package envoy

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	envoyutils "github.com/banzaicloud/kafka-operator/pkg/util/envoy"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName = "envoy"
	// The deployment and configmap name should made from the external listener name the cluster name to avoid all naming collision
	envoyVolumeAndConfigName          = "envoy-config-%s-%s"
	envoyVolumeAndConfigNameWithScope = "envoy-config-%s-%s-%s"
	envoyDeploymentName               = "envoy-%s-%s"
	envoyDeploymentNameWithScope      = "envoy-%s-%s-%s"
	allBrokerEnvoyConfigName          = "all-brokers"
)

var eListenerLabelName string

// labelsForEnvoyIngress returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForEnvoyIngress(crName, eLName string) map[string]string {
	return map[string]string{"app": "envoyingress", "eListenerName": eLName, "kafka_cr": crName}
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
	log = log.WithValues("component", componentName)

	log.V(1).Info("Reconciling")

	if r.KafkaCluster.Spec.GetIngressController() == envoyutils.IngressControllerName {

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
						eListenerLabelName = eListener.Name
					} else {
						eListenerLabelName = fmt.Sprintf("%s-%s", eListener.Name, name)
					}
					for _, res := range []resources.ResourceWithLogAndExternalListenerSpecificInfos{
						r.service,
						r.configMap,
						r.deployment,
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
