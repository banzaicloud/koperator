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
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

var (
	defaultStorageConfig = []v1beta1.StorageConfig{
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
	}
)

var _ = Describe("KafkaClusterNodeportExternalAccess", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-nodeport-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		// create default Kafka cluster spec
		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%v", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)

	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	When("NodePortExternalIP is configured", func() {
		BeforeEach(func() {
			// update the external listener config with a nodeport listener
			kafkaCluster.Spec.ListenersConfig.ExternalListeners = []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Name:          "test",
						ContainerPort: 9733,
					},
					ExternalStartingPort: 31123,
					AccessMethod:         corev1.ServiceTypeNodePort,
				},
			}
			kafkaCluster.Spec.BrokerConfigGroups = map[string]v1beta1.BrokerConfig{
				"br-0": {
					StorageConfigs: defaultStorageConfig,
					NodePortExternalIP: map[string]string{
						"test": "1.2.3.4",
					},
				},
				"br-1": {
					StorageConfigs: defaultStorageConfig,
					NodePortExternalIP: map[string]string{
						"test": "1.2.3.5",
					},
				},
				"br-2": {
					StorageConfigs: defaultStorageConfig,
					NodePortExternalIP: map[string]string{
						"test": "1.2.3.6",
					},
				},
			}
			kafkaCluster.Spec.Brokers = []v1beta1.Broker{
				{
					Id:                0,
					BrokerConfigGroup: "br-0",
				},
				{
					Id:                1,
					BrokerConfigGroup: "br-1",
				},
				{
					Id:                2,
					BrokerConfigGroup: "br-2",
				},
			}
		})

		It("reconciles the service successfully", func() {
			var svc corev1.Service
			svcName := fmt.Sprintf("%s-0-test", kafkaClusterCRName)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: svcName}, &svc)
				return err
			}).Should(Succeed())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":      "kafka",
				"brokerId": "0",
				"kafka_cr": kafkaCluster.Name,
			}))

			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
			Expect(svc.Spec.Selector).To(Equal(map[string]string{
				"app":      "kafka",
				"brokerId": "0",
				"kafka_cr": kafkaCluster.Name,
			}))

			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Name).To(Equal("broker-0"))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(svc.Spec.Ports[0].Port).To(BeEquivalentTo(9733))
			Expect(svc.Spec.Ports[0].TargetPort.IntVal).To(BeEquivalentTo(9733))

			Expect(svc.Spec.Ports).To(ConsistOf(corev1.ServicePort{
				Name:       "broker-0",
				Protocol:   "TCP",
				Port:       9733,
				TargetPort: intstr.FromInt(9733),
				NodePort:   31123,
			}))

			// check status
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {
						{
							Name:    "any-broker",
							Address: fmt.Sprintf("%s-all-broker.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-0",
							Address: fmt.Sprintf("%s-0.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-1",
							Address: fmt.Sprintf("%s-1.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-2",
							Address: fmt.Sprintf("%s-2.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
					},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"test": {
						{
							Name:    "broker-0",
							Address: "1.2.3.4:9733",
						},
						{
							Name:    "broker-1",
							Address: "1.2.3.5:9733",
						},
						{
							Name:    "broker-2",
							Address: "1.2.3.6:9733",
						},
					},
				},
			}))
		})
	})

	When("hostnameOverride is configured", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.ListenersConfig.ExternalListeners = []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Name:          "test",
						ContainerPort: 9733,
					},
					ExternalStartingPort: 30100,
					IngressServiceSettings: v1beta1.IngressServiceSettings{
						HostnameOverride: ".external.nodeport.com",
					},
					AccessMethod: corev1.ServiceTypeNodePort,
				},
			}
		})

		It("reconciles the status successfully", func() {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
				InternalListeners: map[string]v1beta1.ListenerStatusList{
					"internal": {
						{
							Name:    "any-broker",
							Address: fmt.Sprintf("%s-all-broker.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-0",
							Address: fmt.Sprintf("%s-0.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-1",
							Address: fmt.Sprintf("%s-1.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-2",
							Address: fmt.Sprintf("%s-2.kafka-nodeport-%d.svc.cluster.local:29092", kafkaCluster.Name, count),
						},
					},
				},
				ExternalListeners: map[string]v1beta1.ListenerStatusList{
					"test": {
						{
							Name:    "broker-0",
							Address: fmt.Sprintf("%s-0-test.kafka-nodeport-%d.external.nodeport.com:30100", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-1",
							Address: fmt.Sprintf("%s-1-test.kafka-nodeport-%d.external.nodeport.com:30101", kafkaCluster.Name, count),
						},
						{
							Name:    "broker-2",
							Address: fmt.Sprintf("%s-2-test.kafka-nodeport-%d.external.nodeport.com:30102", kafkaCluster.Name, count),
						},
					},
				},
			}))
		})
	})
})
