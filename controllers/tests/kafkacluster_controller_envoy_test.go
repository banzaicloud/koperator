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

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func expectEnvoyIngressLabels(labels map[string]string, eListenerName, crName string) {
	Expect(labels).To(HaveKeyWithValue("app", "envoyingress"))
	Expect(labels).To(HaveKeyWithValue("eListenerName", eListenerName))
	Expect(labels).To(HaveKeyWithValue("kafka_cr", crName))
}

func expectEnvoyIngressAnnotations(annotations map[string]string) {
	Expect(annotations).To(HaveKeyWithValue("envoy-annotation-key", "envoy-annotation-value"))
}

func expectEnvoyLoadBalancer(kafkaCluster *v1beta1.KafkaCluster, eListenerTemplate string) {
	var loadBalancer corev1.Service
	lbName := fmt.Sprintf("envoy-loadbalancer-%s-%s", eListenerTemplate, kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: lbName}, &loadBalancer)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(loadBalancer.Labels, eListenerTemplate, kafkaCluster.Name)
	Expect(loadBalancer.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
	Expect(loadBalancer.Spec.Selector).To(Equal(map[string]string{
		"app":           "envoyingress",
		"eListenerName": eListenerTemplate,
		"kafka_cr":      kafkaCluster.Name,
	}))
	Expect(loadBalancer.Spec.Ports).To(HaveLen(6))
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

	Expect(loadBalancer.Spec.Ports[4].Name).To(Equal("tcp-health"))
	Expect(loadBalancer.Spec.Ports[4].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[4].Port).To(BeEquivalentTo(v1beta1.DefaultEnvoyHealthCheckPort))
	Expect(loadBalancer.Spec.Ports[4].TargetPort.IntVal).To(BeEquivalentTo(v1beta1.DefaultEnvoyHealthCheckPort))

	Expect(loadBalancer.Spec.Ports[5].Name).To(Equal("tcp-admin"))
	Expect(loadBalancer.Spec.Ports[5].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[5].Port).To(BeEquivalentTo(v1beta1.DefaultEnvoyAdminPort))
	Expect(loadBalancer.Spec.Ports[5].TargetPort.IntVal).To(BeEquivalentTo(v1beta1.DefaultEnvoyAdminPort))
}

func expectEnvoyConfigMap(kafkaCluster *v1beta1.KafkaCluster, eListenerTemplate string) {
	var configMap corev1.ConfigMap
	configMapName := fmt.Sprintf("envoy-config-%s-%s", eListenerTemplate, kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: configMapName}, &configMap)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(configMap.Labels, eListenerTemplate, kafkaCluster.Name)
	Expect(configMap.Data).To(HaveKey("envoy.yaml"))
	svcTemplate := fmt.Sprintf("%s-%s.%s.svc.%s", kafkaCluster.Name, "%s", kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain())
	expected := fmt.Sprintf(`admin:
  address:
    socketAddress:
      address: 0.0.0.0
      portValue: 8081
staticResources:
  clusters:
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    loadAssignment:
      clusterName: broker-0
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
    name: broker-0
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    loadAssignment:
      clusterName: broker-1
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
    name: broker-1
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    loadAssignment:
      clusterName: broker-2
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
    name: broker-2
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    healthChecks:
    - eventLogPath: /dev/stdout
      healthyThreshold: 1
      httpHealthCheck:
        path: /-/healthy
      interval: 5s
      intervalJitter: 1s
      noTrafficInterval: 5s
      timeout: 1s
      unhealthyInterval: 2s
      unhealthyThreshold: 2
    ignoreHealthOnHostRemoval: true
    loadAssignment:
      clusterName: all-brokers
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
            healthCheckConfig:
              portValue: 9020
    name: all-brokers
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  listeners:
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19090
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: broker-0
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: broker_tcp-0
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19091
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: broker-1
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: broker_tcp-1
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19092
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: broker-2
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: broker_tcp-2
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 29092
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: all-brokers
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: all-brokers
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 8080
    filterChains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          accessLog:
          - name: envoy.access_loggers.stdout
            typedConfig:
              '@type': type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
          httpFilters:
          - name: envoy.filters.http.health_check
            typedConfig:
              '@type': type.googleapis.com/envoy.extensions.filters.http.health_check.v3.HealthCheck
              clusterMinHealthyPercentages:
                all-brokers:
                  value: 1
              headers:
              - exactMatch: /healthcheck
                name: :path
              passThroughMode: false
          - name: envoy.filters.http.router
          routeConfig:
            name: local
            virtualHosts:
            - domains:
              - '*'
              name: localhost
              routes:
              - match:
                  prefix: /
                redirect:
                  pathRedirect: /healthcheck
          statPrefix: all-brokers-healthcheck
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
`, fmt.Sprintf(svcTemplate, "0"), fmt.Sprintf(svcTemplate, "1"), fmt.Sprintf(svcTemplate, "2"), fmt.Sprintf(svcTemplate, "all-broker"))
	Expect(configMap.Data["envoy.yaml"]).To(Equal(expected))
}

