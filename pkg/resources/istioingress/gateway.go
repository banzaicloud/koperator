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

	istioclientv1beta1 "github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

func (r *Reconciler) gateway(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig,
	ingressConf v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, externalListenerConfig.Name)

	var gatewayName string
	if ingressConfigName == util.IngressConfigGlobalName {
		gatewayName = fmt.Sprintf(gatewayNameTemplate, r.KafkaCluster.Name, externalListenerConfig.Name)
	} else {
		gatewayName = fmt.Sprintf(gatewayNameTemplateWithScope, r.KafkaCluster.Name, externalListenerConfig.Name, ingressConfigName)
	}
	return &istioclientv1beta1.Gateway{
		ObjectMeta: templates.ObjectMeta(gatewayName,
			labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName), r.KafkaCluster),
		Spec: istioclientv1beta1.GatewaySpec{
			Selector: labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName),
			Servers: generateServers(r.KafkaCluster, externalListenerConfig, log, ingressConf,
				ingressConfigName, defaultIngressConfigName),
		},
	}
}

func generateServers(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger,
	ingressConf v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) []istioclientv1beta1.Server {
	servers := make([]istioclientv1beta1.Server, 0)
	protocol := istioclientv1beta1.ProtocolTCP
	var tlsConfig *istioclientv1beta1.TLSOptions
	if ingressConf.IstioIngressConfig.TLSOptions != nil {
		tlsConfig = ingressConf.IstioIngressConfig.TLSOptions
		protocol = istioclientv1beta1.ProtocolTLS
	}

	brokerIds := util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log)

	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kc.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kc.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
			servers = append(servers, istioclientv1beta1.Server{
				Port: &istioclientv1beta1.Port{
					Number:   int(externalListenerConfig.ExternalStartingPort) + brokerId,
					Protocol: protocol,
					Name:     fmt.Sprintf("tcp-broker-%d", brokerId),
				},
				TLS:   tlsConfig,
				Hosts: []string{"*"},
			})
		}
	}
	servers = append(servers, istioclientv1beta1.Server{
		Port: &istioclientv1beta1.Port{
			Number:   int(externalListenerConfig.GetAnyCastPort()),
			Protocol: protocol,
			Name:     fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, "tcp"),
		},
		Hosts: []string{"*"},
		TLS:   tlsConfig,
	})

	return servers
}
