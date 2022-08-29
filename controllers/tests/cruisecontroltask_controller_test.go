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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
)

var _ = Describe("CruiseControlTaskReconciler", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-cc-task-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%d", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)
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
	When("new broker is added", func() {
		JustBeforeEach(func() {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			kafkaCluster.Spec.Brokers = append(kafkaCluster.Spec.Brokers, v1beta1.Broker{
				Id:                3,
				BrokerConfigGroup: "default",
			})
			err = k8sClient.Update(context.TODO(), kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should create one add_broker CruiseControlOperation", func() {
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["3"]
				if ok && brokerState.GracefulActionState.CruiseControlOperationReference != nil {
					actionState := brokerState.GracefulActionState

					operationList := &v1alpha1.CruiseControlOperationList{}

					err = k8sClient.List(context.Background(), operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
					Expect(err).NotTo(HaveOccurred())

					if len(operationList.Items) != 1 {
						return false
					}
					operation := &operationList.Items[0]
					return actionState.CruiseControlOperationReference.Name == operation.Name && operation.GetCurrentTaskOp() == v1alpha1.OperationAddBroker && actionState.CruiseControlState == v1beta1.GracefulUpscaleScheduled
				}
				return false
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
		When("created CruiseControlOperation state is inExecutuin", func() {
			It("kafkaCluster gracefulActionState should be GracefulUpscaleRunning", func() {
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Name:      kafkaCluster.Name,
						Namespace: kafkaCluster.Namespace,
					}, kafkaCluster)
					Expect(err).NotTo(HaveOccurred())

					brokerState, ok := kafkaCluster.Status.BrokersState["3"]
					if ok && brokerState.GracefulActionState.CruiseControlOperationReference != nil {
						actionState := brokerState.GracefulActionState

						operationList := &v1alpha1.CruiseControlOperationList{}

						err = k8sClient.List(context.Background(), operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
						Expect(err).NotTo(HaveOccurred())

						if len(operationList.Items) != 1 {
							return false
						}
						operation := &operationList.Items[0]
						if operation.Status.CurrentTask.State != v1beta1.CruiseControlTaskInExecution {
							operation.Status.CurrentTask.State = v1beta1.CruiseControlTaskInExecution
							err := k8sClient.Status().Update(context.Background(), operation)
							Expect(err).NotTo(HaveOccurred())
						}

						return actionState.CruiseControlOperationReference.Name == operation.Name &&
							operation.GetCurrentTaskOp() == v1alpha1.OperationAddBroker && actionState.CruiseControlState == v1beta1.GracefulUpscaleRunning
					}
					return false
				}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
			})
		})

	})
	When("a broker is removed", func() {
		JustBeforeEach(func() {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			kafkaCluster.Spec.Brokers = kafkaCluster.Spec.Brokers[:1]

			err = k8sClient.Update(context.TODO(), kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			brokerState := kafkaCluster.Status.BrokersState["2"]
			brokerState.GracefulActionState.CruiseControlState = v1beta1.GracefulDownscaleRequired
			kafkaCluster.Status.BrokersState["2"] = brokerState
			err = k8sClient.Status().Update(context.Background(), kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

		})
		FIt("should create one remove_broker CruiseControlOperation", func() {
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["2"]
				if ok && brokerState.GracefulActionState.CruiseControlOperationReference != nil {
					actionState := brokerState.GracefulActionState

					operationList := &v1alpha1.CruiseControlOperationList{}

					err = k8sClient.List(context.Background(), operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
					Expect(err).NotTo(HaveOccurred())

					if len(operationList.Items) != 1 {
						return false
					}
					operation := &operationList.Items[0]
					return actionState.CruiseControlOperationReference.Name == operation.Name && operation.GetCurrentTaskOp() == v1alpha1.OperationRemoveBroker && actionState.CruiseControlState == v1beta1.GracefulDownscaleScheduled
				}
				return false
			}, 20*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("broker is removed from CR", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.Brokers = []v1beta1.Broker{
				{
					Id:                0,
					BrokerConfigGroup: "default",
				},
				{
					Id:                1,
					BrokerConfigGroup: "default",
				},
				{
					Id:                2,
					BrokerConfigGroup: "default",
				},
			}
		})

		It("reconciles the task correctly", func() {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			getGracefulActionState := func(state v1beta1.BrokerState) v1beta1.CruiseControlState {
				return state.GracefulActionState.CruiseControlState
			}
			Expect(kafkaCluster.Status.BrokersState).To(And(
				HaveKeyWithValue("0", WithTransform(getGracefulActionState, Equal(v1beta1.GracefulUpscaleSucceeded))),
				HaveKeyWithValue("1", WithTransform(getGracefulActionState, Equal(v1beta1.GracefulUpscaleSucceeded))),
				HaveKeyWithValue("2", WithTransform(getGracefulActionState, Equal(v1beta1.GracefulUpscaleSucceeded))),
			))

			// remove broker with id 2, update CR
			kafkaCluster.Spec.Brokers = kafkaCluster.Spec.Brokers[:1]
			err = k8sClient.Update(context.TODO(), kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(kafkaCluster.Status.BrokersState).To(And(
				HaveKeyWithValue("0", WithTransform(getGracefulActionState, Equal(v1beta1.GracefulUpscaleSucceeded))),
				HaveKeyWithValue("1", WithTransform(getGracefulActionState, Equal(v1beta1.GracefulUpscaleSucceeded))),
			))
		})
	})
})
