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

package controllers

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

var _ = Describe("CruiseControlReconciler", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-cc-%v", count)
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
					ExternalListeners: []v1beta1.ExternalListenerConfig{},
					InternalListeners: []v1beta1.InternalListenerConfig{},
				},
				Brokers:     []v1beta1.Broker{},
				ZKAddresses: []string{},
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
	})

	Context("CruiseControl topics", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.CruiseControlConfig = v1beta1.CruiseControlConfig{
				CruiseControlEndpoint: "test",
			}
		})

		PWhen("CC automatic topic creation is disabled in CC config", func() {
			BeforeEach(func() {
				kafkaCluster.Spec.ReadOnlyConfig = "cruise.control.metrics.topic.auto.create=false"

				kafkaCluster.Spec.CruiseControlConfig.TopicConfig = &v1beta1.TopicConfig{
					Partitions:        7,
					ReplicationFactor: 2,
				}
			})

			It("creates CC Kafka topic", func() {
				topic := &v1alpha1.KafkaTopic{}
				Eventually(func() error {
					topicObjName := fmt.Sprintf("%s-cruise-control-topic", kafkaCluster.Name)
					err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: topicObjName}, topic)
					if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
						return nil
					}
					return err
				}, 5000, 1000).Should(Succeed())

				Expect(topic).To(BeNil())
				Expect(topic.Name).To(Equal(fmt.Sprintf("%s-cruise-control-topic", kafkaCluster.Name)))
				Expect(topic.Labels).To(HaveKeyWithValue("app", "kafka"))
				Expect(topic.Labels).To(HaveKeyWithValue("clusterName", kafkaCluster.Name))
				Expect(topic.Labels).To(HaveKeyWithValue("clusterNamespace", namespace))

				Expect(topic.Spec).To(Equal(v1alpha1.KafkaTopicSpec{
					Name:              "__CruiseControlMetrics",
					Partitions:        7,
					ReplicationFactor: 2,
					ClusterRef: v1alpha1.ClusterReference{
						Name:      kafkaCluster.Name,
						Namespace: namespace,
					},
				}))
			})
		})

		When("CC automatic topic creation is enabled in CC config", func() {
			BeforeEach(func() {
				kafkaCluster.Spec.ReadOnlyConfig = "cruise.control.metrics.topic.auto.create=true"
			})

			It("does not create CC Kafka topic", func() {
				Consistently(func() ([]v1alpha1.KafkaTopic, error) {
					topicList := &v1alpha1.KafkaTopicList{}
					err := k8sClient.List(context.Background(), topicList, client.InNamespace(namespace), client.MatchingLabels{"app": "kafka"})
					if err != nil {
						return nil, err
					}
					return topicList.Items, nil
				}).Should(BeEmpty())
			})
		})
	})

	It("reconciles without error when not enabled", func() {

	})

	When("CC is enabled", func() {
		BeforeEach(func() {

		})

		It("reconciles correctly", func() {
			// CC topic generation assert is in another ginkgo path
		})
	})
})
