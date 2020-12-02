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

package controllers

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"sync/atomic"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

var _ = Describe("KafkaClusterEnvoyController", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
	)

	ExpectEnvoyIngressLabels := func(labels map[string]string, eListenerName, crName string) {
		Expect(labels).To(HaveKeyWithValue("app", "envoyingress"))
		Expect(labels).To(HaveKeyWithValue("eListenerName", eListenerName))
		Expect(labels).To(HaveKeyWithValue("kafka_cr", crName))
	}

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-envoy-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		// create default Kafka cluster spec
		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%v", count)
		kafkaCluster = &v1beta1.KafkaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kafkaClusterCRName,
				Namespace: namespace,
			},
			Spec: v1beta1.KafkaClusterSpec{
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{},
					InternalListeners: []v1beta1.InternalListenerConfig{
						{
							CommonListenerSpec: v1beta1.CommonListenerSpec{
								Type:          "plaintext",
								Name:          "internal",
								ContainerPort: 29092,
							},
							UsedForInnerBrokerCommunication: true,
						},
						{
							CommonListenerSpec: v1beta1.CommonListenerSpec{
								Type:          "plaintext",
								Name:          "controller",
								ContainerPort: 29093,
							},
							UsedForInnerBrokerCommunication: false,
							UsedForControllerCommunication:  true,
						},
					},
				},
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{
								MountPath: "/kafka-logs",
								PvcSpec: &corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									Resources: corev1.ResourceRequirements{
										Requests: map[corev1.ResourceName]resource.Quantity{
											corev1.ResourceStorage: resource.MustParse("10Gi"),
										},
									},
								},
							},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                0,
						BrokerConfigGroup: "default",
					},
				},
				ClusterImage: "ghcr.io/banzaicloud/kafka:2.13-2.6.0-bzc.1",
				ZKAddresses:  []string{},
			},
		}
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
	})

	JustAfterEach(func() {
		By("deleting kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	When("envoy is not enabled", func() {
		It("does not create envoy related objects", func() {

		})
	})

	When("envoy is enabled", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.IngressController = "envoy"

			kafkaCluster.Spec.ListenersConfig.ExternalListeners = []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Name:          "test",
						ContainerPort: 9733,
					},
					ExternalStartingPort: 11202,
					HostnameOverride:     "test-host",
					AccessMethod:         corev1.ServiceTypeLoadBalancer,
					// ServiceAnnotations:   nil,
				},
			}
		})

		// TODO consider better tests with https://onsi.github.io/ginkgo/#patterns-for-dynamically-generating-tests

		It("creates envoy related objects", func() {
			var loadBalancer corev1.Service
			lbName := fmt.Sprintf("envoy-loadbalancer-test-%s", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: lbName}, &loadBalancer)
				return err
			}).Should(Succeed())

			ExpectEnvoyIngressLabels(loadBalancer.Labels, "test", kafkaClusterCRName)
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

			ExpectEnvoyIngressLabels(configMap.Labels, "test", kafkaClusterCRName)
			Expect(configMap.Data).To(HaveKey("envoy.yaml"))
			Expect(configMap.Data["envoy.yaml"]).To(Equal(`admin:
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
        address: kafkacluster-2-0.kafka-envoy-2.svc.cluster.local
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
`))

			var deployment appsv1.Deployment
			deploymentName := fmt.Sprintf("envoy-test-%s", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, &deployment)
				return err
			}).Should(Succeed())

			ExpectEnvoyIngressLabels(deployment.Labels, "test", kafkaClusterCRName)
			Expect(deployment.Spec.Selector).NotTo(BeNil())
			ExpectEnvoyIngressLabels(deployment.Spec.Selector.MatchLabels, "test", kafkaClusterCRName)
			Expect(deployment.Spec.Replicas).NotTo(BeNil())
			Expect(*deployment.Spec.Replicas).To(BeEquivalentTo(1))
			ExpectEnvoyIngressLabels(deployment.Spec.Template.Labels, "test", kafkaClusterCRName)
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
		})
	})
})
