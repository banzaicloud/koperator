// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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

	"github.com/banzaicloud/koperator/api/v1beta1"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

func expectDefaultBrokerSettingsForExternalListenerBinding(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	// Check Brokers
	for _, broker := range kafkaCluster.Spec.Brokers {
		broker := broker
		// expect ConfigMap
		configMap := corev1.ConfigMap{}
		Eventually(ctx, func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, broker.Id),
			}, &configMap)
		}).Should(Succeed())

		Expect(configMap.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
		Expect(configMap.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
		Expect(configMap.Labels).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(int(broker.Id))))

		brokerConfig, err := properties.NewFromString(configMap.Data["broker-config"])
		Expect(err).NotTo(HaveOccurred())
		advertisedListener, found := brokerConfig.Get("advertised.listeners")
		expectedAdvertisedListenersFound := true
		var expectedAdvertisedListener string
		if kafkaCluster.Spec.KRaftMode {
			switch broker.Id {
			case 0:
				// broker-only node
				expectedAdvertisedListener = fmt.Sprintf("INTERNAL://kafkacluster-kraft-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
					randomGenTestNumber, broker.Id, randomGenTestNumber, 19090+broker.Id)
			case 1:
				// controller-only node
				expectedAdvertisedListenersFound = false
				expectedAdvertisedListener = ""
			case 2:
				// combined node
				expectedAdvertisedListener = fmt.Sprintf("INTERNAL://kafkacluster-kraft-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
					randomGenTestNumber, broker.Id, randomGenTestNumber, 19090+broker.Id)
			}
		} else {
			expectedAdvertisedListener = fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,"+
				"INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
				randomGenTestNumber, broker.Id, randomGenTestNumber, randomGenTestNumber, broker.Id, randomGenTestNumber, 19090+broker.Id)
		}
		Expect(found).To(Equal(expectedAdvertisedListenersFound))
		Expect(advertisedListener.Value()).To(Equal(expectedAdvertisedListener))

		listeners, found := brokerConfig.Get("listeners")
		Expect(found).To(BeTrue())

		var expectedListeners string
		if kafkaCluster.Spec.KRaftMode {
			switch broker.Id {
			case 0:
				expectedListeners = "INTERNAL://:29092,TEST://:9094"
			case 1:
				expectedListeners = "CONTROLLER://:29093"
			case 2:
				expectedListeners = "INTERNAL://:29092,CONTROLLER://:29093,TEST://:9094"
			}
		} else {
			expectedListeners = "INTERNAL://:29092,CONTROLLER://:29093,TEST://:9094"
		}
		Expect(listeners.Value()).To(Equal(expectedListeners))

		listenerSecMap, found := brokerConfig.Get(kafkautils.KafkaConfigListenerSecurityProtocolMap)
		Expect(found).To(BeTrue())
		Expect(listenerSecMap.Value()).To(Equal("INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT,TEST:PLAINTEXT"))
		// check service
		service := corev1.Service{}
		Eventually(ctx, func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      fmt.Sprintf("%s-%d", kafkaCluster.Name, broker.Id),
			}, &service)
		}).Should(Succeed())

		Expect(service.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
		Expect(service.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
		Expect(service.Labels).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(int(broker.Id))))

		Expect(service.Spec.Ports).To(ConsistOf(
			corev1.ServicePort{
				Name:       "tcp-internal",
				Protocol:   corev1.ProtocolTCP,
				Port:       29092,
				TargetPort: intstr.FromInt(29092),
			},
			corev1.ServicePort{
				Name:       "tcp-controller",
				Protocol:   corev1.ProtocolTCP,
				Port:       29093,
				TargetPort: intstr.FromInt(29093),
			},
			corev1.ServicePort{
				Name:       "tcp-test",
				Protocol:   corev1.ProtocolTCP,
				Port:       9094,
				TargetPort: intstr.FromInt(9094),
			},
			corev1.ServicePort{
				Name:       "metrics",
				Protocol:   corev1.ProtocolTCP,
				Port:       9020,
				TargetPort: intstr.FromInt(9020),
			}))
	}
}

func expectBrokerConfigmapForAz1ExternalListener(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	configMap := corev1.ConfigMap{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, 0),
		}, &configMap)
	}).Should(Succeed())

	brokerConfig, err := properties.NewFromString(configMap.Data["broker-config"])
	Expect(err).NotTo(HaveOccurred())
	advertisedListener, found := brokerConfig.Get("advertised.listeners")
	Expect(found).To(BeTrue())
	var expectedAdvertisedListener string
	if kafkaCluster.Spec.KRaftMode {
		// broker-0 is a broker-only node
		expectedAdvertisedListener = fmt.Sprintf("INTERNAL://kafkacluster-kraft-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
			randomGenTestNumber, 0, randomGenTestNumber, 19090)
	} else {
		expectedAdvertisedListener = fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d",
			randomGenTestNumber, 0, randomGenTestNumber, randomGenTestNumber, 0, randomGenTestNumber, 19090)
	}
	Expect(advertisedListener.Value()).To(Equal(expectedAdvertisedListener))
}

func expectBrokerConfigmapForAz2ExternalListener(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	configMap := corev1.ConfigMap{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, 1),
		}, &configMap)
	}).Should(Succeed())

	brokerConfig, err := properties.NewFromString(configMap.Data["broker-config"])
	Expect(err).NotTo(HaveOccurred())
	advertisedListener, found := brokerConfig.Get("advertised.listeners")
	var (
		expectedAdvertisedListener string
		expectedFound              bool
	)
	// for the Kafka cluster under KRaft mode, broker-1 is a controller-only node and therefore "advertised.listeners" is not available
	if !kafkaCluster.Spec.KRaftMode {
		expectedAdvertisedListener = fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az2.host.com:%d",
			randomGenTestNumber, 1, randomGenTestNumber, randomGenTestNumber, 1, randomGenTestNumber, 19091)
		expectedFound = true
	}
	Expect(found).To(Equal(expectedFound))
	Expect(advertisedListener.Value()).To(Equal(expectedAdvertisedListener))

	configMap = corev1.ConfigMap{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, 2),
		}, &configMap)
	}).Should(Succeed())

	brokerConfig, err = properties.NewFromString(configMap.Data["broker-config"])
	Expect(err).NotTo(HaveOccurred())
	advertisedListener, found = brokerConfig.Get("advertised.listeners")
	Expect(found).To(BeTrue())
	if kafkaCluster.Spec.KRaftMode {
		expectedAdvertisedListener = fmt.Sprintf("INTERNAL://kafkacluster-kraft-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az2.host.com:%d",
			randomGenTestNumber, 2, randomGenTestNumber, 19092)
	} else {
		expectedAdvertisedListener = fmt.Sprintf("CONTROLLER://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafkaconfigtest-%d.svc.cluster.local:29092,TEST://external.az2.host.com:%d",
			randomGenTestNumber, 2, randomGenTestNumber, randomGenTestNumber, 2, randomGenTestNumber, 19092)
	}
	Expect(advertisedListener.Value()).To(Equal(expectedAdvertisedListener))
}
