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

package nodeportExternalAccess

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/banzaicloud/kafka-operator/pkg/util/kafka"
)

func (r *Reconciler) service(log logr.Logger, id int32,
	brokerConfig *v1beta1.BrokerConfig, extListener v1beta1.ExternalListenerConfig) runtime.Object {

	exposedPorts := getExposedServicePorts(extListener,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log))

	service := &corev1.Service{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			fmt.Sprintf(serviceName, r.KafkaCluster.GetName(), id, extListener.Name),
			util.MergeLabels(kafka.LabelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", id)}),
			extListener.GetServiceAnnotations(), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector: util.MergeLabels(kafka.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{"brokerId": fmt.Sprintf("%d", id)}),
			Type:                  corev1.ServiceTypeNodePort,
			Ports:                 exposedPorts,
			ExternalTrafficPolicy: extListener.ExternalTrafficPolicy,
		},
	}
	if brokerConfig.NodePortExternalIP != "" {
		service.Spec.ExternalIPs = []string{brokerConfig.NodePortExternalIP}
	}
	return service
}

func getExposedServicePorts(extListener v1beta1.ExternalListenerConfig, brokersIds []int) []corev1.ServicePort {
	var exposedPorts []corev1.ServicePort
	for _, brokerId := range brokersIds {
		exposedPorts = append(exposedPorts, corev1.ServicePort{
			Name:       fmt.Sprintf("broker-%d", brokerId),
			TargetPort: intstr.FromInt(int(extListener.ContainerPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	return exposedPorts
}