func expectEnvoyDeployment(kafkaCluster *v1beta1.KafkaCluster, eListenerTemplate string) {
	var deployment appsv1.Deployment
	deploymentName := fmt.Sprintf("envoy-%s-%s", eListenerTemplate, kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: deploymentName}, &deployment)
		return err
	}).Should(Succeed())

	expectEnvoyIngressLabels(deployment.Labels, eListenerTemplate, kafkaCluster.Name)
	Expect(deployment.Spec.Selector).NotTo(BeNil())
	expectEnvoyIngressLabels(deployment.Spec.Selector.MatchLabels, eListenerTemplate, kafkaCluster.Name)
	Expect(deployment.Spec.Replicas).NotTo(BeNil())
	Expect(*deployment.Spec.Replicas).To(BeEquivalentTo(1))
	expectEnvoyIngressLabels(deployment.Spec.Template.Labels, eListenerTemplate, kafkaCluster.Name)
	expectEnvoyIngressAnnotations(deployment.Annotations)
	expectEnvoyIngressAnnotations(deployment.Spec.Template.Annotations)
	templateSpec := deployment.Spec.Template.Spec
	Expect(templateSpec.ServiceAccountName).To(Equal("default"))
	Expect(templateSpec.Containers).To(HaveLen(1))
	container := templateSpec.Containers[0]
	Expect(container.Name).To(Equal("envoy"))
	Expect(container.Image).To(Equal("envoyproxy/envoy:v1.18.3"))
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
			Name:          "tcp-admin",
			ContainerPort: 8081,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "tcp-health",
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
	))
	Expect(container.VolumeMounts).To(ConsistOf(corev1.VolumeMount{
		Name:      fmt.Sprintf("envoy-config-%s-%s", eListenerTemplate, kafkaCluster.Name),
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

func expectEnvoy(kafkaCluster *v1beta1.KafkaCluster, eListenerTemplates []string) {
	for _, eListenerName := range eListenerTemplates {
		expectEnvoyLoadBalancer(kafkaCluster, eListenerName)
		expectEnvoyConfigMap(kafkaCluster, eListenerName)
		expectEnvoyDeployment(kafkaCluster, eListenerName)
	}
}

func expectEnvoyWithConfigAz1(kafkaCluster *v1beta1.KafkaCluster) {
	var loadBalancer corev1.Service
	lbName := fmt.Sprintf("envoy-loadbalancer-test-az1-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: lbName}, &loadBalancer)
		return err
	}).Should(Succeed())
	Expect(loadBalancer.Spec.Ports).To(HaveLen(4))

	Expect(loadBalancer.Spec.Ports[0].Name).To(Equal("broker-0"))
	Expect(loadBalancer.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[0].Port).To(BeEquivalentTo(19090))
	Expect(loadBalancer.Spec.Ports[0].TargetPort.IntVal).To(BeEquivalentTo(19090))

	Expect(loadBalancer.Spec.Ports[1].Name).To(Equal("tcp-all-broker"))
	Expect(loadBalancer.Spec.Ports[1].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[1].Port).To(BeEquivalentTo(29092))
	Expect(loadBalancer.Spec.Ports[1].TargetPort.IntVal).To(BeEquivalentTo(29092))

	Expect(loadBalancer.Spec.Ports[2].Name).To(Equal("tcp-health"))
	Expect(loadBalancer.Spec.Ports[2].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[2].Port).To(BeEquivalentTo(v1beta1.DefaultEnvoyHealthCheckPort))
	Expect(loadBalancer.Spec.Ports[2].TargetPort.IntVal).To(BeEquivalentTo(v1beta1.DefaultEnvoyHealthCheckPort))

	Expect(loadBalancer.Spec.Ports[3].Name).To(Equal("tcp-admin"))
	Expect(loadBalancer.Spec.Ports[3].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[3].Port).To(BeEquivalentTo(v1beta1.DefaultEnvoyAdminPort))
	Expect(loadBalancer.Spec.Ports[3].TargetPort.IntVal).To(BeEquivalentTo(v1beta1.DefaultEnvoyAdminPort))

	var deployment appsv1.Deployment
	deploymentName := fmt.Sprintf("envoy-test-az1-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: deploymentName}, &deployment)
		return err
	}).Should(Succeed())
	templateSpec := deployment.Spec.Template.Spec
	Expect(templateSpec.Containers).To(HaveLen(1))
	container := templateSpec.Containers[0]
	Expect(container.Ports).To(ConsistOf(
		corev1.ContainerPort{
			Name:          "broker-0",
			ContainerPort: 19090,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "tcp-all-broker",
			ContainerPort: 29092,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "tcp-admin",
			ContainerPort: v1beta1.DefaultEnvoyAdminPort,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "tcp-health",
			ContainerPort: v1beta1.DefaultEnvoyHealthCheckPort,
			Protocol:      corev1.ProtocolTCP,
		},
	))

	var configMap corev1.ConfigMap
	configMapName := fmt.Sprintf("envoy-config-test-az1-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: configMapName}, &configMap)
		return err
	}).Should(Succeed())
	Expect(configMap.Data).To(HaveKey("envoy.yaml"))
	svcTemplate := fmt.Sprintf("%s-%s.%s.svc.%s", kafkaCluster.Name, "%s", kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain())
	expected := fmt.Sprintf(`admin:
  address:
    socketAddress:
      address: 0.0.0.0
      portValue: 8081
staticResources:
  clusters:
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    loadAssignment:
      clusterName: broker-0
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
    name: broker-0
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    healthChecks:
    - eventLogPath: /dev/stdout
      healthyThreshold: 1
      httpHealthCheck:
        path: /-/healthy
      interval: 5s
      intervalJitter: 1s
      noTrafficInterval: 5s
      timeout: 1s
      unhealthyInterval: 2s
      unhealthyThreshold: 2
    ignoreHealthOnHostRemoval: true
    loadAssignment:
      clusterName: all-brokers
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
            healthCheckConfig:
              portValue: 9020
    name: all-brokers
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  listeners:
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19090
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: broker-0
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: broker_tcp-0
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 29092
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: all-brokers
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: all-brokers
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 8080
    filterChains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          accessLog:
          - name: envoy.access_loggers.stdout
            typedConfig:
              '@type': type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
          httpFilters:
          - name: envoy.filters.http.health_check
            typedConfig:
              '@type': type.googleapis.com/envoy.extensions.filters.http.health_check.v3.HealthCheck
              clusterMinHealthyPercentages:
                all-brokers:
                  value: 1
              headers:
              - exactMatch: /healthcheck
                name: :path
              passThroughMode: false
          - name: envoy.filters.http.router
          routeConfig:
            name: local
            virtualHosts:
            - domains:
              - '*'
              name: localhost
              routes:
              - match:
                  prefix: /
                redirect:
                  pathRedirect: /healthcheck
          statPrefix: all-brokers-healthcheck
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
`, fmt.Sprintf(svcTemplate, "0"), fmt.Sprintf(svcTemplate, "all-broker"))
	Expect(configMap.Data["envoy.yaml"]).To(Equal(expected))
}

