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

	istioOperatorApi "github.com/banzaicloud/istio-operator/api/v2/v1alpha1"

	"github.com/go-logr/logr"
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
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, externalListenerConfig.Name)

	var meshgatewayName string
	if ingressConfigName == util.IngressConfigGlobalName {
		meshgatewayName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplate, externalListenerConfig.Name, r.KafkaCluster.GetName())
	} else {
		meshgatewayName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplateWithScope,
			externalListenerConfig.Name, ingressConfigName, r.KafkaCluster.GetName())
	}

	ingressResources := ingressConfig.IstioIngressConfig.GetResources()

	mgateway := &istioOperatorApi.IstioMeshGateway{
		ObjectMeta: templates.ObjectMeta(
			meshgatewayName,
			labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName), r.KafkaCluster),
		Spec: &istioOperatorApi.IstioMeshGatewaySpec{
			Deployment: &istioOperatorApi.BaseKubernetesResourceConfig{
				Metadata: &istioOperatorApi.K8SObjectMeta{
					Labels:      labelsForIstioIngress(r.KafkaCluster.Name, eListenerLabelName),
					Annotations: ingressConfig.IstioIngressConfig.GetAnnotations(),
				},
				//Image:                "",
				Env: ingressConfig.IstioIngressConfig.Envs,
				Resources: &istioOperatorApi.ResourceRequirements{
					Limits: map[string]*istioOperatorApi.Quantity{
						"cpu":    {Quantity: *ingressResources.Limits.Cpu()},
						"memory": {Quantity: *ingressResources.Limits.Memory()},
					},
					Requests: map[string]*istioOperatorApi.Quantity{
						"cpu":    {Quantity: *ingressResources.Requests.Cpu()},
						"memory": {Quantity: *ingressResources.Requests.Memory()},
					},
				},
				NodeSelector: ingressConfig.IstioIngressConfig.NodeSelector,
				//	Affinity:             &corev1.Affinity{},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:    util.Int64Pointer(0),
					RunAsGroup:   util.Int64Pointer(0),
					RunAsNonRoot: util.BoolPointer(false),
				},
				//	ImagePullPolicy:      "",
				//	ImagePullSecrets:     []corev1.LocalObjectReference{},
				//	PriorityClassName:    "",
				Tolerations: ingressConfig.IstioIngressConfig.Tolerations,
				//	Volumes:              []corev1.Volume{},
				//	VolumeMounts:         []corev1.VolumeMount{},
				Replicas: &istioOperatorApi.Replicas{
					Count: util.Int32Pointer(ingressConfig.IstioIngressConfig.GetReplicas()),
					Min:   util.Int32Pointer(ingressConfig.IstioIngressConfig.GetReplicas()),
					Max:   util.Int32Pointer(ingressConfig.IstioIngressConfig.GetReplicas()),
					//		TargetCPUUtilizationPercentage: new(int32),
				},
				// PodMetadata:          &istioOperatorApi.K8SObjectMeta{},
				// PodDisruptionBudget:  &istioOperatorApi.PodDisruptionBudget{},
				// DeploymentStrategy:   &istioOperatorApi.DeploymentStrategy{},
				// PodSecurityContext:   &corev1.PodSecurityContext{},
				// LivenessProbe:        &corev1.Probe{},
				// ReadinessProbe:       &corev1.Probe{},
			},
			Service: &istioOperatorApi.Service{
				Metadata: &istioOperatorApi.K8SObjectMeta{},
				Ports: generateExternalPorts(r.KafkaCluster,
					util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log),
					externalListenerConfig, log, ingressConfigName, defaultIngressConfigName),
				// Selector:                 map[string]string{},
				// ClusterIP:                "",
				Type: string(ingressConfig.GetServiceType()),
				// ExternalIPs:              []string{},
				// SessionAffinity:          "",
				// LoadBalancerIP:           "",
				// LoadBalancerSourceRanges: []string{},
				// ExternalName:             "",
				// ExternalTrafficPolicy:    string(ingressConfig.ExternalTrafficPolicy),
				// HealthCheckNodePort:      0,
				// PublishNotReadyAddresses: new(bool),
				// SessionAffinityConfig:    &corev1.SessionAffinityConfig{},
				// IpFamily:                 "",
			},
			RunAsRoot: util.BoolPointer(true),
			Type:      istioOperatorApi.GatewayType_ingress,
			IstioControlPlane: &istioOperatorApi.NamespacedName{
				Name:      "icp-v111x-sample",
				Namespace: "istio-system",
			},
			//	K8SResourceOverlays:  []*istioOperatorApi.K8SResourceOverlayPatch{},
		},
	}

	return mgateway
}

func generateExternalPorts(kc *v1beta1.KafkaCluster, brokerIds []int,
	externalListenerConfig v1beta1.ExternalListenerConfig, log logr.Logger, ingressConfigName, defaultIngressConfigName string) []istioOperatorApi.ServicePort {
	generatedPorts := make([]istioOperatorApi.ServicePort, 0)
	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kc.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kc.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
			generatedPorts = append(generatedPorts, istioOperatorApi.ServicePort{
				Name:     fmt.Sprintf("tcp-broker-%d", brokerId),
				Protocol: "TCP",
				Port:     externalListenerConfig.ExternalStartingPort + int32(brokerId),
				TargetPort: &istioOperatorApi.IntOrString{
					IntOrString: intstr.IntOrString{
						Type:   0,
						IntVal: int32(int(externalListenerConfig.ExternalStartingPort) + brokerId),
					},
				},
			})
		}
	}

	generatedPorts = append(generatedPorts, istioOperatorApi.ServicePort{
		Name:     fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, "tcp"),
		Protocol: "TCP",
		Port:     externalListenerConfig.GetAnyCastPort(),
		TargetPort: &istioOperatorApi.IntOrString{
			IntOrString: intstr.IntOrString{
				Type:   0,
				IntVal: int32(int(externalListenerConfig.GetAnyCastPort())),
			},
		},
	})

	return generatedPorts
}
