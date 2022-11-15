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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
	"github.com/banzaicloud/koperator/pkg/util"
)

var _ = Describe("CruiseControlTaskReconciler", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
		operation          *v1alpha1.CruiseControlOperation
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-cc-task-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		kafkaClusterCRName = fmt.Sprintf("kafkacluster-cctask-%v", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.Background(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.Background(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		if operation != nil {
			err := k8sClient.Delete(context.Background(), operation)
			Expect(err).NotTo(HaveOccurred())
		}
		err := k8sClient.Delete(context.Background(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())
	})

	When("new broker is added", Serial, func() {
		JustBeforeEach(func() {
			kafkaClusterCCReconciler.ScaleFactory = NewMockScaleFactory(getScaleMockCCTask1())
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			kafkaCluster.Spec.Brokers = append(kafkaCluster.Spec.Brokers, v1beta1.Broker{
				Id:                3,
				BrokerConfigGroup: "default",
			})
			err = k8sClient.Update(context.Background(), kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should create one add_broker CruiseControlOperation", Serial, func() {
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
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
					operation = &operationList.Items[0]
					return actionState.CruiseControlOperationReference.Name == operation.Name && operation.CurrentTaskOperation() == v1alpha1.OperationAddBroker && actionState.CruiseControlState == v1beta1.GracefulUpscaleScheduled

				}
				return false
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
		When("created CruiseControlOperation state is inExecution", Serial, func() {
			It("kafkaCluster gracefulActionState should be GracefulUpscaleRunning", func() {
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), types.NamespacedName{
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

						var operation v1alpha1.CruiseControlOperation
						if len(operationList.Items) == 0 {
							operation = generateCruiseControlOperation(brokerState.GracefulActionState.CruiseControlOperationReference.Name, kafkaCluster.Namespace, kafkaCluster.Name)
							err := k8sClient.Create(context.Background(), &operation)
							Expect(err).NotTo(HaveOccurred())

						} else {
							operation = operationList.Items[0]
						}

						operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
							ID:        "12345",
							Started:   &metav1.Time{Time: time.Now()},
							Operation: v1alpha1.OperationAddBroker,
							State:     v1beta1.CruiseControlTaskInExecution,
						}
						err = k8sClient.Status().Update(context.Background(), &operation)
						Expect(err).NotTo(HaveOccurred())

						return actionState.CruiseControlOperationReference.Name == operation.Name &&
							operation.CurrentTaskOperation() == v1alpha1.OperationAddBroker && actionState.CruiseControlState == v1beta1.GracefulUpscaleRunning
					}
					return false
				}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
			})
		})

	})
	When("a broker is removed", Serial, func() {
		JustBeforeEach(func() {
			kafkaClusterCCReconciler.ScaleFactory = NewMockScaleFactory(getScaleMockCCTask1())
			err := util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
				if err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster); err != nil {
					return err
				}

				brokerState := kafkaCluster.Status.BrokersState["2"]
				brokerState.GracefulActionState.CruiseControlState = v1beta1.GracefulDownscaleRequired
				kafkaCluster.Status.BrokersState["2"] = brokerState
				err := k8sClient.Status().Update(context.Background(), kafkaCluster)
				return err
			})
			Expect(err).NotTo(HaveOccurred())

		})
		It("should create one remove_broker CruiseControlOperation", func() {
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
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
					operation = &operationList.Items[0]
					return actionState.CruiseControlOperationReference.Name == operation.Name && operation.CurrentTaskOperation() == v1alpha1.OperationRemoveBroker && actionState.CruiseControlState == v1beta1.GracefulDownscaleScheduled
				}
				return false
			}, 15*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
})

func NewMockScaleFactory(mock scale.CruiseControlScaler) func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
	return func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
		return mock, nil
	}
}

func getScaleMockCCTask1() *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	availableBrokers := []string{"1", "2", "3"}
	scaleMock.EXPECT().BrokersWithState(gomock.All()).Return(availableBrokers, nil).AnyTimes()
	return scaleMock
}
