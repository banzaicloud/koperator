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

package kafka

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/util/kafka"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) podDisruptionBudget(log logr.Logger) (runtime.Object, error) {
	minAvailable, err := r.computeMinAvailable(log)

	if err != nil {
		return nil, err
	}

	return &policyv1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1beta1",
		},
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			fmt.Sprintf("%s-pdb", r.KafkaCluster.Name),
			util.MergeLabels(kafka.LabelsForKafka(r.KafkaCluster.Name), r.KafkaCluster.Labels),
			r.KafkaCluster.Spec.ListenersConfig.GetServiceAnnotations(),
			r.KafkaCluster,
		),
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: kafka.LabelsForKafka(r.KafkaCluster.Name),
			},
		},
	}, nil
}

// Calculate maxUnavailable as max between brokerCount - 1 (so we only allow 1 broker to be disrupted)
// and 1 (to cover for 1 broker clusters)
func (r *Reconciler) computeMinAvailable(log logr.Logger) (intstr.IntOrString, error) {
	/*
		budget = r.KafkaCluster.Spec.DisruptionBudget.budget (string) ->
		- can either be %percentage or static number

		Logic:

		Max(1, brokers-budget) - for a static number budget

		Max(1, brokers-brokers*percentage) - for a percentage budget

	*/
	// number of brokers in the KafkaCluster
	brokers := len(r.KafkaCluster.Status.BrokersState)

	// configured budget in the KafkaCluster
	disruptionBudget := r.KafkaCluster.Spec.DisruptionBudget.Budget

	budget := 0

	// treat percentage budget
	if strings.HasSuffix(disruptionBudget, "%") {
		percentage, err := strconv.ParseFloat(disruptionBudget[:len(disruptionBudget)-1], 32)
		if err != nil {
			log.Error(err, "error occurred during parsing the disruption budget")
			return intstr.FromInt(-1), err
		}
		budget = int(math.Floor((percentage * float64(brokers)) / 100))
	} else {
		// treat static number budget
		staticBudget, err := strconv.ParseInt(disruptionBudget, 10, 0)
		if err != nil {
			log.Error(err, "error occurred during parsing the disruption budget")
			return intstr.FromInt(-1), err
		}
		budget = int(staticBudget)
	}

	return intstr.FromInt(util.Max(1, brokers-budget)), nil
}
