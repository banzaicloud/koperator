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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
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
		kafkaCluster = &v1beta1.KafkaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kafkaClusterCRName,
				Namespace: namespace,
			},
			Spec: v1beta1.KafkaClusterSpec{
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{
						{
							CommonListenerSpec: v1beta1.CommonListenerSpec{
								Name:          "test",
								ContainerPort: 9733,
							},
							ExternalStartingPort: 31123,
							HostnameOverride:     "test-host",
							AccessMethod:         corev1.ServiceTypeNodePort,
						},
					},
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

		Eventually(func() (v1beta1.ClusterState, error) {
			createdKafkaCluster := &v1beta1.KafkaCluster{}
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: kafkaCluster.Name, Namespace: namespace}, createdKafkaCluster)
			if err != nil {
				return v1beta1.KafkaClusterReconciling, err
			}
			if createdKafkaCluster == nil {
				return v1beta1.KafkaClusterReconciling, nil
			}
			return createdKafkaCluster.Status.State, nil
		}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1beta1.KafkaClusterRunning))
	})

	JustAfterEach(func() {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	It("service successfully reconciled", func() {
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
	})
})
