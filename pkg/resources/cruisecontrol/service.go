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

package cruisecontrol

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (r *Reconciler) service(log logr.Logger, clientPass string) runtime.Object {
	return &corev1.Service{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(serviceNameTemplate, r.KafkaCluster.Name), labelSelector, r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector: labelSelector,
			Ports: []corev1.ServicePort{
				{
					Name:       "cc",
					Port:       8090,
					TargetPort: intstr.FromInt(8090),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       metricsPort,
					TargetPort: intstr.FromInt(metricsPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
