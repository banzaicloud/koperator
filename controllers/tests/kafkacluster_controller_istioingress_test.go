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

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"google.golang.org/protobuf/types/known/wrapperspb"

	istioclientv1beta1 "github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"

	istioOperatorApi "github.com/banzaicloud/istio-operator/api/v2/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/util/istioingress"
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
		Expect(labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "istioingress"))
		Expect(labels).To(HaveKeyWithValue("eListenerName", eListenerName))
		Expect(labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, crName))
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
		kafkaCluster.Spec.IstioControlPlane = &v1beta1.IstioControlPlaneReference{Name: "icp-v115x-sample", Namespace: "istio-system"}
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

	JustBeforeEach(func(ctx SpecContext) {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(ctx, kafkaCluster)
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
						Name:     "tcp-all-broker",
						Port:     29092, // from MeshGateway (guarded by the tests)
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}
		err = k8sClient.Create(ctx, &svcFromMeshGateway)
		Expect(err).NotTo(HaveOccurred())
		svcFromMeshGateway.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: "ingress.test.host.com"}}
		err = k8sClient.Status().Update(ctx, &svcFromMeshGateway)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(ctx, kafkaCluster, namespace)
	})

	JustAfterEach(func(ctx SpecContext) {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	When("Istio ingress controller is configured", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.IngressController = istioingress.IngressControllerName
		})

		It("creates Istio ingress related objects", func(ctx SpecContext) {
			var meshGateway istioOperatorApi.IstioMeshGateway
			meshGatewayName := fmt.Sprintf("meshgateway-external-%s", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: meshGatewayName}, &meshGateway)
				return err
			}).Should(Succeed())

			meshGatewaySpec := meshGateway.Spec
			ExpectIstioIngressLabels(meshGatewaySpec.Deployment.Metadata.Labels, "external", kafkaClusterCRName)
			Expect(meshGatewaySpec.Service.Type).To(Equal(string(corev1.ServiceTypeLoadBalancer)))
			deploymentConf := meshGatewaySpec.Deployment

			Expect(cmp.Equal(deploymentConf.Replicas.Count, wrapperspb.Int32(1), cmpopts.IgnoreUnexported(wrapperspb.Int32Value{}))).To(BeTrue())
			Expect(cmp.Equal(deploymentConf.Replicas.Min, wrapperspb.Int32(1), cmpopts.IgnoreUnexported(wrapperspb.Int32Value{}))).To(BeTrue())
			Expect(cmp.Equal(deploymentConf.Replicas.Max, wrapperspb.Int32(1), cmpopts.IgnoreUnexported(wrapperspb.Int32Value{}))).To(BeTrue())

			actualResourceJSON, err := json.Marshal(deploymentConf.Resources)
			Expect(err).NotTo(HaveOccurred())
			expectedResource := &istioOperatorApi.ResourceRequirements{
				Limits: map[string]*istioOperatorApi.Quantity{
					"cpu":    {Quantity: resource.MustParse("2000m")},
					"memory": {Quantity: resource.MustParse("1024Mi")},
				},
				Requests: map[string]*istioOperatorApi.Quantity{
					"cpu":    {Quantity: resource.MustParse("100m")},
					"memory": {Quantity: resource.MustParse("128Mi")},
				},
			}
			expectedResourceJSON, err := json.Marshal(expectedResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualResourceJSON).To(Equal(expectedResourceJSON))

			Expect(len(meshGatewaySpec.Service.Ports)).To(Equal(4))

			expectedPort := &istioOperatorApi.ServicePort{
				Name:       "tcp-broker-0",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       19090,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(19090)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[0], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())

			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-broker-1",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       19091,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(19091)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[1], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())
			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-broker-2",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       19092,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(19092)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[2], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())
			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-all-broker",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       29092,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(29092)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[3], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())

			Expect(meshGatewaySpec.Type).To(Equal(istioOperatorApi.GatewayType_ingress))

			var gateway istioclientv1beta1.Gateway
			gatewayName := fmt.Sprintf("%s-external-gateway", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: gatewayName}, &gateway)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(gateway.Labels, "external", kafkaClusterCRName)
			ExpectIstioIngressLabels(gateway.Spec.Selector, "external", kafkaClusterCRName)
			Expect(gateway.Spec.Servers).To(ConsistOf(
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   19090,
						Protocol: "TCP",
						Name:     "tcp-broker-0"},
					Hosts: []string{"*"},
				},
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   19091,
						Protocol: "TCP",
						Name:     "tcp-broker-1"},
					Hosts: []string{"*"},
				},
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   19092,
						Protocol: "TCP",
						Name:     "tcp-broker-2"},
					Hosts: []string{"*"},
				},
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   29092,
						Protocol: "TCP",
						Name:     "tcp-all-broker",
					},
					Hosts: []string{"*"},
				}))

			var virtualService istioclientv1beta1.VirtualService
			virtualServiceName := fmt.Sprintf("%s-external-virtualservice", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: virtualServiceName}, &virtualService)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(virtualService.Labels, "external", kafkaClusterCRName)
			Expect(virtualService.Spec).To(Equal(istioclientv1beta1.VirtualServiceSpec{
				Hosts:    []string{"*"},
				Gateways: []string{fmt.Sprintf("%s-external-gateway", kafkaClusterCRName)},
				TCP: []istioclientv1beta1.TCPRoute{
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(19090)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-0",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(19091)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-1",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(19092)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-2",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(29092)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-all-broker",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
				},
			}))

			// expect kafkaCluster listener status
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {
						{
							Name:    "any-broker",
							Address: fmt.Sprintf("%s-all-broker.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-0",
							Address: fmt.Sprintf("%s-0.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-1",
							Address: fmt.Sprintf("%s-1.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-2",
							Address: fmt.Sprintf("%s-2.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
					},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"external": {
						{
							Name:    "any-broker",
							Address: "ingress.test.host.com:29092",
						},
						{
							Name:    "broker-0",
							Address: "ingress.test.host.com:19090",
						},
						{
							Name:    "broker-1",
							Address: "ingress.test.host.com:19091",
						},
						{
							Name:    "broker-2",
							Address: "ingress.test.host.com:19092",
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

		It("does not add the all-broker service to the listener status", func(ctx SpecContext) {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {
						{
							Name:    "headless",
							Address: fmt.Sprintf("%s-headless.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-0",
							Address: fmt.Sprintf("%s-0.%s-headless.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, kafkaCluster.Name, count),
						},
						{
							Name:    "broker-1",
							Address: fmt.Sprintf("%s-1.%s-headless.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, kafkaCluster.Name, count),
						},
						{
							Name:    "broker-2",
							Address: fmt.Sprintf("%s-2.%s-headless.kafka-istioingress-%d.svc.cluster.local:29092", kafkaCluster.Name, kafkaCluster.Name, count),
						},
					},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"external": {
						{
							Name:    "any-broker",
							Address: "ingress.test.host.com:29092",
						},
						{
							Name:    "broker-0",
							Address: "ingress.test.host.com:19090",
						},
						{
							Name:    "broker-1",
							Address: "ingress.test.host.com:19091",
						},
						{
							Name:    "broker-2",
							Address: "ingress.test.host.com:19092",
						},
					},
				},
			}))
		})
	})
})

var _ = Describe("KafkaClusterIstioIngressControllerWithBrokerIdBindings", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
	)

	ExpectIstioIngressLabels := func(labels map[string]string, eListenerName, crName string) {
		Expect(labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "istioingress"))
		Expect(labels).To(HaveKeyWithValue("eListenerName", eListenerName))
		Expect(labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, crName))
	}

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-istioingress-with-bindings-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%v", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)

		kafkaCluster.Spec.IngressController = istioingress.IngressControllerName
		kafkaCluster.Spec.IstioControlPlane = &v1beta1.IstioControlPlaneReference{Name: "icp-v115x-sample", Namespace: "istio-system"}
		kafkaCluster.Spec.ListenersConfig.ExternalListeners = []v1beta1.ExternalListenerConfig{
			{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9094,
				},
				ExternalStartingPort: 19090,
				Config: &v1beta1.Config{
					DefaultIngressConfig: "az1",
					IngressConfig: map[string]v1beta1.IngressConfig{
						"az1": {IstioIngressConfig: &v1beta1.IstioIngressConfig{
							Annotations: map[string]string{"zone": "az1"},
						},
						},
						"az2": {IstioIngressConfig: &v1beta1.IstioIngressConfig{
							Annotations: map[string]string{"zone": "az2"},
							TLSOptions: &istioclientv1beta1.TLSOptions{
								Mode:           istioclientv1beta1.TLSModeSimple,
								CredentialName: util.StringPointer("foobar"),
							},
						},
						},
					},
				},
			},
		}
		kafkaCluster.Spec.Brokers[0].BrokerConfig = &v1beta1.BrokerConfig{BrokerSpecificConfig: v1beta1.BrokerSpecificConfig{BrokerIngressMapping: []string{"az1"}}}
		kafkaCluster.Spec.Brokers[1].BrokerConfig = &v1beta1.BrokerConfig{BrokerSpecificConfig: v1beta1.BrokerSpecificConfig{BrokerIngressMapping: []string{"az2"}}}
	})

	JustBeforeEach(func(ctx SpecContext) {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		createMeshGatewayService(ctx, "external.az1.host.com",
			fmt.Sprintf("meshgateway-external-az1-%s", kafkaCluster.Name), namespace)
		createMeshGatewayService(ctx, "external.az2.host.com",
			fmt.Sprintf("meshgateway-external-az2-%s", kafkaCluster.Name), namespace)

		waitForClusterRunningState(ctx, kafkaCluster, namespace)
	})

	JustAfterEach(func(ctx SpecContext) {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	When("Istio ingress controller is configured", func() {

		It("creates Istio ingress related objects", func(ctx SpecContext) {
			// Istio ingress Az1 related objects
			var meshGateway istioOperatorApi.IstioMeshGateway
			meshGatewayAz1Name := fmt.Sprintf("meshgateway-external-az1-%s", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: meshGatewayAz1Name}, &meshGateway)
				return err
			}).Should(Succeed())

			meshGatewaySpec := meshGateway.Spec
			ExpectIstioIngressLabels(meshGatewaySpec.Deployment.Metadata.Labels, "external-az1", kafkaClusterCRName)

			Expect(len(meshGatewaySpec.Service.Ports)).To(Equal(3))

			expectedPort := &istioOperatorApi.ServicePort{
				Name:       "tcp-broker-0",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       19090,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(19090)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[0], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())

			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-broker-2",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       19092,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(19092)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[1], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())
			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-all-broker",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       29092,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(29092)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[2], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())

			var gateway istioclientv1beta1.Gateway
			gatewayName := fmt.Sprintf("%s-external-az1-gateway", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: gatewayName}, &gateway)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(gateway.Labels, "external-az1", kafkaClusterCRName)
			ExpectIstioIngressLabels(gateway.Spec.Selector, "external-az1", kafkaClusterCRName)
			Expect(gateway.Spec.Servers).To(ConsistOf(
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   19090,
						Protocol: "TCP",
						Name:     "tcp-broker-0"},
					Hosts: []string{"*"},
				},
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   19092,
						Protocol: "TCP",
						Name:     "tcp-broker-2"},
					Hosts: []string{"*"},
				},
				istioclientv1beta1.Server{
					Port: &istioclientv1beta1.Port{
						Number:   29092,
						Protocol: "TCP",
						Name:     "tcp-all-broker",
					},
					Hosts: []string{"*"},
				}))

			var virtualService istioclientv1beta1.VirtualService
			virtualServiceName := fmt.Sprintf("%s-external-az1-virtualservice", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: virtualServiceName}, &virtualService)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(virtualService.Labels, "external-az1", kafkaClusterCRName)
			Expect(virtualService.Spec).To(Equal(istioclientv1beta1.VirtualServiceSpec{
				Hosts:    []string{"*"},
				Gateways: []string{gatewayName},
				TCP: []istioclientv1beta1.TCPRoute{
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(19090)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-0",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(19092)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-2",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(29092)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-all-broker",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
				},
			}))
			// Istio Ingress Az2 related objects
			meshGatewayAz2Name := fmt.Sprintf("meshgateway-external-az2-%s", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: meshGatewayAz2Name}, &meshGateway)
				return err
			}).Should(Succeed())

			meshGatewaySpec = meshGateway.Spec
			ExpectIstioIngressLabels(meshGatewaySpec.Deployment.Metadata.Labels, "external-az2", kafkaClusterCRName)

			Expect(len(meshGatewaySpec.Service.Ports)).To(Equal(2))

			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-broker-1",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       19091,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(19091)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[0], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())

			expectedPort = &istioOperatorApi.ServicePort{
				Name:       "tcp-all-broker",
				Protocol:   string(corev1.ProtocolTCP),
				Port:       29092,
				TargetPort: &istioOperatorApi.IntOrString{IntOrString: intstr.FromInt(29092)},
			}
			Expect(cmp.Equal(meshGatewaySpec.Service.Ports[1], expectedPort, cmpopts.IgnoreUnexported(istioOperatorApi.ServicePort{}))).To(BeTrue())

			gatewayName = fmt.Sprintf("%s-external-az2-gateway", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: gatewayName}, &gateway)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(gateway.Labels, "external-az2", kafkaClusterCRName)
			ExpectIstioIngressLabels(gateway.Spec.Selector, "external-az2", kafkaClusterCRName)
			Expect(gateway.Spec.Servers).To(ConsistOf(
				istioclientv1beta1.Server{
					TLS: &istioclientv1beta1.TLSOptions{
						Mode:           istioclientv1beta1.TLSModeSimple,
						CredentialName: util.StringPointer("foobar"),
					},
					Port: &istioclientv1beta1.Port{
						Number:   19091,
						Protocol: "TLS",
						Name:     "tcp-broker-1"},
					Hosts: []string{"*"},
				},
				istioclientv1beta1.Server{
					TLS: &istioclientv1beta1.TLSOptions{
						Mode:           istioclientv1beta1.TLSModeSimple,
						CredentialName: util.StringPointer("foobar"),
					},
					Port: &istioclientv1beta1.Port{
						Number:   29092,
						Protocol: "TLS",
						Name:     "tcp-all-broker",
					},
					Hosts: []string{"*"},
				}))

			virtualServiceName = fmt.Sprintf("%s-external-az2-virtualservice", kafkaCluster.Name)
			Eventually(ctx, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: virtualServiceName}, &virtualService)
				return err
			}).Should(Succeed())

			ExpectIstioIngressLabels(virtualService.Labels, "external-az2", kafkaClusterCRName)
			Expect(virtualService.Spec).To(Equal(istioclientv1beta1.VirtualServiceSpec{
				Hosts:    []string{"*"},
				Gateways: []string{gatewayName},
				TCP: []istioclientv1beta1.TCPRoute{
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(19091)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-1",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
					{
						Match: []istioclientv1beta1.L4MatchAttributes{{Port: util.IntPointer(29092)}},
						Route: []*istioclientv1beta1.RouteDestination{{
							Destination: &istioclientv1beta1.Destination{
								Host: "kafkacluster-1-all-broker",
								Port: &istioclientv1beta1.PortSelector{Number: 9094},
							},
						}},
					},
				},
			}))

			// expect kafkaCluster listener status
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {
						{
							Name:    "any-broker",
							Address: fmt.Sprintf("%s-all-broker.kafka-istioingress-with-bindings-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-0",
							Address: fmt.Sprintf("%s-0.kafka-istioingress-with-bindings-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-1",
							Address: fmt.Sprintf("%s-1.kafka-istioingress-with-bindings-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-2",
							Address: fmt.Sprintf("%s-2.kafka-istioingress-with-bindings-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
					},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"external": {
						{
							Name:    "any-broker-az1",
							Address: "external.az1.host.com:29092",
						},
						{
							Name:    "any-broker-az2",
							Address: "external.az2.host.com:29092",
						},
						{
							Name:    "broker-0",
							Address: "external.az1.host.com:19090",
						},
						{
							Name:    "broker-1",
							Address: "external.az2.host.com:19091",
						},
						{
							Name:    "broker-2",
							Address: "external.az1.host.com:19092",
						},
					},
				},
			}))
		})
	})
})

func createMeshGatewayService(ctx context.Context, extListenerName, extListenerServiceName, namespace string) {
	svcFromMeshGateway := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extListenerServiceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				// other ports omitted
				{
					Name:     "tcp-all-broker",
					Port:     29092, // from MeshGateway (guarded by the tests)
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
	err := k8sClient.Create(ctx, &svcFromMeshGateway)
	Expect(err).NotTo(HaveOccurred())
	svcFromMeshGateway.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: extListenerName}}
	err = k8sClient.Status().Update(ctx, &svcFromMeshGateway)
	Expect(err).NotTo(HaveOccurred())
}
