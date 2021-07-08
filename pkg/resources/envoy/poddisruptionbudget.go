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

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) podDisruptionBudget(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, extListener.Name)

	var deploymentName string
	if ingressConfigName == util.IngressConfigGlobalName {
		deploymentName = fmt.Sprintf(envoyDeploymentName, extListener.Name, r.KafkaCluster.GetName())
	} else {
		deploymentName = fmt.Sprintf(envoyDeploymentNameWithScope, extListener.Name, ingressConfigName, r.KafkaCluster.GetName())
	}

	pdbConfig := r.KafkaCluster.Spec.EnvoyConfig.DisruptionBudget

	// We use intstr.Parse so that the proper structure is used when passing to PDB spec validator, otherwise
	// an improper regex will be used to verify the value
	budget := intstr.Parse(pdbConfig.Budget)

	var spec policyv1beta1.PodDisruptionBudgetSpec
	var matchLabels map[string]string = labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName)

	if pdbConfig.Stategy == "minAvailable" {
		spec = policyv1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &budget,
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		}
	} else if pdbConfig.Stategy == "maxUnavailable" {
		spec = policyv1beta1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			MaxUnavailable: &budget,
		}
	}

	return &policyv1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1beta1",
		},
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			deploymentName+"-pdb",
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName),
			ingressConfig.EnvoyConfig.GetAnnotations(),
			r.KafkaCluster,
		),
		Spec: spec,
	}
}
