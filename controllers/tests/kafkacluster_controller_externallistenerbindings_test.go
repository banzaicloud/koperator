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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

func expectConfigConfiguredExternalListenerBinding(kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
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

		Expect(configMap.Data).To(HaveKeyWithValue("broker-config", fmt.Sprintf(`advertised.listeners=CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://external.az1.host.com:%d
broker.id=%d
control.plane.listener.name=CONTROLLER
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT,TEST:PLAINTEXT
listeners=INTERNAL://:29092,CONTROLLER://:29093,TEST://:9094
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=/`, randomGenTestNumber, broker.Id, randomGenTestNumber, randomGenTestNumber,
			broker.Id, randomGenTestNumber, 19090+broker.Id, broker.Id, randomGenTestNumber, broker.Id, randomGenTestNumber)))
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

func expectExternalListenerBindings(kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	expectAllBrokerServiceWithTwoEListeners(kafkaCluster)
	broker := kafkaCluster.Spec.Brokers[0]
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

	Expect(configMap.Data).To(HaveKeyWithValue("broker-config", fmt.Sprintf(`advertised.listeners=CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,EXTERNAL://external.host.com:%d,INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092
broker.id=%d
control.plane.listener.name=CONTROLLER
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT,EXTERNAL:PLAINTEXT
listeners=INTERNAL://:29092,CONTROLLER://:29093,EXTERNAL://:9095
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=/`, randomGenTestNumber, broker.Id, randomGenTestNumber, 19095+broker.Id, randomGenTestNumber, broker.Id, randomGenTestNumber, broker.Id, randomGenTestNumber, broker.Id, randomGenTestNumber)))
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
			Name:       "tcp-external",
			Protocol:   corev1.ProtocolTCP,
			Port:       9095,
			TargetPort: intstr.FromInt(9095),
		},
		corev1.ServicePort{
			Name:       "metrics",
			Protocol:   "TCP",
			Port:       9020,
			TargetPort: intstr.FromInt(9020),
		}))
	// check pod
	podList := corev1.PodList{}
	Eventually(func() ([]corev1.Pod, error) {
		err := k8sClient.List(context.Background(), &podList,
			client.ListOption(client.InNamespace(kafkaCluster.Namespace)),
			client.ListOption(client.MatchingLabels(map[string]string{"app": "kafka", "kafka_cr": kafkaCluster.Name, "brokerId": strconv.Itoa(int(broker.Id))})))
		return podList.Items, err
	}).Should(HaveLen(1))

	pod := podList.Items[0]

	Expect(pod.GenerateName).To(Equal(fmt.Sprintf("%s-%d-", kafkaCluster.Name, broker.Id)))
	container := pod.Spec.Containers[0]
	Expect(container.Name).To(Equal("kafka"))
	// broker 1
	broker = kafkaCluster.Spec.Brokers[1]
	// expect ConfigMap
	configMap = corev1.ConfigMap{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, broker.Id),
		}, &configMap)
	}).Should(Succeed())

	Expect(configMap.Labels).To(HaveKeyWithValue("app", "kafka"))
	Expect(configMap.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
	Expect(configMap.Labels).To(HaveKeyWithValue("brokerId", strconv.Itoa(int(broker.Id))))

	Expect(configMap.Data).To(HaveKeyWithValue("broker-config", fmt.Sprintf(`advertised.listeners=CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://test.host.com:%d
broker.id=%d
control.plane.listener.name=CONTROLLER
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT,TEST:PLAINTEXT
listeners=INTERNAL://:29092,CONTROLLER://:29093,TEST://:9094
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=/`, randomGenTestNumber, broker.Id, randomGenTestNumber, randomGenTestNumber, broker.Id, randomGenTestNumber, 19090+broker.Id, broker.Id, randomGenTestNumber, broker.Id, randomGenTestNumber)))
	// check service
	service = corev1.Service{}
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
			Protocol:   corev1.ProtocolTCP,
			Port:       9094,
			TargetPort: intstr.FromInt(9094),
		},
		corev1.ServicePort{
			Name:       "metrics",
			Protocol:   "TCP",
			Port:       9020,
			TargetPort: intstr.FromInt(9020),
		}))
	// check pod
	podList = corev1.PodList{}
	Eventually(func() ([]corev1.Pod, error) {
		err := k8sClient.List(context.Background(), &podList,
			client.ListOption(client.InNamespace(kafkaCluster.Namespace)),
			client.ListOption(client.MatchingLabels(map[string]string{"app": "kafka", "kafka_cr": kafkaCluster.Name, "brokerId": strconv.Itoa(int(broker.Id))})))
		return podList.Items, err
	}).Should(HaveLen(1))

	pod = podList.Items[0]

	Expect(pod.GenerateName).To(Equal(fmt.Sprintf("%s-%d-", kafkaCluster.Name, broker.Id)))
	container = pod.Spec.Containers[0]
	Expect(container.Name).To(Equal("kafka"))

}

func expectExternalListenerGroupBindings(kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	expectAllBrokerServiceWithTwoEListeners(kafkaCluster)
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

		Expect(configMap.Data).To(HaveKeyWithValue("broker-config", fmt.Sprintf(`advertised.listeners=CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092,TEST://test.host.com:%d
broker.id=%d
control.plane.listener.name=CONTROLLER
cruise.control.metrics.reporter.bootstrap.servers=INTERNAL://kafkacluster-3-%d.kafka-3.svc.cluster.local:29092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT,TEST:PLAINTEXT
listeners=INTERNAL://:29092,CONTROLLER://:29093,TEST://:9094
log.dirs=/kafka-logs/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=/`, randomGenTestNumber, broker.Id, randomGenTestNumber, randomGenTestNumber, broker.Id, randomGenTestNumber, 19090+broker.Id, broker.Id, broker.Id)))
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
		// check pod
		podList := corev1.PodList{}
		Eventually(func() ([]corev1.Pod, error) {
			err := k8sClient.List(context.Background(), &podList,
				client.ListOption(client.InNamespace(kafkaCluster.Namespace)),
				client.ListOption(client.MatchingLabels(map[string]string{"app": "kafka", "kafka_cr": kafkaCluster.Name, "brokerId": strconv.Itoa(int(broker.Id))})))
			return podList.Items, err
		}).Should(HaveLen(1))

		pod := podList.Items[0]

		Expect(pod.GenerateName).To(Equal(fmt.Sprintf("%s-%d-", kafkaCluster.Name, broker.Id)))
		container := pod.Spec.Containers[0]
		Expect(container.Name).To(Equal("kafka"))
	}

}

func expectAllBrokerServiceWithTwoEListeners(kafkaCluster *v1beta1.KafkaCluster) {
	service := &corev1.Service{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-all-broker", kafkaCluster.Name),
		}, service)
	}).Should(Succeed())

	Expect(service.Labels).To(HaveKeyWithValue("app", "kafka"))
	Expect(service.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))

	Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
	Expect(service.Spec.SessionAffinity).To(Equal(corev1.ServiceAffinityNone))
	Expect(service.Spec.Selector).To(HaveKeyWithValue("app", "kafka"))
	Expect(service.Spec.Selector).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
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
			Name:       "tcp-external",
			Protocol:   corev1.ProtocolTCP,
			Port:       9095,
			TargetPort: intstr.FromInt(9095),
		}))
}
