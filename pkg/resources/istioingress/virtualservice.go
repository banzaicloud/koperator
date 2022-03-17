// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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

	istioclientv1beta1 "github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

func (r *Reconciler) virtualService(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName, istioRevision string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, externalListenerConfig.Name)

	var gatewayName, virtualSName string
	if ingressConfigName == util.IngressConfigGlobalName {
		gatewayName = fmt.Sprintf(gatewayNameTemplate, r.KafkaCluster.Name, externalListenerConfig.Name)
		virtualSName = fmt.Sprintf(virtualServiceTemplate, r.KafkaCluster.Name, externalListenerConfig.Name)
	} else {
		gatewayName = fmt.Sprintf(gatewayNameTemplateWithScope, r.KafkaCluster.Name, externalListenerConfig.Name, ingressConfigName)
		virtualSName = fmt.Sprintf(virtualServiceTemplateWithScope, r.KafkaCluster.Name, externalListenerConfig.Name, ingressConfigName)
	}

	vServiceSpec := istioclientv1beta1.VirtualServiceSpec{
		Hosts:    []string{"*"},
		Gateways: []string{gatewayName},
	}

	if ingressConfig.IstioIngressConfig.TLSOptions != nil &&
		ingressConfig.IstioIngressConfig.TLSOptions.Mode == istioclientv1beta1.TLSModePassThrough {
		vServiceSpec.TLS = generateTlsRoutes(r.KafkaCluster, externalListenerConfig, log, ingressConfigName, defaultIngressConfigName)
	} else {
		vServiceSpec.TCP = generateTcpRoutes(r.KafkaCluster, externalListenerConfig, log, ingressConfigName, defaultIngressConfigName)
	}

	return &istioclientv1beta1.VirtualService{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			virtualSName,
			labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName, istioRevision),
			ingressConfig.IstioIngressConfig.GetVirtualServiceAnnotations(),
			r.KafkaCluster),
		Spec: vServiceSpec,
	}
}

func generateTlsRoutes(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger,
	ingressConfigName, defaultIngressConfigName string) []istioclientv1beta1.TLSRoute {
	tlsRoutes := make([]istioclientv1beta1.TLSRoute, 0)

	brokerIds := util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log)

	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kc.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kc.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
			tlsRoutes = append(tlsRoutes, istioclientv1beta1.TLSRoute{
				Match: []istioclientv1beta1.TLSMatchAttributes{
					{
						Port:     util.IntPointer(int(externalListenerConfig.GetBrokerPort(int32(brokerId)))),
						SniHosts: []string{"*"},
					},
				},
				Route: []*istioclientv1beta1.RouteDestination{
					{
						Destination: &istioclientv1beta1.Destination{
							Host: fmt.Sprintf("%s-%d", kc.Name, brokerId),
							Port: &istioclientv1beta1.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
						},
					},
				},
			})
		}
	}
	if !kc.Spec.HeadlessServiceEnabled && len(kc.Spec.ListenersConfig.ExternalListeners) > 0 {
		tlsRoutes = append(tlsRoutes, istioclientv1beta1.TLSRoute{
			Match: []istioclientv1beta1.TLSMatchAttributes{
				{
					Port:     util.IntPointer(int(externalListenerConfig.GetAnyCastPort())),
					SniHosts: []string{"*"},
				},
			},
			Route: []*istioclientv1beta1.RouteDestination{
				{
					Destination: &istioclientv1beta1.Destination{
						Host: fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, kc.Name),
						Port: &istioclientv1beta1.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
					},
				},
			},
		})
	}

	return tlsRoutes
}

func generateTcpRoutes(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger,
	ingressConfigName, defaultIngressConfigName string) []istioclientv1beta1.TCPRoute {
	tcpRoutes := make([]istioclientv1beta1.TCPRoute, 0)

	brokerIds := util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log)

	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kc.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kc.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
			tcpRoutes = append(tcpRoutes, istioclientv1beta1.TCPRoute{
				Match: []istioclientv1beta1.L4MatchAttributes{
					{
						Port: util.IntPointer(int(externalListenerConfig.GetBrokerPort(int32(brokerId)))),
					},
				},
				Route: []*istioclientv1beta1.RouteDestination{
					{
						Destination: &istioclientv1beta1.Destination{
							Host: fmt.Sprintf("%s-%d", kc.Name, brokerId),
							Port: &istioclientv1beta1.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
						},
					},
				},
			})
		}
	}
	if !kc.Spec.HeadlessServiceEnabled {
		tcpRoutes = append(tcpRoutes, istioclientv1beta1.TCPRoute{
			Match: []istioclientv1beta1.L4MatchAttributes{
				{
					Port: util.IntPointer(int(externalListenerConfig.GetAnyCastPort())),
				},
			},
			Route: []*istioclientv1beta1.RouteDestination{
				{
					Destination: &istioclientv1beta1.Destination{
						Host: fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, kc.Name),
						Port: &istioclientv1beta1.PortSelector{Number: uint32(externalListenerConfig.ContainerPort)},
					},
				},
			},
		})
	}

	return tcpRoutes
}
