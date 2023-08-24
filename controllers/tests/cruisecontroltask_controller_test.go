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
	"context"
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/controllers/tests/mocks"

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

	const (
		mountPath = "/kafka-logs-test"
		trueStr   = "true"
	)

	BeforeEach(func(ctx SpecContext) {
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

	JustBeforeEach(func(ctx context.Context) {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(ctx, kafkaCluster, namespace)
	})

	JustAfterEach(func(ctx SpecContext) {
		if operation != nil {
			err := k8sClient.DeleteAllOf(ctx, &v1alpha1.CruiseControlOperation{}, client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
		}
		err := k8sClient.Delete(context.Background(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())
	})
	When("new storage is added", Serial, func() {

		JustBeforeEach(func(ctx SpecContext) {
			kafkaClusterCCReconciler.ScaleFactory = mocks.NewMockScaleFactory(getScaleMockCCTask2([]string{mountPath}))

			err := util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster); err != nil {
					return err
				}
				brokerState := kafkaCluster.Status.BrokersState["0"]
				volumeState := make(map[string]v1beta1.VolumeState)
				volumeState[mountPath] = v1beta1.VolumeState{
					CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired,
				}

				brokerState.GracefulActionState.VolumeStates = volumeState
				kafkaCluster.Status.BrokersState["0"] = brokerState
				return k8sClient.Status().Update(ctx, kafkaCluster)
			})

			Expect(err).NotTo(HaveOccurred())

		})
		It("should create one JBOD rebalance CruiseControlOperation", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["0"]
				if !ok {
					return false
				}
				volumeState, ok := brokerState.GracefulActionState.VolumeStates[mountPath]
				if !ok || volumeState.CruiseControlOperationReference == nil {
					return false
				}

				operationList := &v1alpha1.CruiseControlOperationList{}

				err = k8sClient.List(ctx, operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
				Expect(err).NotTo(HaveOccurred())

				if len(operationList.Items) != 1 {
					return false
				}
				operation = &operationList.Items[0]
				return volumeState.CruiseControlOperationReference.Name == operation.Name &&
					operation.CurrentTaskOperation() == v1alpha1.OperationRebalance &&
					volumeState.CruiseControlVolumeState == v1beta1.GracefulDiskRebalanceScheduled &&
					operation.CurrentTask() != nil && operation.CurrentTask().Parameters["rebalance_disk"] == trueStr

			}, 15*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("new storage is added but there is a not JBOD capacityConfig for that", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			kafkaClusterCCReconciler.ScaleFactory = mocks.NewMockScaleFactory(getScaleMockCCTask2([]string{mountPath}))
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			kafkaCluster.Spec.CruiseControlConfig.CapacityConfig = `
			{
			"brokerCapacities":[
			  {
				"brokerId": "0",
				"capacity": {
				  "DISK": "50000",
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`
			err = k8sClient.Update(ctx, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			err = util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster); err != nil {
					return err
				}
				brokerState := kafkaCluster.Status.BrokersState["0"]
				volumeState := make(map[string]v1beta1.VolumeState)
				volumeState[mountPath] = v1beta1.VolumeState{
					CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired,
				}

				brokerState.GracefulActionState.VolumeStates = volumeState
				kafkaCluster.Status.BrokersState["0"] = brokerState
				return k8sClient.Status().Update(ctx, kafkaCluster)
			})

			Expect(err).NotTo(HaveOccurred())

		})
		It("should create one not JBOD rebalance CruiseControlOperation", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["0"]
				if !ok {
					return false
				}
				volumeState, ok := brokerState.GracefulActionState.VolumeStates[mountPath]
				if !ok || volumeState.CruiseControlOperationReference == nil {
					return false
				}

				operationList := &v1alpha1.CruiseControlOperationList{}

				err = k8sClient.List(ctx, operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
				Expect(err).NotTo(HaveOccurred())

				if len(operationList.Items) != 1 {
					return false
				}
				operation = &operationList.Items[0]
				return volumeState.CruiseControlOperationReference.Name == operation.Name &&
					operation.CurrentTaskOperation() == v1alpha1.OperationRebalance &&
					volumeState.CruiseControlVolumeState == v1beta1.GracefulDiskRebalanceScheduled &&
					operation.CurrentTask() != nil && operation.CurrentTask().Parameters["rebalance_disk"] != trueStr

			}, 15*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("new storage is added and one broker is JBOD and another is not JBOD", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			kafkaClusterCCReconciler.ScaleFactory = mocks.NewMockScaleFactory(getScaleMockCCTask2([]string{mountPath}))
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())

			kafkaCluster.Spec.CruiseControlConfig.CapacityConfig = `
			{
			"brokerCapacities":[
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": "50000",
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`
			err = k8sClient.Update(ctx, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			err = util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster); err != nil {
					return err
				}
				brokerState0 := kafkaCluster.Status.BrokersState["0"]
				volumeState := make(map[string]v1beta1.VolumeState)
				volumeState[mountPath] = v1beta1.VolumeState{
					CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired,
				}
				brokerState0.GracefulActionState.VolumeStates = volumeState
				kafkaCluster.Status.BrokersState["0"] = brokerState0

				brokerState1 := kafkaCluster.Status.BrokersState["1"]
				brokerState1.GracefulActionState.VolumeStates = volumeState
				kafkaCluster.Status.BrokersState["1"] = brokerState1

				return k8sClient.Status().Update(ctx, kafkaCluster)
			})

			Expect(err).NotTo(HaveOccurred())

		})
		It("should create one not JBOD rebalance CruiseControlOperation", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["0"]
				if !ok {
					return false
				}
				volumeState, ok := brokerState.GracefulActionState.VolumeStates[mountPath]
				if !ok || volumeState.CruiseControlOperationReference == nil {
					return false
				}

				operationList := &v1alpha1.CruiseControlOperationList{}

				err = k8sClient.List(ctx, operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
				Expect(err).NotTo(HaveOccurred())

				if len(operationList.Items) != 1 {
					return false
				}

				operation = &operationList.Items[0]

				return volumeState.CruiseControlOperationReference.Name == operation.Name &&
					operation.CurrentTaskOperation() == v1alpha1.OperationRebalance &&
					volumeState.CruiseControlVolumeState == v1beta1.GracefulDiskRebalanceScheduled &&
					operation.CurrentTask() != nil && operation.CurrentTask().Parameters["rebalance_disk"] != trueStr
			}, 15*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("new broker is added", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			kafkaClusterCCReconciler.ScaleFactory = mocks.NewMockScaleFactory(getScaleMockCCTask1())
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      kafkaCluster.Name,
				Namespace: kafkaCluster.Namespace,
			}, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
			kafkaCluster.Spec.Brokers = append(kafkaCluster.Spec.Brokers, v1beta1.Broker{
				Id:                3,
				BrokerConfigGroup: "default",
			})
			err = k8sClient.Update(ctx, kafkaCluster)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should create one add_broker CruiseControlOperation", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["3"]
				if ok && brokerState.GracefulActionState.CruiseControlOperationReference != nil {
					actionState := brokerState.GracefulActionState

					operationList := &v1alpha1.CruiseControlOperationList{}

					err = k8sClient.List(ctx, operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
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
			It("kafkaCluster gracefulActionState should be GracefulUpscaleRunning", func(ctx SpecContext) {
				Eventually(ctx, func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      kafkaCluster.Name,
						Namespace: kafkaCluster.Namespace,
					}, kafkaCluster)
					Expect(err).NotTo(HaveOccurred())

					brokerState, ok := kafkaCluster.Status.BrokersState["3"]
					if ok && brokerState.GracefulActionState.CruiseControlOperationReference != nil {
						actionState := brokerState.GracefulActionState

						operationList := &v1alpha1.CruiseControlOperationList{}

						err = k8sClient.List(ctx, operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
						Expect(err).NotTo(HaveOccurred())

						var operation v1alpha1.CruiseControlOperation
						if len(operationList.Items) == 0 {
							operation = generateCruiseControlOperation(brokerState.GracefulActionState.CruiseControlOperationReference.Name, kafkaCluster.Namespace, kafkaCluster.Name)
							err := k8sClient.Create(ctx, &operation)
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
						err = k8sClient.Status().Update(ctx, &operation)
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
		JustBeforeEach(func(ctx SpecContext) {
			kafkaClusterCCReconciler.ScaleFactory = mocks.NewMockScaleFactory(getScaleMockCCTask1())
			err := util.RetryOnConflict(util.DefaultBackOffForConflict, func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster); err != nil {
					return err
				}

				brokerState := kafkaCluster.Status.BrokersState["2"]
				brokerState.GracefulActionState.CruiseControlState = v1beta1.GracefulDownscaleRequired
				kafkaCluster.Status.BrokersState["2"] = brokerState
				err := k8sClient.Status().Update(ctx, kafkaCluster)
				return err
			})
			Expect(err).NotTo(HaveOccurred())

		})
		It("should create one remove_broker CruiseControlOperation", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      kafkaCluster.Name,
					Namespace: kafkaCluster.Namespace,
				}, kafkaCluster)
				Expect(err).NotTo(HaveOccurred())

				brokerState, ok := kafkaCluster.Status.BrokersState["2"]
				if ok && brokerState.GracefulActionState.CruiseControlOperationReference != nil {
					actionState := brokerState.GracefulActionState

					operationList := &v1alpha1.CruiseControlOperationList{}

					err = k8sClient.List(ctx, operationList, client.ListOption(client.InNamespace(kafkaCluster.Namespace)))
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

func getScaleMockCCTask1() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	availableBrokers := []string{"1", "2", "3"}
	scaleMock.EXPECT().BrokersWithState(gomock.Any(), gomock.All()).Return(availableBrokers, nil).AnyTimes()
	return scaleMock
}

func getScaleMockCCTask2(onlineLogDirs []string) *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	availableBrokers := []string{"1", "2", "3"}
	scaleMock.EXPECT().BrokersWithState(gomock.Any(), gomock.All()).Return(availableBrokers, nil).AnyTimes()

	logDirs := make(map[scale.LogDirState][]string)
	logDirsBrokerRet := make(map[string]map[scale.LogDirState][]string)

	logDirs[scale.LogDirStateOnline] = onlineLogDirs
	logDirsBrokerRet["0"] = logDirs
	logDirsBrokerRet["1"] = logDirs
	scaleMock.EXPECT().LogDirsByBroker(gomock.Any()).Return(logDirsBrokerRet, nil).AnyTimes()
	return scaleMock
}
