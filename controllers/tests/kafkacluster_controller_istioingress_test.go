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
	"encoding/json"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/istio-client-go/pkg/networking/v1alpha3"
	istioOperatorApi "github.com/banzaicloud/istio-operator/pkg/apis/istio/v1beta1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/banzaicloud/kafka-operator/pkg/util/istioingress"
)

var _ = Describe("KafkaClusterIstioIngressController", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
	)

	ExpectIstioIngressLabels := func(labels map[string]string, eListenerName, crName string) {
		Expect(labels).To(HaveKeyWithValue("app", "istioingress"))
		Expect(labels).To(HaveKeyWithValue("eListenerName", eListenerName))
		Expect(labels).To(HaveKeyWithValue("kafka_cr", crName))
	}

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-istioingress-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%v", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)

		kafkaCluster.Spec.IngressController = istioingress.IngressControllerName
		kafkaCluster.Spec.ListenersConfig.ExternalListeners = []v1beta1.ExternalListenerConfig{
			{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9094,
				},
				ExternalStartingPort: 19090,
			},
		}
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		svcName := fmt.Sprintf("meshgateway-external-%s", kafkaCluster.Name)
		svcFromMeshGateway := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					// other ports omitted
					{
						Name:     "tcp-all-brokers",
						Port:     29092, // from MeshGateway (guarded by the tests)
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: "ingress.test.host.com",
						},
					},
				},
			},
		}
		err = k8sClient.Create(context.TODO(), &svcFromMeshGateway)
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Status().Update(context.TODO(), &svcFromMeshGateway)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	When("Istio ingress controller is configured", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.IngressController = istioingress.IngressControllerName
		})

		It("creates Istio ingress related objects", func() {
			var meshGateway istioOperatorApi.MeshGateway
			meshGatewayName := fmt.Sprintf("meshgateway-external-%s", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: meshGatewayName}, &meshGateway)
				return err
			}).Should(Succeed())

			meshGatewayConf := meshGateway.Spec.MeshGatewayConfiguration
			ExpectIstioIngressLabels(meshGatewayConf.Labels, "external", kafkaClusterCRName)
			Expect(meshGatewayConf.ServiceType).To(Equal(corev1.ServiceTypeLoadBalancer))
			baseConf := meshGatewayConf.BaseK8sResourceConfigurationWithHPAWithoutImage
			Expect(baseConf.ReplicaCount).To(Equal(util.Int32Pointer(1)))
			Expect(baseConf.MinReplicas).To(Equal(util.Int32Pointer(1)))
			Expect(baseConf.MaxReplicas).To(Equal(util.Int32Pointer(1)))

			actualResourceJSON, err := json.Marshal(baseConf.BaseK8sResourceConfiguration.Resources)
			Expect(err).NotTo(HaveOccurred())
			expectedResource := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("2000m"),
					"memory": resource.MustParse("1024Mi"),
				},
			}
			expectedResourceJSON, err := json.Marshal(expectedResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualResourceJSON).To(Equal(expectedResourceJSON))

			Expect(meshGateway.Spec.Ports).To(ConsistOf(
				corev1.ServicePort{
					Name:       "tcp-broker-0",
					Port:       19090,
					TargetPort: intstr.FromInt(19090),
				},
				corev1.ServicePort{
					Name:       "tcp-broker-1",
					Port:       19091,
					TargetPort: intstr.FromInt(19091),
				},
				corev1.ServicePort{
					Name:       "tcp-broker-2",
					Port:       19092,
					TargetPort: intstr.FromInt(19092),
				},
				corev1.ServicePort{
					Name:       "tcp-all-broker",
					Port:       29092,
					TargetPort: intstr.FromInt(29092),
				}))
			Expect(meshGateway.Spec.Type).To(Equal(istioOperatorApi.GatewayTypeIngress))

			var gateway v1alpha3.Gateway
			gatewayName := fmt.Sprintf("%s-external-gateway", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: gatewayName}, &gateway)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(gateway.Labels, "external", kafkaClusterCRName)
			ExpectIstioIngressLabels(gateway.Spec.Selector, "external", kafkaClusterCRName)
			Expect(gateway.Spec.Servers).To(ConsistOf(
				v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number:   19090,
						Protocol: "TCP",
						Name:     "tcp-broker-0"},
					Hosts: []string{"*"},
				},
				v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number:   19091,
						Protocol: "TCP",
						Name:     "tcp-broker-1"},
					Hosts: []string{"*"},
				},
				v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number:   19092,
						Protocol: "TCP",
						Name:     "tcp-broker-2"},
					Hosts: []string{"*"},
				},
				v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number:   29092,
						Protocol: "TCP",
						Name:     "tcp-all-broker",
					},
					Hosts: []string{"*"},
				}))

			var virtualService v1alpha3.VirtualService
			virtualServiceName := fmt.Sprintf("%s-external-virtualservice", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: virtualServiceName}, &virtualService)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(virtualService.Labels, "external", kafkaClusterCRName)
			Expect(virtualService.Spec).To(Equal(v1alpha3.VirtualServiceSpec{
				Hosts:    []string{"*"},
				Gateways: []string{fmt.Sprintf("%s-external-gateway", kafkaClusterCRName)},
				TCP: []v1alpha3.TCPRoute{
					{
						Match: []v1alpha3.L4MatchAttributes{{Port: util.IntPointer(19090)}},
						Route: []*v1alpha3.RouteDestination{{
							Destination: &v1alpha3.Destination{
								Host: "kafkacluster-1-0",
								Port: &v1alpha3.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []v1alpha3.L4MatchAttributes{{Port: util.IntPointer(19091)}},
						Route: []*v1alpha3.RouteDestination{{
							Destination: &v1alpha3.Destination{
								Host: "kafkacluster-1-1",
								Port: &v1alpha3.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []v1alpha3.L4MatchAttributes{{Port: util.IntPointer(19092)}},
						Route: []*v1alpha3.RouteDestination{{
							Destination: &v1alpha3.Destination{
								Host: "kafkacluster-1-2",
								Port: &v1alpha3.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []v1alpha3.L4MatchAttributes{{Port: util.IntPointer(29092)}},
						Route: []*v1alpha3.RouteDestination{{
							Destination: &v1alpha3.Destination{
								Host: "kafkacluster-1-all-broker",
								Port: &v1alpha3.PortSelector{Number: 9094},
							},
						}},
					},
				},
			}))

			// expect kafkaCluster listener status
			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {{
						Host: fmt.Sprintf("%s-all-broker.kafka-istioingress-%d.svc.cluster.local", kafkaCluster.Name, count),
						Port: 29092,
					}},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"external": {
						{
							Host: "ingress.test.host.com",
							Port: 19090,
						},
						{
							Host: "ingress.test.host.com",
							Port: 19091,
						},
						{
							Host: "ingress.test.host.com",
							Port: 19092,
						},
						{
							Host: "ingress.test.host.com",
							Port: 29092,
						},
					},
				},
			}))
		})
	})

	When("Headless mode is turned on", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.HeadlessServiceEnabled = true
		})

		It("does not add the all-broker service to the listener status", func() {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {{
						Host: fmt.Sprintf("%s-headless.kafka-istioingress-%d.svc.cluster.local", kafkaCluster.Name, count),
						Port: 29092,
					}},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"external": {
						{
							Host: "ingress.test.host.com",
							Port: 19090,
						},
						{
							Host: "ingress.test.host.com",
							Port: 19091,
						},
						{
							Host: "ingress.test.host.com",
							Port: 19092,
						},
					},
				},
			}))
		})
	})
})
