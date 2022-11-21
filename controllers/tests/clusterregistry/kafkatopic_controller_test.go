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

package clusterregistry

import (
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clusterregv1alpha1 "github.com/cisco-open/cluster-registry-controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
)

var _ = Describe("KafkaTopicController", func() {
	var (
		count            uint64 = 0
		namespace        string
		namespaceObj     *corev1.Namespace
		kafkaClusterName string
		kafkaTopic       *v1alpha1.KafkaTopic
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-topic-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaClusterName = fmt.Sprintf("kafkacluster-%d", count)

		kafkaTopic = &v1alpha1.KafkaTopic{
			ObjectMeta: metav1.ObjectMeta{
				Name:        namespace,
				Namespace:   namespace,
				Annotations: map[string]string{},
			},
			Spec: v1alpha1.KafkaTopicSpec{
				Name:              namespace,
				Partitions:        10,
				ReplicationFactor: 3,
				Config: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				ClusterRef: v1alpha1.ClusterReference{
					Name:      kafkaClusterName,
					Namespace: namespace,
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		kafkaTopicReconciler.Reset()
	})

	Describe("Controller should ignore events for KafkaTopic CRs", func() {
		It("with OwnershipAnnotation is set", func() {
			By("creating the KafkaTopic CR")
			expectedNumOfReconciles := 0
			kafkaTopic.Annotations[clusterregv1alpha1.OwnershipAnnotation] = "id"
			err := k8sClient.Create(ctx, kafkaTopic)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaTopicReconciler.NumOfRequests()
				log.Info("check reconciles on create", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("updating the KafkaTopic CR")
			kafkaTopic.Spec.Config = map[string]string{
				"key3": "value3",
				"key4": "value4",
			}
			err = k8sClient.Update(ctx, kafkaTopic)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaTopicReconciler.NumOfRequests()
				log.Info("check reconciles on update", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("deleting the KafkaTopic CR")
			err = k8sClient.Delete(ctx, kafkaTopic)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaTopicReconciler.NumOfRequests()
				log.Info("check reconciles on delete", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))
		})
	})

	Describe("Controller should not ignore events for KafkaTopic CRs", func() {
		It("with OwnershipAnnotation not set", func() {
			By("creating the KafkaTopic CR")
			expectedNumOfReconciles := 1
			err := k8sClient.Create(ctx, kafkaTopic)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaTopicReconciler.NumOfRequests()
				log.Info("check reconciles on create", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("updating the KafkaTopic CR")
			expectedNumOfReconciles += 1
			kafkaTopic.Spec.Config = map[string]string{
				"key3": "value3",
				"key4": "value4",
			}
			err = k8sClient.Update(ctx, kafkaTopic)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaTopicReconciler.NumOfRequests()
				log.Info("check reconciles on update", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("deleting the KafkaTopic CR")
			expectedNumOfReconciles += 1
			err = k8sClient.Delete(ctx, kafkaTopic)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaTopicReconciler.NumOfRequests()
				log.Info("check reconciles on delete", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))
		})
	})
})
