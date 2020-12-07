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
	"errors"
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/Shopify/sarama"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

var _ = Describe("KafkaTopic", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-topic-%v", count)
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

	When("the topic does not exist", func() {
		JustBeforeEach(func() {

		})

		JustAfterEach(func() {

		})

		It("creates properly", func() {
			topicName := "test-topic"
			crTopicName := fmt.Sprintf("kafkatopic-%v", count)
			topic := v1alpha1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crTopicName,
					Namespace: namespace,
				},
				Spec: v1alpha1.KafkaTopicSpec{
					Name:              topicName,
					Partitions:        17,
					ReplicationFactor: 19,
					Config: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
					ClusterRef: v1alpha1.ClusterReference{
						Name:      kafkaCluster.Name,
						Namespace: namespace,
					},
				},
			}

			err := k8sClient.Create(context.TODO(), &topic)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() (v1alpha1.TopicState, error) {
				topic := v1alpha1.KafkaTopic{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Namespace: kafkaCluster.Namespace,
					Name:      crTopicName,
				}, &topic)
				if err != nil {
					return "", err
				}
				return topic.Status.State, nil
			}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.TopicStateCreated))

			name := types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}
			mockKafkaClient := mockKafkaClients[name]
			Expect(mockKafkaClient).NotTo(BeNil())
			detail, err := mockKafkaClient.GetTopic(topicName)
			Expect(err).NotTo(HaveOccurred())
			Expect(detail).To(Equal(&sarama.TopicDetail{
				NumPartitions:     17,
				ReplicationFactor: 19,
				ConfigEntries: map[string]*string{
					"key1": util.StringPointer("value1"),
					"key2": util.StringPointer("value2"),
				},
			}))

			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      topic.Name,
				Namespace: topic.Namespace,
			}, &topic)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(context.TODO(), &topic)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				createdKafkaTopic := &v1alpha1.KafkaTopic{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, createdKafkaTopic)
				if err == nil {
					return errors.New("cluster should be deleted")
				}
				if apierrors.IsNotFound(err) {
					return nil
				} else {
					return err
				}
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})
	})

	When("the topic exists", func() {
		var topicName = "already-created-topic"

		JustBeforeEach(func() {
			name := types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}
			mockKafkaClient, err := kafkaclient.NewMockFromCluster(k8sClient, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockKafkaClient).NotTo(BeNil())
			mockKafkaClients[name] = mockKafkaClient

			err = mockKafkaClient.CreateTopic(&kafkaclient.CreateTopicOptions{
				Name:              topicName,
				Partitions:        11,
				ReplicationFactor: 13,
				Config:            map[string]*string{
					"key": util.StringPointer("value"),
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates properly", func() {
			crTopicName := fmt.Sprintf("kafkatopic-%v", count)

			topic := v1alpha1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crTopicName,
					Namespace: namespace,
				},
				Spec: v1alpha1.KafkaTopicSpec{
					Name:              topicName,
					Partitions:        17,
					ReplicationFactor: 19,
					Config: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
					ClusterRef: v1alpha1.ClusterReference{
						Name:      kafkaCluster.Name,
						Namespace: namespace,
					},
				},
			}

			err := k8sClient.Create(context.TODO(), &topic)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() (v1alpha1.TopicState, error) {
				topic := v1alpha1.KafkaTopic{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Namespace: kafkaCluster.Namespace,
					Name:      crTopicName,
				}, &topic)
				if err != nil {
					return "", err
				}
				return topic.Status.State, nil
			}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.TopicStateCreated))

			name := types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}
			mockKafkaClient := mockKafkaClients[name]
			Expect(mockKafkaClient).NotTo(BeNil())
			detail, err := mockKafkaClient.GetTopic(topicName)
			Expect(err).NotTo(HaveOccurred())
			Expect(detail).To(Equal(&sarama.TopicDetail{
				NumPartitions:     11,
				ReplicationFactor: 13,
				ConfigEntries: map[string]*string{
					"key": util.StringPointer("value"),
				},
			}))

			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      topic.Name,
				Namespace: topic.Namespace,
			}, &topic)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(context.TODO(), &topic)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				createdKafkaTopic := &v1alpha1.KafkaTopic{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, createdKafkaTopic)
				if err == nil {
					return errors.New("cluster should be deleted")
				}
				if apierrors.IsNotFound(err) {
					return nil
				} else {
					return err
				}
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})
	})
})
