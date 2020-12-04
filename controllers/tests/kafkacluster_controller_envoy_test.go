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

func expectEnvoy(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	var loadBalancer corev1.Service
	lbName := fmt.Sprintf("envoy-loadbalancer-test-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: lbName}, &loadBalancer)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(loadBalancer.Labels, "test", kafkaCluster.Name)
	Expect(loadBalancer.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
	Expect(loadBalancer.Spec.Selector).To(Equal(map[string]string{
		"app":           "envoyingress",
		"eListenerName": "test",
		"kafka_cr":      kafkaCluster.Name,
	}))
	Expect(loadBalancer.Spec.Ports).To(HaveLen(1))
	Expect(loadBalancer.Spec.Ports[0].Name).To(Equal("broker-0"))
	Expect(loadBalancer.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[0].Port).To(BeEquivalentTo(11202))
	Expect(loadBalancer.Spec.Ports[0].TargetPort.IntVal).To(BeEquivalentTo(11202))

	var configMap corev1.ConfigMap
	configMapName := fmt.Sprintf("envoy-config-test-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: configMapName}, &configMap)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(configMap.Labels, "test", kafkaCluster.Name)
	Expect(configMap.Data).To(HaveKey("envoy.yaml"))
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
        address: %s-0.%s.svc.cluster.local
        portValue: 9733
    http2ProtocolOptions: {}
    name: broker-0
    type: STRICT_DNS
  listeners:
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 11202
    filterChains:
    - filters:
      - config:
          cluster: broker-0
          stat_prefix: broker_tcp-0
        name: envoy.filters.network.tcp_proxy
`, kafkaCluster.Name, namespace)))

	var deployment appsv1.Deployment
	deploymentName := fmt.Sprintf("envoy-test-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, &deployment)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(deployment.Labels, "test", kafkaCluster.Name)
	Expect(deployment.Spec.Selector).NotTo(BeNil())
	expectEnvoyIngressLabels(deployment.Spec.Selector.MatchLabels, "test", kafkaCluster.Name)
	Expect(deployment.Spec.Replicas).NotTo(BeNil())
	Expect(*deployment.Spec.Replicas).To(BeEquivalentTo(1))
	expectEnvoyIngressLabels(deployment.Spec.Template.Labels, "test", kafkaCluster.Name)
	templateSpec := deployment.Spec.Template.Spec
	Expect(templateSpec.ServiceAccountName).To(Equal("default"))
	Expect(templateSpec.Containers).To(HaveLen(1))
	container := templateSpec.Containers[0]
	Expect(container.Name).To(Equal("envoy"))
	Expect(container.Image).To(Equal("envoyproxy/envoy:v1.14.4"))
	Expect(container.Ports).To(ConsistOf(
		corev1.ContainerPort{
			Name:          "broker-0",
			ContainerPort: 11202,
			Protocol:      "TCP",
		},
		corev1.ContainerPort{
			Name:          "envoy-admin",
			ContainerPort: 9901,
			Protocol:      "TCP",
		},
	))
	Expect(container.VolumeMounts).To(ConsistOf(corev1.VolumeMount{
		Name:      configMapName,
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
