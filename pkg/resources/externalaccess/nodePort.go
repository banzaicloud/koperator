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

package externalaccess

import (
	"fmt"
	"strconv"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// nodePort return a nodePort service for Kafka Broker
func (r *Reconciler) nodePort(log logr.Logger, eListener banzaicloudv1alpha1.ExternalListenerConfig,
	broker banzaicloudv1alpha1.BrokerConfig, crName string) runtime.Object {
	return &corev1.Service{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf("%s-%d-svc", crName, broker.Id),
			map[string]string{}, r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "kafka", "brokerId": strconv.Itoa(int(broker.Id))},
			Type:     corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name:       fmt.Sprintf("broker-%d", broker.Id),
					Port:       eListener.ContainerPort,
					TargetPort: intstr.FromInt(int(eListener.ContainerPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
