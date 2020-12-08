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

	corev1 "k8s.io/api/core/v1"
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
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)

		// update the external listener config with a nodeport listener
		kafkaCluster.Spec.ListenersConfig.ExternalListeners = []v1beta1.ExternalListenerConfig{
			{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Name:          "test",
					ContainerPort: 9733,
				},
				ExternalStartingPort: 31123,
				HostnameOverride:     "test-host",
				AccessMethod:         corev1.ServiceTypeNodePort,
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

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
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
	})
})
