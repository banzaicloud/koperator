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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

var _ = Describe("KafkaCluster", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		// create default Kafka cluster spec
		kafkaCluster = &v1beta1.KafkaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("kafkacluster-%v", count),
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
							ExternalStartingPort: 11202,
							HostnameOverride:     "test-host",
							AccessMethod:         corev1.ServiceTypeLoadBalancer,
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
				MonitoringConfig: v1beta1.MonitoringConfig{
					CCJMXExporterConfig: "custom_property: custom_value",
				},
				CruiseControlConfig: v1beta1.CruiseControlConfig{
					TopicConfig: &v1beta1.TopicConfig{
						Partitions:        7,
						ReplicationFactor: 2,
					},
					Config: "some.config=value",
				},
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

		waitClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	It("should reconciles objects properly", func() {
		expectEnvoy(kafkaCluster, namespace)
		expectKafkaMonitoring(kafkaCluster, namespace)
		expectCruiseControlMonitoring(kafkaCluster, namespace)
		expectKafka(kafkaCluster, namespace)
		expectCruiseControl(kafkaCluster, namespace)
	})
})

func expectKafkaMonitoring(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	configMap := corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-kafka-jmx-exporter", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: namespace}, &configMap)
		return err
	}).Should(Succeed())

	Expect(configMap.Labels).To(And(HaveKeyWithValue("app", "kafka-jmx"), HaveKeyWithValue("kafka_cr", kafkaCluster.Name)))
	Expect(configMap.Data).To(HaveKeyWithValue("config.yaml", Not(BeEmpty())))
}

func expectCruiseControlMonitoring(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	configMap := corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-cc-jmx-exporter", kafkaCluster.Name)
	logf.Log.Info("name", "name", configMapName)
	Eventually(func() error {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: namespace}, &configMap)
		return err
	}).Should(Succeed())

	Expect(configMap.Labels).To(And(HaveKeyWithValue("app", "cruisecontrol-jmx"), HaveKeyWithValue("kafka_cr", kafkaCluster.Name)))
	Expect(configMap.Data).To(HaveKeyWithValue("config.yaml", kafkaCluster.Spec.MonitoringConfig.CCJMXExporterConfig))
}
