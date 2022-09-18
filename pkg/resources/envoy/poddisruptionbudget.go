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
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
)

const (
	MIN_AVAILABLE   string = "minAvailable"
	MAX_UNAVAILABLE string = "maxUnavailable"
)

func (r *Reconciler) podDisruptionBudget(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, extListener.Name)

	var deploymentName = util.GenerateEnvoyResourceName(envoyutils.EnvoyDeploymentName, envoyutils.EnvoyDeploymentNameWithScope,
		extListener, ingressConfig, ingressConfigName, r.KafkaCluster.GetName())

	pdbConfig := r.KafkaCluster.Spec.EnvoyConfig.GetDistruptionBudget()

	// We use intstr.Parse so that the proper structure is used when passing to PDB spec validator, otherwise
	// an improper regex will be used to verify the value
	budget := intstr.Parse(pdbConfig.DisruptionBudget.Budget)

	var spec policyv1.PodDisruptionBudgetSpec
	var matchLabels = labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName)

	if pdbConfig.Stategy == MIN_AVAILABLE {
		spec = policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &budget,
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		}
	} else if pdbConfig.Stategy == MAX_UNAVAILABLE {
		spec = policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			MaxUnavailable: &budget,
		}
	}

	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1",
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
