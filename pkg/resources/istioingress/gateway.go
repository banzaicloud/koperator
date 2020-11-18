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
)

func (r *Reconciler) gateway(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig) runtime.Object {
	return &v1alpha3.Gateway{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(gatewayNameTemplate, r.KafkaCluster.Name, externalListenerConfig.Name), labelsForIstioIngress(r.KafkaCluster.Name, externalListenerConfig.Name), r.KafkaCluster),
		Spec: v1alpha3.GatewaySpec{
			Selector: labelsForIstioIngress(r.KafkaCluster.Name, externalListenerConfig.Name),
			Servers:  generateServers(r.KafkaCluster, externalListenerConfig, log),
		},
	}
}

func generateServers(kc *v1beta1.KafkaCluster, externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger) []v1alpha3.Server {
	servers := make([]v1alpha3.Server, 0)
	protocol := v1alpha3.ProtocolTCP
	var tlsConfig *v1alpha3.TLSOptions
	if kc.Spec.IstioIngressConfig.TLSOptions != nil {
		tlsConfig = kc.Spec.IstioIngressConfig.TLSOptions
		protocol = v1alpha3.ProtocolTLS
	}

	brokerIds := util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log)

	for _, brokerId := range brokerIds {
		servers = append(servers, v1alpha3.Server{
			Port: &v1alpha3.Port{
				Number:   int(externalListenerConfig.ExternalStartingPort) + brokerId,
				Protocol: protocol,
				Name:     fmt.Sprintf("tcp-broker-%d", brokerId),
			},
			TLS:   tlsConfig,
			Hosts: []string{"*"},
		})
	}
	if !kc.Spec.HeadlessServiceEnabled && len(kc.Spec.ListenersConfig.ExternalListeners) > 0 {
		servers = append(servers, v1alpha3.Server{
			Port: &v1alpha3.Port{
				Number:   int(kc.Spec.ListenersConfig.InternalListeners[0].ContainerPort),
				Protocol: protocol,
				Name:     "tcp-" + allBrokers,
			},
			Hosts: []string{"*"},
			TLS:   tlsConfig,
		})
	}

	return servers
}
