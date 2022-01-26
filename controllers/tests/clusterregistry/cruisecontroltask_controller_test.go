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

	clusterregv1alpha1 "github.com/banzaicloud/cluster-registry/api/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

var _ = Describe("CruiseControllerTaskController", func() {
	var (
		count            uint64 = 0
		namespace        string
		namespaceObj     *corev1.Namespace
		kafkaClusterName string
		kafkaCluster     *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-cluster-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaClusterName = fmt.Sprintf("kafkacluster-%d", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterName, namespace)
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		kafkaClusterReconciler.Reset()
	})

	Describe("Controller should ignore events for KafkaCluster CRs", func() {
		It("with OwnershipAnnotation is set", func() {
			By("creating the KafkaCluster CR")
			expectedNumOfReconciles := 0
			kafkaCluster.Annotations[clusterregv1alpha1.OwnershipAnnotation] = "id"
			err := k8sClient.Create(ctx, kafkaCluster)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaClusterReconciler.NumOfRequests()
				log.Info("check reconciles on create", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("updating the KafkaCluster CR Status")
			kafkaCluster.Status.BrokersState = map[string]v1beta1.BrokerState{
				"0": {
					GracefulActionState: v1beta1.GracefulActionState{
						CruiseControlState: v1beta1.GracefulUpscaleRequired,
					},
				},
				"1": {
					GracefulActionState: v1beta1.GracefulActionState{
						CruiseControlState: v1beta1.GracefulUpscaleRequired,
					},
				},
				"2": {
					GracefulActionState: v1beta1.GracefulActionState{
						CruiseControlState: v1beta1.GracefulUpscaleRequired,
					},
				},
			}
			err = k8sClient.Status().Update(ctx, kafkaCluster)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaClusterReconciler.NumOfRequests()
				log.Info("check reconciles on update", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("deleting the KafkaCluster CR")
			err = k8sClient.Delete(ctx, kafkaCluster)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaClusterReconciler.NumOfRequests()
				log.Info("check reconciles on delete", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))
		})
	})

	Describe("Controller should not ignore events for KafkaCluster CRs", func() {
		It("with OwnershipAnnotation not set", func() {
			By("creating the KafkaCluster CR")
			expectedNumOfReconciles := 1
			err := k8sClient.Create(ctx, kafkaCluster)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaClusterReconciler.NumOfRequests()
				log.Info("check reconciles on create", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("updating the KafkaCluster CR Status")
			expectedNumOfReconciles += 1
			kafkaCluster.Status.BrokersState = map[string]v1beta1.BrokerState{
				"0": {
					GracefulActionState: v1beta1.GracefulActionState{
						CruiseControlState: v1beta1.GracefulUpscaleRequired,
					},
				},
				"1": {
					GracefulActionState: v1beta1.GracefulActionState{
						CruiseControlState: v1beta1.GracefulUpscaleRequired,
					},
				},
				"2": {
					GracefulActionState: v1beta1.GracefulActionState{
						CruiseControlState: v1beta1.GracefulUpscaleRequired,
					},
				},
			}
			err = k8sClient.Status().Update(ctx, kafkaCluster)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaClusterReconciler.NumOfRequests()
				log.Info("check reconciles on update", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))

			By("deleting the KafkaCluster CR")
			expectedNumOfReconciles += 1
			err = k8sClient.Delete(ctx, kafkaCluster)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() int {
				reconciles := kafkaClusterReconciler.NumOfRequests()
				log.Info("check reconciles on delete", "reconciles", reconciles)
				return reconciles
			}, 10).Should(BeNumerically("==", expectedNumOfReconciles))
		})
	})
})
