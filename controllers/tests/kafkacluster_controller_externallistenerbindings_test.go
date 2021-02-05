// Copyright © 2021 Banzai Cloud
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
	"strconv"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	properties "github.com/banzaicloud/kafka-operator/properties/pkg"
)

func expectDefaultBrokerSettingsForExternalListenerBinding(kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	// Check Brokers
	for _, broker := range kafkaCluster.Spec.Brokers {
		// expect ConfigMap
		configMap := corev1.ConfigMap{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, broker.Id),
			}, &configMap)
		}).Should(Succeed())

		Expect(configMap.Labels).To(HaveKeyWithValue("app", "kafka"))
		Expect(configMap.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
		Expect(configMap.Labels).To(HaveKeyWithValue("brokerId", strconv.Itoa(int(broker.Id))))

		brokerConfig, err := properties.NewFromString(configMap.Data["broker-config"])
		Expect(err).NotTo(HaveOccurred())
		advertisedListener, found := brokerConfig.Get("advertised.listeners")
		Expect(found).To(BeTrue())
		Expect(advertisedListener.Value()).To(Equal(fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
			randomGenTestNumber, broker.Id, randomGenTestNumber, randomGenTestNumber, broker.Id, randomGenTestNumber, 19090+broker.Id)))
		listeners, found := brokerConfig.Get("listeners")
		Expect(found).To(BeTrue())
		Expect(listeners.Value()).To(Equal("INTERNAL://:29092,CONTROLLER://:29093,TEST://:9094"))
		listenerSecMap, found := brokerConfig.Get("listener.security.protocol.map")
		Expect(found).To(BeTrue())
		Expect(listenerSecMap.Value()).To(Equal("INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT,TEST:PLAINTEXT"))
		// check service
		service := corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      fmt.Sprintf("%s-%d", kafkaCluster.Name, broker.Id),
			}, &service)
		}).Should(Succeed())

		Expect(service.Labels).To(HaveKeyWithValue("app", "kafka"))
		Expect(service.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
		Expect(service.Labels).To(HaveKeyWithValue("brokerId", strconv.Itoa(int(broker.Id))))

		Expect(service.Spec.Ports).To(ConsistOf(
			corev1.ServicePort{
				Name:       "tcp-internal",
				Protocol:   "TCP",
				Port:       29092,
				TargetPort: intstr.FromInt(29092),
			},
			corev1.ServicePort{
				Name:       "tcp-controller",
				Protocol:   "TCP",
				Port:       29093,
				TargetPort: intstr.FromInt(29093),
			},
			corev1.ServicePort{
				Name:       "tcp-test",
				Protocol:   "TCP",
				Port:       9094,
				TargetPort: intstr.FromInt(9094),
			},
			corev1.ServicePort{
				Name:       "metrics",
				Protocol:   "TCP",
				Port:       9020,
				TargetPort: intstr.FromInt(9020),
			}))
	}
}

func expectBrokerConfigmapForAz1ExternalListener(kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	configMap := corev1.ConfigMap{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, 0),
		}, &configMap)
	}).Should(Succeed())

	brokerConfig, err := properties.NewFromString(configMap.Data["broker-config"])
	Expect(err).NotTo(HaveOccurred())
	advertisedListener, found := brokerConfig.Get("advertised.listeners")
	Expect(found).To(BeTrue())
	Expect(advertisedListener.Value()).To(Equal(fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
		randomGenTestNumber, 0, randomGenTestNumber, randomGenTestNumber, 0, randomGenTestNumber, 19090)))
}

func expectBrokerConfigmapForAz2ExternalListener(kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	configMap := corev1.ConfigMap{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, 1),
		}, &configMap)
	}).Should(Succeed())

	brokerConfig, err := properties.NewFromString(configMap.Data["broker-config"])
	Expect(err).NotTo(HaveOccurred())
	advertisedListener, found := brokerConfig.Get("advertised.listeners")
	Expect(found).To(BeTrue())
	Expect(advertisedListener.Value()).To(Equal(fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az2.host.com:%d",
		randomGenTestNumber, 1, randomGenTestNumber, randomGenTestNumber, 1, randomGenTestNumber, 19091)))

	configMap = corev1.ConfigMap{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, 2),
		}, &configMap)
	}).Should(Succeed())

	brokerConfig, err = properties.NewFromString(configMap.Data["broker-config"])
	Expect(err).NotTo(HaveOccurred())
	advertisedListener, found = brokerConfig.Get("advertised.listeners")
	Expect(found).To(BeTrue())
	Expect(advertisedListener.Value()).To(Equal(fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az2.host.com:%d",
		randomGenTestNumber, 2, randomGenTestNumber, randomGenTestNumber, 2, randomGenTestNumber, 19092)))
}
