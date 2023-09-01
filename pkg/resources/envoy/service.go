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

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

// service return a external facing service for Envoy
func (r *Reconciler) service(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, extListener.Name)

	// Determine Service Name from the configuration
	var serviceName string = util.GenerateEnvoyResourceName(envoyutils.EnvoyServiceName, envoyutils.EnvoyServiceNameWithScope,
		extListener, ingressConfig, ingressConfigName, r.KafkaCluster.GetName())

	exposedPorts := getExposedServicePorts(extListener,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log),
		r.KafkaCluster, ingressConfig, ingressConfigName, defaultIngressConfigName, log)

	service := &corev1.Service{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			serviceName,
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName),
			ingressConfig.IngressServiceSettings.GetServiceAnnotations(), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector:                 labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName),
			Type:                     ingressConfig.GetServiceType(),
			Ports:                    exposedPorts,
			LoadBalancerSourceRanges: ingressConfig.EnvoyConfig.GetLoadBalancerSourceRanges(),
			LoadBalancerIP:           ingressConfig.EnvoyConfig.LoadBalancerIP,
			ExternalTrafficPolicy:    ingressConfig.ExternalTrafficPolicy,
		},
	}
	return service
}

func getExposedServicePorts(extListener v1beta1.ExternalListenerConfig, brokersIds []int,
	kafkaCluster *v1beta1.KafkaCluster, ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string, log logr.Logger) []corev1.ServicePort {
	var exposedPorts []corev1.ServicePort
	if !extListener.TLSEnabled() {
		for _, brokerId := range brokersIds {
			brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kafkaCluster.Spec, brokerId)
			if err != nil {
				log.Error(err, "could not determine brokerConfig")
				continue
			}
			if util.ShouldIncludeBroker(brokerConfig, kafkaCluster.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
				exposedPorts = append(exposedPorts, corev1.ServicePort{
					Name:       fmt.Sprintf("broker-%d", brokerId),
					Port:       extListener.GetBrokerPort(int32(brokerId)),
					TargetPort: intstr.FromInt(int(extListener.GetBrokerPort(int32(brokerId)))),
					Protocol:   corev1.ProtocolTCP,
				})
			}
		}
	}

	// append anycast port
	exposedPorts = append(exposedPorts, corev1.ServicePort{
		Name:       fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, "tcp"),
		TargetPort: intstr.FromInt(int(extListener.GetAnyCastPort())),
		Port:       extListener.GetAnyCastPort(),
		Protocol:   corev1.ProtocolTCP,
	})

	// append envoy healthcheck port
	exposedPorts = append(exposedPorts, corev1.ServicePort{
		Name:       "tcp-health",
		TargetPort: intstr.FromInt(int(ingressConfig.EnvoyConfig.GetEnvoyHealthCheckPort())),
		Port:       ingressConfig.EnvoyConfig.GetEnvoyHealthCheckPort(),
		Protocol:   corev1.ProtocolTCP,
	})

	// append envoy admin port
	exposedPorts = append(exposedPorts, corev1.ServicePort{
		Name:       "tcp-admin",
		TargetPort: intstr.FromInt(int(ingressConfig.EnvoyConfig.GetEnvoyAdminPort())),
		Port:       ingressConfig.EnvoyConfig.GetEnvoyAdminPort(),
		Protocol:   corev1.ProtocolTCP,
	})

	return exposedPorts
}
