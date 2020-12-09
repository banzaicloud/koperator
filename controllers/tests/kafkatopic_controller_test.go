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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/Shopify/sarama"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
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

		kafkaCluster = createMinimalKafkaClusterCR(fmt.Sprintf("kafkacluster-%v", count), namespace)
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
		resetMockKafkaClient(kafkaCluster)

		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		kafkaCluster = nil
	})

	When("the topic does not exist", func() {
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

			mockKafkaClient := getMockedKafkaClientForCluster(kafkaCluster)
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
		})
	})

	When("the topic exists", func() {
		var topicName = "already-created-topic"

		JustBeforeEach(func() {
			mockKafkaClient := getMockedKafkaClientForCluster(kafkaCluster)

			err := mockKafkaClient.CreateTopic(&kafkaclient.CreateTopicOptions{
				Name:              topicName,
				Partitions:        11,
				ReplicationFactor: 13,
				Config: map[string]*string{
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

			mockKafkaClient := getMockedKafkaClientForCluster(kafkaCluster)
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
		})
	})
})
