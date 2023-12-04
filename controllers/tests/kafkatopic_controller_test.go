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
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/IBM/sarama"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/util"
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

	JustBeforeEach(func(ctx SpecContext) {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(ctx, kafkaCluster, namespace)
	})

	JustAfterEach(func(ctx SpecContext) {
		resetMockKafkaClient(kafkaCluster)

		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		kafkaCluster = nil
	})

	When("the topic does not exist", func() {
		It("creates properly", func(ctx SpecContext) {
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

			err := k8sClient.Create(ctx, &topic)
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, func() (v1alpha1.TopicState, error) {
				topic := v1alpha1.KafkaTopic{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: kafkaCluster.Namespace,
					Name:      crTopicName,
				}, &topic)
				if err != nil {
					return "", err
				}
				return topic.Status.State, nil
			}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.TopicStateCreated))

			mockKafkaClient, _ := getMockedKafkaClientForCluster(kafkaCluster)
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

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      topic.Name,
				Namespace: topic.Namespace,
			}, &topic)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, &topic)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the topic exists", func() {
		var topicName = "already-created-topic"

		JustBeforeEach(func() {
			mockKafkaClient, _ := getMockedKafkaClientForCluster(kafkaCluster)

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

		It("creates properly", func(ctx SpecContext) {
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

			err := k8sClient.Create(ctx, &topic)
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, func() (v1alpha1.TopicState, error) {
				topic := v1alpha1.KafkaTopic{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: kafkaCluster.Namespace,
					Name:      crTopicName,
				}, &topic)
				if err != nil {
					return "", err
				}
				return topic.Status.State, nil
			}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.TopicStateCreated))

			mockKafkaClient, _ := getMockedKafkaClientForCluster(kafkaCluster)
			detail, err := mockKafkaClient.GetTopic(topicName)
			Expect(err).NotTo(HaveOccurred())
			Expect(detail).To(Equal(&sarama.TopicDetail{
				NumPartitions:     11,
				ReplicationFactor: 13,
				ConfigEntries: map[string]*string{
					"key": util.StringPointer("value"),
				},
			}))

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      topic.Name,
				Namespace: topic.Namespace,
			}, &topic)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, &topic)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
