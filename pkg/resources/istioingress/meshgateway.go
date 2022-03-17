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

	istioOperatorApi "github.com/banzaicloud/istio-operator/api/v2/v1alpha1"
	"github.com/go-logr/logr"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	istioingressutils "github.com/banzaicloud/koperator/pkg/util/istioingress"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

func (r *Reconciler) meshgateway(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName, istioRevision string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, externalListenerConfig.Name)

	var meshgatewayName string
	if ingressConfigName == util.IngressConfigGlobalName {
		meshgatewayName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplate, externalListenerConfig.Name, r.KafkaCluster.GetName())
	} else {
		meshgatewayName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplateWithScope,
			externalListenerConfig.Name, ingressConfigName, r.KafkaCluster.GetName())
	}

	mgateway := &istioOperatorApi.IstioMeshGateway{
		ObjectMeta: templates.ObjectMeta(
			meshgatewayName,
			labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName, istioRevision), r.KafkaCluster),
		Spec: &istioOperatorApi.IstioMeshGatewaySpec{
			Deployment: &istioOperatorApi.BaseKubernetesResourceConfig{
				Metadata: &istioOperatorApi.K8SObjectMeta{
					Labels:      labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName, istioRevision),
					Annotations: ingressConfig.IstioIngressConfig.GetAnnotations(),
				},
				Env:          ingressConfig.IstioIngressConfig.Envs,
				Resources:    istioOperatorApi.InitResourceRequirementsFromK8sRR(ingressConfig.IstioIngressConfig.GetResources()),
				NodeSelector: ingressConfig.IstioIngressConfig.NodeSelector,
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:    util.Int64Pointer(0),
					RunAsGroup:   util.Int64Pointer(0),
					RunAsNonRoot: util.BoolPointer(false),
				},
				Tolerations: ingressConfig.IstioIngressConfig.Tolerations,
				Replicas: &istioOperatorApi.Replicas{
					Count: wrapperspb.Int32(ingressConfig.IstioIngressConfig.GetReplicas()),
					Min:   wrapperspb.Int32(ingressConfig.IstioIngressConfig.GetReplicas()),
					Max:   wrapperspb.Int32(ingressConfig.IstioIngressConfig.GetReplicas()),
				},
			},
			Service: &istioOperatorApi.Service{
				Metadata: &istioOperatorApi.K8SObjectMeta{
					Annotations: ingressConfig.GetServiceAnnotations(),
				},
				Ports: generateExternalPorts(r.KafkaCluster,
					util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log),
					externalListenerConfig, log, ingressConfigName, defaultIngressConfigName),
				Type:                     string(ingressConfig.GetServiceType()),
				LoadBalancerSourceRanges: ingressConfig.IstioIngressConfig.GetLoadBalancerSourceRanges(),
			},
			RunAsRoot: wrapperspb.Bool(true),
			Type:      istioOperatorApi.GatewayType_ingress,
			IstioControlPlane: &istioOperatorApi.NamespacedName{
				Name:      r.KafkaCluster.Spec.IstioControlPlane.Name,
				Namespace: r.KafkaCluster.Spec.IstioControlPlane.Namespace,
			},
		},
	}

	return mgateway
}

func generateExternalPorts(kc *v1beta1.KafkaCluster, brokerIds []int,
	externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger, ingressConfigName, defaultIngressConfigName string) []*istioOperatorApi.ServicePort {
	generatedPorts := make([]*istioOperatorApi.ServicePort, 0)
	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kc.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kc.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
			generatedPorts = append(generatedPorts, &istioOperatorApi.ServicePort{
				Name:       fmt.Sprintf("tcp-broker-%d", brokerId),
				Protocol:   string(corev1.ProtocolTCP),
				Port:       externalListenerConfig.GetBrokerPort(int32(brokerId)),
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(int(externalListenerConfig.GetBrokerPort(int32(brokerId))))},
			})
		}
	}

	generatedPorts = append(generatedPorts, &istioOperatorApi.ServicePort{
		Name:       fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, "tcp"),
		Protocol:   string(corev1.ProtocolTCP),
		Port:       externalListenerConfig.GetAnyCastPort(),
		TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(int(externalListenerConfig.GetAnyCastPort()))},
	})

	return generatedPorts
}