func expectEnvoyWithConfigAz2(kafkaCluster *v1beta1.KafkaCluster) {
	var loadBalancer corev1.Service
	lbName := fmt.Sprintf("envoy-loadbalancer-test-az2-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: lbName}, &loadBalancer)
		return err
	}).Should(Succeed())
	Expect(loadBalancer.Spec.Ports).To(HaveLen(5))
	Expect(loadBalancer.Spec.Ports[0].Name).To(Equal("broker-1"))
	Expect(loadBalancer.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[0].Port).To(BeEquivalentTo(19091))
	Expect(loadBalancer.Spec.Ports[0].TargetPort.IntVal).To(BeEquivalentTo(19091))

	Expect(loadBalancer.Spec.Ports[1].Name).To(Equal("broker-2"))
	Expect(loadBalancer.Spec.Ports[1].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[1].Port).To(BeEquivalentTo(19092))
	Expect(loadBalancer.Spec.Ports[1].TargetPort.IntVal).To(BeEquivalentTo(19092))

	Expect(loadBalancer.Spec.Ports[2].Name).To(Equal("tcp-all-broker"))
	Expect(loadBalancer.Spec.Ports[2].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[2].Port).To(BeEquivalentTo(29092))
	Expect(loadBalancer.Spec.Ports[2].TargetPort.IntVal).To(BeEquivalentTo(29092))

	Expect(loadBalancer.Spec.Ports[3].Name).To(Equal("tcp-health"))
	Expect(loadBalancer.Spec.Ports[3].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[3].Port).To(BeEquivalentTo(v1beta1.DefaultEnvoyHealthCheckPort))
	Expect(loadBalancer.Spec.Ports[3].TargetPort.IntVal).To(BeEquivalentTo(v1beta1.DefaultEnvoyHealthCheckPort))

	Expect(loadBalancer.Spec.Ports[4].Name).To(Equal("tcp-admin"))
	Expect(loadBalancer.Spec.Ports[4].Protocol).To(Equal(corev1.ProtocolTCP))
	Expect(loadBalancer.Spec.Ports[4].Port).To(BeEquivalentTo(v1beta1.DefaultEnvoyAdminPort))
	Expect(loadBalancer.Spec.Ports[4].TargetPort.IntVal).To(BeEquivalentTo(v1beta1.DefaultEnvoyAdminPort))

	var deployment appsv1.Deployment
	deploymentName := fmt.Sprintf("envoy-test-az2-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: deploymentName}, &deployment)
		return err
	}).Should(Succeed())
	templateSpec := deployment.Spec.Template.Spec
	Expect(templateSpec.Containers).To(HaveLen(1))
	container := templateSpec.Containers[0]
	Expect(container.Ports).To(ConsistOf(
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
			Name:          "tcp-admin",
			ContainerPort: v1beta1.DefaultEnvoyAdminPort,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			Name:          "tcp-health",
			ContainerPort: v1beta1.DefaultEnvoyHealthCheckPort,
			Protocol:      corev1.ProtocolTCP,
		},
	))

	var configMap corev1.ConfigMap
	configMapName := fmt.Sprintf("envoy-config-test-az2-%s", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: configMapName}, &configMap)
		return err
	}).Should(Succeed())
	Expect(configMap.Data).To(HaveKey("envoy.yaml"))
	svcTemplate := fmt.Sprintf("%s-%s.%s.svc.%s", kafkaCluster.Name, "%s", kafkaCluster.Namespace, kafkaCluster.Spec.GetKubernetesClusterDomain())
	expected := fmt.Sprintf(`admin:
  address:
    socketAddress:
      address: 0.0.0.0
      portValue: 8081
staticResources:
  clusters:
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    loadAssignment:
      clusterName: broker-1
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
    name: broker-1
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    loadAssignment:
      clusterName: broker-2
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
    name: broker-2
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  - circuitBreakers:
      thresholds:
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
      - maxConnections: 1000000000
        maxPendingRequests: 1000000000
        maxRequests: 1000000000
        maxRetries: 1000000000
        priority: HIGH
    connectTimeout: 1s
    healthChecks:
    - eventLogPath: /dev/stdout
      healthyThreshold: 1
      httpHealthCheck:
        path: /-/healthy
      interval: 5s
      intervalJitter: 1s
      noTrafficInterval: 5s
      timeout: 1s
      unhealthyInterval: 2s
      unhealthyThreshold: 2
    ignoreHealthOnHostRemoval: true
    loadAssignment:
      clusterName: all-brokers
      endpoints:
      - lbEndpoints:
        - endpoint:
            address:
              socketAddress:
                address: %s
                portValue: 9094
            healthCheckConfig:
              portValue: 9020
    name: all-brokers
    type: STRICT_DNS
    upstreamConnectionOptions:
      tcpKeepalive:
        keepaliveInterval: 30
        keepaliveProbes: 3
        keepaliveTime: 30
  listeners:
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19091
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: broker-1
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: broker_tcp-1
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 19092
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: broker-2
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: broker_tcp-2
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 29092
    filterChains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          cluster: all-brokers
          idleTimeout: 560s
          maxConnectAttempts: 2
          statPrefix: all-brokers
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
  - address:
      socketAddress:
        address: 0.0.0.0
        portValue: 8080
    filterChains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typedConfig:
          '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          accessLog:
          - name: envoy.access_loggers.stdout
            typedConfig:
              '@type': type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
          httpFilters:
          - name: envoy.filters.http.health_check
            typedConfig:
              '@type': type.googleapis.com/envoy.extensions.filters.http.health_check.v3.HealthCheck
              clusterMinHealthyPercentages:
                all-brokers:
                  value: 1
              headers:
              - exactMatch: /healthcheck
                name: :path
              passThroughMode: false
          - name: envoy.filters.http.router
          routeConfig:
            name: local
            virtualHosts:
            - domains:
              - '*'
              name: localhost
              routes:
              - match:
                  prefix: /
                redirect:
                  pathRedirect: /healthcheck
          statPrefix: all-brokers-healthcheck
    socketOptions:
    - intValue: "1"
      level: "1"
      name: "9"
    - intValue: "30"
      level: "6"
      name: "4"
    - intValue: "30"
      level: "6"
      name: "5"
    - intValue: "3"
      level: "6"
      name: "6"
`, fmt.Sprintf(svcTemplate, "1"), fmt.Sprintf(svcTemplate, "2"), fmt.Sprintf(svcTemplate, "all-broker"))
	Expect(configMap.Data["envoy.yaml"]).To(Equal(expected))
}
