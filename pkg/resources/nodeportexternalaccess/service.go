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

package nodeportexternalaccess

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/util/kafka"
)

// TODO handle deletion gracefully from status
func (r *Reconciler) service(_ logr.Logger, id int32,
	brokerConfig *v1beta1.BrokerConfig, extListener v1beta1.ExternalListenerConfig) runtime.Object {
	nodePort := int32(0)
	if extListener.ExternalStartingPort > 0 {
		nodePort = extListener.GetBrokerPort(id)
	}
	service := &corev1.Service{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			fmt.Sprintf(kafka.NodePortServiceTemplate, r.KafkaCluster.GetName(), id, extListener.Name),
			util.MergeLabels(kafka.LabelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", id)}),
			extListener.GetServiceAnnotations(), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector: util.MergeLabels(kafka.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{"brokerId": fmt.Sprintf("%d", id)}),
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{
				Name:       fmt.Sprintf("broker-%d", id),
				Port:       extListener.ContainerPort,
				NodePort:   nodePort,
				TargetPort: intstr.FromInt(int(extListener.ContainerPort)),
				Protocol:   corev1.ProtocolTCP,
			},
			},
			ExternalTrafficPolicy: extListener.ExternalTrafficPolicy,
		},
	}
	if nodePortExternalIP, ok := brokerConfig.NodePortExternalIP[extListener.Name]; ok && nodePortExternalIP != "" {
		service.Spec.ExternalIPs = []string{nodePortExternalIP}
	}
	return service
}
