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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	istioOperatorApi "github.com/banzaicloud/istio-operator/pkg/apis/istio/v1beta1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	istioingressutils "github.com/banzaicloud/kafka-operator/pkg/util/istioingress"
)

func (r *Reconciler) meshgateway(log logr.Logger, externalListenerConfig v1beta1.ExternalListenerConfig) runtime.Object {
	return &istioOperatorApi.MeshGateway{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(istioingressutils.MeshGatewayNameTemplate, r.KafkaCluster.Name), labelsForIstioIngress(r.KafkaCluster.Name, externalListenerConfig.Name), r.KafkaCluster),
		Spec: istioOperatorApi.MeshGatewaySpec{
			MeshGatewayConfiguration: istioOperatorApi.MeshGatewayConfiguration{
				Labels: labelsForIstioIngress(r.KafkaCluster.Name, externalListenerConfig.Name),
				BaseK8sResourceConfigurationWithHPAWithoutImage: istioOperatorApi.BaseK8sResourceConfigurationWithHPAWithoutImage{
					ReplicaCount: util.Int32Pointer(1),
					MinReplicas:  util.Int32Pointer(1),
					MaxReplicas:  util.Int32Pointer(5),
					BaseK8sResourceConfiguration: istioOperatorApi.BaseK8sResourceConfiguration{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("2000m"),
								"memory": resource.MustParse("1024Mi"),
							},
						},
					},
				},
				ServiceType: corev1.ServiceTypeLoadBalancer,
			},
			Ports: generateExternalPorts(r.KafkaCluster.Spec, externalListenerConfig),
			Type:  istioOperatorApi.GatewayTypeIngress,
		},
	}
}

func generateExternalPorts(clusterSpec v1beta1.KafkaClusterSpec, externalListenerConfig v1beta1.ExternalListenerConfig) []corev1.ServicePort {
	generatedPorts := make([]corev1.ServicePort, 0)
	for _, broker := range clusterSpec.Brokers {
		generatedPorts = append(generatedPorts, corev1.ServicePort{
			Name: fmt.Sprintf("broker-%d", broker.Id),
			Port: externalListenerConfig.ExternalStartingPort + broker.Id,
		})
	}

	return generatedPorts
}
