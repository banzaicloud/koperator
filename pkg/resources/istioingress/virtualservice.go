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

package istioingress

import (
	"fmt"

	"github.com/banzaicloud/istio-client-go/pkg/networking/v1alpha3"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
)

func (r *Reconciler) virtualService(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig) runtime.Object {
	vServiceSpec := v1alpha3.VirtualServiceSpec{
		Hosts:    []string{"*"},
		Gateways: []string{fmt.Sprintf(gatewayNameTemplate, r.KafkaCluster.Name, externalListenerConfig.Name)},
	}

	if r.KafkaCluster.Spec.IstioIngressConfig.TLSOptions != nil &&
		r.KafkaCluster.Spec.IstioIngressConfig.TLSOptions.Mode == v1alpha3.TLSModePassThrough {
		vServiceSpec.TLS = generateTlsRoutes(r.KafkaCluster, externalListenerConfig, log)

	} else {
		vServiceSpec.TCP = generateTcpRoutes(r.KafkaCluster, externalListenerConfig, log)
	}

	return &v1alpha3.VirtualService{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			fmt.Sprintf(virtualServiceTemplate, r.KafkaCluster.Name, externalListenerConfig.Name),
			labelsForIstioIngress(r.KafkaCluster.Name, externalListenerConfig.Name),
			r.KafkaCluster.Spec.IstioIngressConfig.GetVirtualServiceAnnotations(),
			r.KafkaCluster),
		Spec: vServiceSpec,
	}
}

func generateTlsRoutes(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger) []v1alpha3.TLSRoute {
	tlsRoutes := make([]v1alpha3.TLSRoute, 0)

	for _, broker := range kc.Spec.Brokers {
		tlsRoutes = append(tlsRoutes, v1alpha3.TLSRoute{
			Match: []v1alpha3.TLSMatchAttributes{
				{
					Port:     util.IntPointer(int(broker.Id + externalListenerConfig.ExternalStartingPort)),
					SniHosts: []string{"*"},
				},
			},
			Route: []*v1alpha3.RouteDestination{
				{
					Destination: &v1alpha3.Destination{
						Host: fmt.Sprintf("%s-%d", kc.Name, broker.Id),
						Port: &v1alpha3.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
					},
				},
			},
		})
	}
	if !kc.Spec.HeadlessServiceEnabled && len(kc.Spec.ListenersConfig.ExternalListeners) > 0 {
		tlsRoutes = append(tlsRoutes, v1alpha3.TLSRoute{
			Match: []v1alpha3.TLSMatchAttributes{
				{
					Port:     util.IntPointer(int(kc.Spec.ListenersConfig.InternalListeners[0].ContainerPort)),
					SniHosts: []string{"*"},
				},
			},
			Route: []*v1alpha3.RouteDestination{
				{
					Destination: &v1alpha3.Destination{
						Host: fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, kc.Name),
						Port: &v1alpha3.PortSelector{Number: uint32(kc.Spec.ListenersConfig.ExternalListeners[0].ContainerPort)},
					},
				},
			},
		})
	}

	return tlsRoutes
}

func generateTcpRoutes(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger) []v1alpha3.TCPRoute {
	tcpRoutes := make([]v1alpha3.TCPRoute, 0)

	for _, broker := range kc.Spec.Brokers {
		tcpRoutes = append(tcpRoutes, v1alpha3.TCPRoute{
			Match: []v1alpha3.L4MatchAttributes{
				{
					Port: util.IntPointer(int(broker.Id + externalListenerConfig.ExternalStartingPort)),
				},
			},
			Route: []*v1alpha3.RouteDestination{
				{
					Destination: &v1alpha3.Destination{
						Host: fmt.Sprintf("%s-%d", kc.Name, broker.Id),
						Port: &v1alpha3.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
					},
				},
			},
		})
	}
	if !kc.Spec.HeadlessServiceEnabled && len(kc.Spec.ListenersConfig.ExternalListeners) > 0 {
		tcpRoutes = append(tcpRoutes, v1alpha3.TCPRoute{
			Match: []v1alpha3.L4MatchAttributes{
				{
					Port: util.IntPointer(int(kc.Spec.ListenersConfig.InternalListeners[0].ContainerPort)),
				},
			},
			Route: []*v1alpha3.RouteDestination{
				{
					Destination: &v1alpha3.Destination{
						Host: fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, kc.Name),
						Port: &v1alpha3.PortSelector{Number: uint32(kc.Spec.ListenersConfig.ExternalListeners[0].ContainerPort)},
					},
				},
			},
		})
	}

	return tcpRoutes
}
