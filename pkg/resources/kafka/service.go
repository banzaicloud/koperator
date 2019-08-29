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
	"strings"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) service(broker banzaicloudv1alpha1.Brokers, log logr.Logger) runtime.Object {

	var usedPorts []corev1.ServicePort

	for _, iListeners := range r.KafkaCluster.Spec.ListenersConfig.InternalListeners {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name:       strings.ReplaceAll(iListeners.Name, "_", ""),
			Port:       iListeners.ContainerPort,
			TargetPort: intstr.FromInt(int(iListeners.ContainerPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	usedPorts = append(usedPorts, corev1.ServicePort{
		Name:       "metrics",
		Port:       metricsPort,
		TargetPort: intstr.FromInt(metricsPort),
		Protocol:   corev1.ProtocolTCP,
	})

	return &corev1.Service{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf("%s-%d", r.KafkaCluster.Name, broker.Id), util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", broker.Id)}), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
			Selector:        util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", broker.Id)}),
			Ports:           usedPorts,
		},
	}

}
