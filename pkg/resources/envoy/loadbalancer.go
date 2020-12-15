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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	envoyutils "github.com/banzaicloud/kafka-operator/pkg/util/envoy"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"

	corev1 "k8s.io/api/core/v1"
)

// loadBalancer return a Loadbalancer service for Envoy
func (r *Reconciler) loadBalancer(log logr.Logger, extListener v1beta1.ExternalListenerConfig) runtime.Object {

	exposedPorts := getExposedServicePorts(extListener,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log))

	if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
		exposedPorts = append(exposedPorts, corev1.ServicePort{
			Name:       fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, "tcp"),
			TargetPort: intstr.FromInt(int(extListener.GetAnyCastPort())),
			Port:       extListener.GetAnyCastPort(),
		})
	}

	service := &corev1.Service{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			fmt.Sprintf(envoyutils.EnvoyServiceName, extListener.Name, r.KafkaCluster.GetName()),
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), extListener.Name),
			r.KafkaCluster.Spec.EnvoyConfig.GetAnnotations(), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector:                 labelsForEnvoyIngress(r.KafkaCluster.GetName(), extListener.Name),
			Type:                     corev1.ServiceTypeLoadBalancer,
			Ports:                    exposedPorts,
			LoadBalancerSourceRanges: r.KafkaCluster.Spec.EnvoyConfig.GetLoadBalancerSourceRanges(),
			LoadBalancerIP:           r.KafkaCluster.Spec.EnvoyConfig.LoadBalancerIP,
			ExternalTrafficPolicy:    extListener.ExternalTrafficPolicy,
		},
	}
	return service
}

func getExposedServicePorts(extListener v1beta1.ExternalListenerConfig, brokersIds []int) []corev1.ServicePort {
	var exposedPorts []corev1.ServicePort
	for _, brokerId := range brokersIds {
		exposedPorts = append(exposedPorts, corev1.ServicePort{
			Name:       fmt.Sprintf("broker-%d", brokerId),
			Port:       extListener.ExternalStartingPort + int32(brokerId),
			TargetPort: intstr.FromInt(int(extListener.ExternalStartingPort) + brokerId),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	return exposedPorts
}
