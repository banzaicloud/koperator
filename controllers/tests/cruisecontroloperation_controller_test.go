// Copyright © 2022 Banzai Cloud
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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		opName1            string = "operation1"
		opName2            string = "operation2"
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

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

	})

	When("there is an add_broker operation for execution", func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock1(GinkgoT())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

		})
		It("should execute the task and the task later will be completed", func() {
			Eventually(func() v1beta1.CruiseControlUserTaskState {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation)
				if err != nil {
					return ""
				}
				return operation.GetCurrentTaskState()
			}, 10*time.Second, 500*time.Millisecond).Should(Equal(v1beta1.CruiseControlTaskCompleted))
		})
	})
	When("add_broker operation is finished with completedWithError and 30s has not elapsed", func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock2(GinkgoT())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should not retry the failed task", func() {
			Eventually(func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation)
				if err != nil {
					return false
				}
				return operation.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompletedWithError && len(operation.Status.FailedTasks) == 0
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("add_broker operation is finished with completedWithError and 30s has elapsed", func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock5(GinkgoT())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationAddBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should retry the failed task", func() {
			Eventually(func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation)
				if err != nil {
					return false
				}
				return operation.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompleted && len(operation.Status.FailedTasks) == 1
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is an errored remove_broker and an add_broker operation", func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock3(GinkgoT())
			// First operation will get completedWithError
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			// Creating other operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should not retry the errored task but should execute the add_broker", func() {
			Eventually(func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation2.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompleted
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is a new remove_broker and an errored remove_broker operation with pause annotation", func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock4(GinkgoT())
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			operation.Labels["pause"] = "true"
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationRemoveBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			// Creating the second operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should execute new remove_broker operation and should not retry the other one with pause annotation", func() {
			Eventually(func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation1.Status.NumberOfRetries == 0 && operation2.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompleted
			}, 40*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
	When("there is a new remove_broker and an errored remove_broker operation with ignore ErrorPolicy", func() {
		JustBeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock4(GinkgoT())
			// Creating first operation
			operation := generateCruiseControlOperation(opName1, namespace, kafkaCluster.GetName())
			operation.Spec.ErrorPolicy = v1alpha1.ErrorPolicyIgnore
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
				Operation: v1alpha1.OperationRemoveBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			// Creating the second operation
			operation = generateCruiseControlOperation(opName2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should execute the new remove_broker operation and should not execute the errored one", func() {
			Eventually(func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName1,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation1.Status.NumberOfRetries == 0 && operation2.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompleted
			}, 40*time.Second, 500*time.Millisecond).Should(BeTrue())
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
func getScaleMock2(t GinkgoTInterface) *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(t)
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp().Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	scaleMock.EXPECT().GetUserTasks(gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)
	return scaleMock
}
func getScaleMock1(t GinkgoTInterface) *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp().Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	scaleMock.EXPECT().GetUserTasks(gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil)
	return scaleMock
}

func getScaleMock3(t GinkgoTInterface) *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp().Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	}), scaleResultPointer(scale.Result{
		TaskID:    "2",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	scaleMock.EXPECT().GetUserTasks(gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().RemoveBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "2",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)

	return scaleMock
}

func getScaleMock4(t GinkgoTInterface) *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(t)
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp().Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "1",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompleted,
	}), scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	scaleMock.EXPECT().GetUserTasks(gomock.Any()).Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().RemoveBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "1",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)
	return scaleMock
}

func getScaleMock5(t GinkgoTInterface) *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(t)
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp().Return(true).AnyTimes()

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
	first := scaleMock.EXPECT().GetUserTasks(gomock.Any()).Return(userTaskResult, nil).Times(1)
	scaleMock.EXPECT().GetUserTasks(gomock.Any()).Return(userTaskResult2, nil).After(first).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
		MonitorReady:  true,
		AnalyzerReady: true,
	}, nil).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "Sat, 27 Aug 2022 12:22:21 GMT",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(2)
	return scaleMock
}

func scaleResultPointer(res scale.Result) *scale.Result {
	return &res
}
