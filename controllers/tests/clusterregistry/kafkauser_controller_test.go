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

var _ = Describe("KafkaUserController", func() {
	var (
		count            uint64 = 0
		namespace        string
		namespaceObj     *corev1.Namespace
		kafkaClusterName string
		kafkaUser        *v1alpha1.KafkaUser
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-user-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaClusterName = fmt.Sprintf("kafkacluster-%d", count)

		kafkaUser = &v1alpha1.KafkaUser{
			ObjectMeta: metav1.ObjectMeta{
				Name:        namespace,
				Namespace:   namespace,
				Annotations: map[string]string{},
			},
			Spec: v1alpha1.KafkaUserSpec{
				ClusterRef: v1alpha1.ClusterReference{
					Namespace: namespace,
					Name:      kafkaClusterName,
				},
				TopicGrants: []v1alpha1.UserTopicGrant{
					{
						TopicName:   "test-topic-1",
						AccessType:  v1alpha1.KafkaAccessTypeRead,
						PatternType: v1alpha1.KafkaPatternTypeAny,
					},
					{
						TopicName:   "test-topic-2",
						AccessType:  v1alpha1.KafkaAccessTypeWrite,
						PatternType: v1alpha1.KafkaPatternTypeLiteral,
					},
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
		kafkaUserReconciler.Reset()
	})

	Describe("Controller should ignore events for KafkaUser CRs", func() {
		It("with OwnershipAnnotation is set", func() {
			By("creating the KafkaUser CR")
			expectedNumOfReconciles := 0
			kafkaUser.Annotations[clusterregv1alpha1.OwnershipAnnotation] = "id"
			err := k8sClient.Create(ctx, kafkaUser)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaUserReconciler.NumOfRequests()
				log.Info("check reconciles on create", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("updating the KafkaUser CR")
			kafkaUser.Spec.TopicGrants = []v1alpha1.UserTopicGrant{
				{
					TopicName:   "test-topic-3",
					AccessType:  v1alpha1.KafkaAccessTypeRead,
					PatternType: v1alpha1.KafkaPatternTypeAny,
				},
				{
					TopicName:   "test-topic-4",
					AccessType:  v1alpha1.KafkaAccessTypeWrite,
					PatternType: v1alpha1.KafkaPatternTypeLiteral,
				},
			}
			err = k8sClient.Update(ctx, kafkaUser)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaUserReconciler.NumOfRequests()
				log.Info("check reconciles on update", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("deleting the KafkaUser CR")
			err = k8sClient.Delete(ctx, kafkaUser)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaUserReconciler.NumOfRequests()
				log.Info("check reconciles on delete", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))
		})
	})

	Describe("Controller should not ignore events for KafkaUser CRs", func() {
		It("with OwnershipAnnotation not set", func() {
			By("creating the KafkaUser CR")
			expectedNumOfReconciles := 1
			err := k8sClient.Create(ctx, kafkaUser)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaUserReconciler.NumOfRequests()
				log.Info("check reconciles on create", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("updating the KafkaUser CR")
			expectedNumOfReconciles += 1
			kafkaUser.Spec.TopicGrants = []v1alpha1.UserTopicGrant{
				{
					TopicName:   "test-topic-3",
					AccessType:  v1alpha1.KafkaAccessTypeRead,
					PatternType: v1alpha1.KafkaPatternTypeAny,
				},
				{
					TopicName:   "test-topic-4",
					AccessType:  v1alpha1.KafkaAccessTypeWrite,
					PatternType: v1alpha1.KafkaPatternTypeLiteral,
				},
			}
			err = k8sClient.Update(ctx, kafkaUser)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaUserReconciler.NumOfRequests()
				log.Info("check reconciles on update", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("deleting the KafkaUser CR")
			expectedNumOfReconciles += 1
			err = k8sClient.Delete(ctx, kafkaUser)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaUserReconciler.NumOfRequests()
				log.Info("check reconciles on delete", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))
		})
	})
})
