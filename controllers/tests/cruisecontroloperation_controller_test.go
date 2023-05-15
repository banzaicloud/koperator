// Copyright © 2022 Cisco Systems, Inc. and/or its affiliates
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
	"sync/atomic"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/controllers/tests/mocks"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
)

var _ = Describe("CruiseControlTaskReconciler", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
		opName1            = "operation1"
		opName2            = "operation2"
	)
	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("cruisecontroloperation-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%v", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)
	})

	JustBeforeEach(func(ctx SpecContext) {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

	})
	JustAfterEach(func() {
		cruiseControlOperationReconciler.ScaleFactory = func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
			return nil, errors.New("there is no scale mock")
		}
	})
	When("there is an add_broker operation for execution", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock1())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())

		})
		It("should execute the task and the task later will be completed", func(ctx SpecContext) {
			Eventually(ctx, func() v1beta1.CruiseControlUserTaskState {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation)
				if err != nil {
					return ""
				}
				return operation.CurrentTaskState()
			}, 10*time.Second, 500*time.Millisecond).Should(Equal(v1beta1.CruiseControlTaskCompleted))
		})
	})
	When("add_broker operation is finished with completedWithError and 30s has not elapsed", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock2())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should not retry the failed task", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation)
				if err != nil {
					return false
				}
				return operation.CurrentTaskState() == v1beta1.CruiseControlTaskCompletedWithError && len(operation.Status.FailedTasks) == 0
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("add_broker operation is finished with completedWithError and 30s has elapsed", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock5())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationAddBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should retry the failed task", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation)
				if err != nil {
					return false
				}
				return operation.CurrentTaskState() == v1beta1.CruiseControlTaskCompleted && len(operation.Status.FailedTasks) == 1
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is an errored remove_broker and an add_broker operation", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock3())
			// First operation will get completedWithError
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			// Creating other operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should not retry the errored task but should execute the add_broker", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation2.CurrentTaskState() == v1beta1.CruiseControlTaskCompleted
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is a new remove_broker and an errored remove_broker operation with pause annotation", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock4())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			operation.Labels["pause"] = "true"
			err := k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationRemoveBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			// Creating the second operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should execute new remove_broker operation and should not retry the other one with pause annotation", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation1.Status.RetryCount == 0 && operation2.CurrentTaskState() == v1beta1.CruiseControlTaskCompleted
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is a new remove_broker and an errored remove_broker operation with ignore ErrorPolicy", Serial, func() {
		JustBeforeEach(func(ctx SpecContext) {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock4())
			// Creating first operation
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			operation.Spec.ErrorPolicy = v1alpha1.ErrorPolicyIgnore
			err := k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
				Operation: v1alpha1.OperationRemoveBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			// Creating the second operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
			}
			err = k8sClient.Status().Update(ctx, &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should execute the new remove_broker operation and should not execute the errored one", func(ctx SpecContext) {
			Eventually(ctx, func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(ctx, client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation1.Status.RetryCount == 0 && operation2.CurrentTaskState() == v1beta1.CruiseControlTaskCompleted
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is an errored remove_disks and a rebalance disks operation for the same broker", Serial, func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.ScaleFactory = NewMockScaleFactory(getScaleMock6())
			// Remove_disk operation - errored
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveDisks,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
				Parameters: map[string]string{
					scale.ParamBrokerIDAndLogDirs: "101-logdir1",
				},
			}
			err = k8sClient.Status().Update(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())
			// Rebalance operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRebalance,
				Parameters: map[string]string{
					scale.ParamDestbrokerIDs: "101,102",
				},
			}
			err = k8sClient.Status().Update(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should mark the removed disk task as paused and should execute the rebalance", func() {
			Eventually(func() bool {
				removeDisksOp := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &removeDisksOp)
				if err != nil {
					return false
				}
				rebalanceOp := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &rebalanceOp)
				if err != nil {
					return false
				}

				return rebalanceOp.CurrentTaskState() == v1beta1.CruiseControlTaskCompleted &&
					removeDisksOp.GetLabels()[v1alpha1.PauseLabel] == v1alpha1.True
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
})

func generateCruiseControlOperation(name, namespace, kafkaRef string) v1alpha1.CruiseControlOperation {
	return v1alpha1.CruiseControlOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    util.LabelsForKafka(kafkaRef),
		},
		Spec: v1alpha1.CruiseControlOperationSpec{},
	}
}
func getScaleMock2() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp(gomock.Any()).Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status(gomock.Any()).Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)
	return scaleMock
}
func getScaleMock1() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp(gomock.Any()).Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status(gomock.Any()).Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil)
	return scaleMock
}

func getScaleMock3() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp(gomock.Any()).Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	}), scaleResultPointer(scale.Result{
		TaskID:    "2",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status(gomock.Any()).Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().RemoveBrokersWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "2",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)

	return scaleMock
}

func getScaleMock4() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp(gomock.Any()).Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "1",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	}), scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status(gomock.Any()).Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().RemoveBrokersWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "1",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)
	return scaleMock
}

func getScaleMock5() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp(gomock.Any()).Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	userTaskResult2 := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	first := scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult, nil).Times(1)
	scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult2, nil).After(first).AnyTimes()
	scaleMock.EXPECT().Status(gomock.Any()).Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)
	return scaleMock
}

func getScaleMock6() *mocks.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := mocks.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp(gomock.Any()).Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	scaleMock.EXPECT().UserTasks(gomock.Any(), gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status(gomock.Any()).Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().RebalanceWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12346",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	}), nil).Times(1)

	scaleMock.EXPECT().RemoveDisksWithParams(gomock.Any(), gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	}), nil).AnyTimes()

	return scaleMock
}

func scaleResultPointer(res scale.Result) *scale.Result {
	return &res
}
