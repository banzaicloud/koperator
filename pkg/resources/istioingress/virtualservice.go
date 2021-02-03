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

func (r *Reconciler) virtualService(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {

	var gatewayName, virtualSName string
	if ingressConfigName == util.IngressConfigGlobalName {
		gatewayName = fmt.Sprintf(gatewayNameTemplate, r.KafkaCluster.Name, externalListenerConfig.Name)
		virtualSName = fmt.Sprintf(virtualServiceTemplate, r.KafkaCluster.Name, externalListenerConfig.Name)
	} else {
		gatewayName = fmt.Sprintf(gatewayNameTemplateWithScope, r.KafkaCluster.Name, externalListenerConfig.Name, ingressConfigName)
		virtualSName = fmt.Sprintf(virtualServiceTemplateWithScope, r.KafkaCluster.Name, externalListenerConfig.Name, ingressConfigName)
	}

	vServiceSpec := v1alpha3.VirtualServiceSpec{
		Hosts:    []string{"*"},
		Gateways: []string{gatewayName},
	}

	if ingressConfig.IstioIngressConfig.TLSOptions != nil &&
		ingressConfig.IstioIngressConfig.TLSOptions.Mode == v1alpha3.TLSModePassThrough {
		vServiceSpec.TLS = generateTlsRoutes(r.KafkaCluster, externalListenerConfig, log, ingressConfigName, defaultIngressConfigName)

	} else {
		vServiceSpec.TCP = generateTcpRoutes(r.KafkaCluster, externalListenerConfig, log, ingressConfigName, defaultIngressConfigName)
	}

	return &v1alpha3.VirtualService{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			virtualSName,
			labelsForIstioIngress(r.KafkaCluster.Name, annotationName),
			ingressConfig.IstioIngressConfig.GetVirtualServiceAnnotations(),
			r.KafkaCluster),
		Spec: vServiceSpec,
	}
}

func generateTlsRoutes(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger,
	ingressConfigName, defaultIngressConfigName string) []v1alpha3.TLSRoute {
	tlsRoutes := make([]v1alpha3.TLSRoute, 0)

	brokerIds := util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log)

	for _, brokerId := range brokerIds {
		brokerConfig, err := util.GetBrokerConfig(kc.Spec.Brokers[brokerId], kc.Spec)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if (len(brokerConfig.BrokerIdBindings) == 0 && ingressConfigName == defaultIngressConfigName) ||
			util.StringSliceContains(brokerConfig.BrokerIdBindings, ingressConfigName) ||
			ingressConfigName == "" {
			tlsRoutes = append(tlsRoutes, v1alpha3.TLSRoute{
				Match: []v1alpha3.TLSMatchAttributes{
					{
						Port:     util.IntPointer(int(externalListenerConfig.ExternalStartingPort) + brokerId),
						SniHosts: []string{"*"},
					},
				},
				Route: []*v1alpha3.RouteDestination{
					{
						Destination: &v1alpha3.Destination{
							Host: fmt.Sprintf("%s-%d", kc.Name, brokerId),
							Port: &v1alpha3.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
						},
					},
				},
			})
		}
	}
	if !kc.Spec.HeadlessServiceEnabled && len(kc.Spec.ListenersConfig.ExternalListeners) > 0 {
		tlsRoutes = append(tlsRoutes, v1alpha3.TLSRoute{
			Match: []v1alpha3.TLSMatchAttributes{
				{
					Port:     util.IntPointer(int(externalListenerConfig.GetAnyCastPort())),
					SniHosts: []string{"*"},
				},
			},
			Route: []*v1alpha3.RouteDestination{
				{
					Destination: &v1alpha3.Destination{
						Host: fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, kc.Name),
						Port: &v1alpha3.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
					},
				},
			},
		})
	}

	return tlsRoutes
}

func generateTcpRoutes(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger,
	ingressConfigName, defaultIngressConfigName string) []v1alpha3.TCPRoute {

	tcpRoutes := make([]v1alpha3.TCPRoute, 0)

	brokerIds := util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log)

	for _, brokerId := range brokerIds {
		brokerConfig, err := util.GetBrokerConfig(kc.Spec.Brokers[brokerId], kc.Spec)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, defaultIngressConfigName, ingressConfigName) {
			tcpRoutes = append(tcpRoutes, v1alpha3.TCPRoute{
				Match: []v1alpha3.L4MatchAttributes{
					{
						Port: util.IntPointer(int(externalListenerConfig.ExternalStartingPort) + brokerId),
					},
				},
				Route: []*v1alpha3.RouteDestination{
					{
						Destination: &v1alpha3.Destination{
							Host: fmt.Sprintf("%s-%d", kc.Name, brokerId),
							Port: &v1alpha3.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
						},
					},
				},
			})
		}
	}
	if !kc.Spec.HeadlessServiceEnabled {
		tcpRoutes = append(tcpRoutes, v1alpha3.TCPRoute{
			Match: []v1alpha3.L4MatchAttributes{
				{
					Port: util.IntPointer(int(externalListenerConfig.GetAnyCastPort())),
				},
			},
			Route: []*v1alpha3.RouteDestination{
				{
					Destination: &v1alpha3.Destination{
						Host: fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, kc.Name),
						Port: &v1alpha3.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
					},
				},
			},
		})
	}

	return tcpRoutes
}
