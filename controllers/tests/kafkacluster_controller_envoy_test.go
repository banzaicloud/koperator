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

package tests

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

func expectEnvoyIngressLabels(labels map[string]string, eListenerName, crName string) {
	Expect(labels).To(HaveKeyWithValue("app", "envoyingress"))
	Expect(labels).To(HaveKeyWithValue("eListenerName", eListenerName))
	Expect(labels).To(HaveKeyWithValue("kafka_cr", crName))
}

func expectEnvoyLoadBalancer(kafkaCluster *v1beta1.KafkaCluster, eListenerName string) {
	var loadBalancer corev1.Service
	lbName := fmt.Sprintf("envoy-loadbalancer-%s-%s", eListenerName, kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: lbName}, &loadBalancer)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(loadBalancer.Labels, eListenerName, kafkaCluster.Name)
	Expect(loadBalancer.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
	Expect(loadBalancer.Spec.Selector).To(Equal(map[string]string{
		"app":           "envoyingress",
		"eListenerName": eListenerName,
		"kafka_cr":      kafkaCluster.Name,
	}))
	Expect(loadBalancer.Spec.Ports).To(HaveLen(4))
	for i, port := range loadBalancer.Spec.Ports {
		if i == 3 {
			break
		}
		Expect(port.Name).To(Equal(fmt.Sprintf("broker-%d", i)))
		Expect(port.Protocol).To(Equal(corev1.ProtocolTCP))
		Expect(port.Port).To(BeEquivalentTo(19090 + i))
		Expect(port.TargetPort.IntVal).To(BeEquivalentTo(19090 + i))
	}
	Expect(loadBalancer.Spec.Ports[3].Name).To(Equal("tcp-all-broker"))
	Expect(loadBalancer.Spec.Ports[3].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[3].Port).To(BeEquivalentTo(29092))
	Expect(loadBalancer.Spec.Ports[3].TargetPort.IntVal).To(BeEquivalentTo(29092))
}

func expectEnvoyConfigMap(kafkaCluster *v1beta1.KafkaCluster, eListenerName string) {
	var configMap corev1.ConfigMap
	configMapName := fmt.Sprintf("envoy-config-%s-%s", eListenerName, kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: configMapName}, &configMap)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(configMap.Labels, eListenerName, kafkaCluster.Name)
	Expect(configMap.Data).To(HaveKey("envoy.yaml"))
	svcTemplate := fmt.Sprintf("%s-%s.%s.svc.%s", kafkaCluster.Name, "%s", kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain())
	Expect(configMap.Data["envoy.yaml"]).To(Equal(fmt.Sprintf(`admin:
  accessLogPath: /tmp/admin_access.log
  address:
    socketAddress:
      address: 0.0.0.0
      portValue: 9901
staticResources:
  clusters:
  - connectTimeout: 1s
    hosts:
    - socketAddress:
        address: %s
        portValue: 9094
    http2ProtocolOptions: {}
    name: broker-0
    type: STRICT_DNS
  - connectTimeout: 1s
    hosts:
    - socketAddress:
        address: %s
        portValue: 9094
    http2ProtocolOptions: {}
    name: broker-1
    type: STRICT_DNS
  - connectTimeout: 1s
    hosts:
    - socketAddress:
        address: %s
        portValue: 9094
    http2ProtocolOptions: {}
    name: broker-2
    type: STRICT_DNS
  - connectTimeout: 1s
    hosts:
    - socketAddress:
        address: %s
        portValue: 9094
    http2ProtocolOptions: {}
    name: all-brokers
    type: STRICT_DNS
  listeners:
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19090
    filterChains:
    - filters:
      - config:
          cluster: broker-0
          stat_prefix: broker_tcp-0
        name: envoy.filters.network.tcp_proxy
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19091
    filterChains:
    - filters:
      - config:
          cluster: broker-1
          stat_prefix: broker_tcp-1
        name: envoy.filters.network.tcp_proxy
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19092
    filterChains:
    - filters:
      - config:
          cluster: broker-2
          stat_prefix: broker_tcp-2
        name: envoy.filters.network.tcp_proxy
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 29092
    filterChains:
    - filters:
      - config:
          cluster: all-brokers
          stat_prefix: all-brokers
        name: envoy.filters.network.tcp_proxy
`, fmt.Sprintf(svcTemplate, "0"), fmt.Sprintf(svcTemplate, "1"), fmt.Sprintf(svcTemplate, "2"), fmt.Sprintf(svcTemplate, "all-broker"))))
}

func expectEnvoyDeployment(kafkaCluster *v1beta1.KafkaCluster, eListenerName string) {
	var deployment appsv1.Deployment
	deploymentName := fmt.Sprintf("envoy-%s-%s", eListenerName, kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: deploymentName}, &deployment)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(deployment.Labels, eListenerName, kafkaCluster.Name)
	Expect(deployment.Spec.Selector).NotTo(BeNil())
	expectEnvoyIngressLabels(deployment.Spec.Selector.MatchLabels, eListenerName, kafkaCluster.Name)
	Expect(deployment.Spec.Replicas).NotTo(BeNil())
	Expect(*deployment.Spec.Replicas).To(BeEquivalentTo(1))
	expectEnvoyIngressLabels(deployment.Spec.Template.Labels, eListenerName, kafkaCluster.Name)
	templateSpec := deployment.Spec.Template.Spec
	Expect(templateSpec.ServiceAccountName).To(Equal("default"))
	Expect(templateSpec.Containers).To(HaveLen(1))
	container := templateSpec.Containers[0]
	Expect(container.Name).To(Equal("envoy"))
	Expect(container.Image).To(Equal("envoyproxy/envoy:v1.14.4"))
	Expect(container.Ports).To(ConsistOf(
		corev1.ContainerPort{
			Name:          "broker-0",
			ContainerPort: 19090,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "broker-1",
			ContainerPort: 19091,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "broker-2",
			ContainerPort: 19092,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "tcp-all-broker",
			ContainerPort: 29092,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "envoy-admin",
			ContainerPort: 9901,
			Protocol:      corev1.ProtocolTCP,
		},
	))
	Expect(container.VolumeMounts).To(ConsistOf(corev1.VolumeMount{
		Name:      fmt.Sprintf("envoy-config-%s-%s", eListenerName, kafkaCluster.Name),
		ReadOnly:  true,
		MountPath: "/etc/envoy",
	}))
	Expect(container.Resources).To(Equal(corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("100Mi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("100Mi"),
		},
	}))
}

func expectEnvoyWithoutConfig(kafkaCluster *v1beta1.KafkaCluster) {
	for _, eListener := range kafkaCluster.Spec.ListenersConfig.ExternalListeners {
		expectEnvoyLoadBalancer(kafkaCluster, eListener.Name)
		expectEnvoyConfigMap(kafkaCluster, eListener.Name)
		expectEnvoyDeployment(kafkaCluster, eListener.Name)
	}
}
