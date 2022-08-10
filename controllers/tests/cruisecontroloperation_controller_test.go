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

	// JustAfterEach(func() {
	// 	By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
	// 	err := k8sClient.Delete(context.TODO(), kafkaCluster)
	// 	Expect(err).NotTo(HaveOccurred())

	// 	kafkaCluster = nil
	// })

	When("CruiseControlOperation created and the controller executed and ended without error", func() {
		BeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock1(GinkgoT())
		})
		It("should execute the task", func() {
			name := "operation1"
			operation := getCruiseControlOperation(name, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() v1beta1.CruiseControlUserTaskState {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      name,
				}, &operation)
				if err != nil {
					return ""
				}
				return operation.GetCurrentTaskState()
			}, 15*time.Second, 500*time.Millisecond).Should(Equal(v1beta1.CruiseControlTaskCompleted))
		})
	})
	When("CruiseControlOperation created and the controller executed and task resulted error.", func() {
		BeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock2(GinkgoT())
		})
		It("should retry the failedtask", func() {
			name := "operation1"
			operation := getCruiseControlOperation(name, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      name,
				}, &operation)
				if err != nil {
					return false
				}
				return operation.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompleted && len(operation.Status.FailedTasks) == 1
			}, 15*time.Second, 500*time.Millisecond).Should(Equal(true))
		})
	})
	When("Multiple CruiseControlOperation created and the controller executed", func() {
		BeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock3(GinkgoT())
		})
		FIt("should not retry the failedtask and should execute addBroker", func() {
			name := "operation1"
			// First operation will get completedWithError
			operation := getCruiseControlOperation(name, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			// Add other operation
			name2 := name + "2"
			operation = getCruiseControlOperation(name2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      name,
				}, &operation1)
				if err != nil {
					return false
				}
				operation2 := v1alpha1.CruiseControlOperation{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      name2,
				}, &operation2)
				if err != nil {
					return false
				}

				return operation1.Status.NumberOfRetries == 0 && operation2.GetCurrentTaskState() == v1beta1.CruiseControlTaskActive
			}, 15*time.Second, 500*time.Millisecond).Should(Equal(true))
		})
	})
	When("When there is a new remove_roker and an errored one with pause annotation", func() {
		BeforeEach(func() {
			cruiseControlOperationReconciler.Scaler = getScaleMock3(GinkgoT())
		})
		It("should execute new remove_broker operation", func() {
			name := "operation1"
			// First operation will get completedWithError
			operation := getCruiseControlOperation(name, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationRemoveBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			// Add other operation
			name2 := name + "2"
			operation = getCruiseControlOperation(name2, namespace, kafkaCluster.GetName())
			err = k8sClient.Create(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())
			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				Operation: v1alpha1.OperationAddBroker,
			}
			err = k8sClient.Status().Update(context.TODO(), &operation)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				operation1 := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      name,
				}, &operation1)
				if err != nil {
					return false
				}

				return operation1.Status.NumberOfRetries > 1
			}, 15*time.Second, 500*time.Millisecond).Should(Equal(true))
		})
	})

})

func getCruiseControlOperation(name, namespace, kafkaRef string) v1alpha1.CruiseControlOperation {
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
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	userTaskResult2 := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	first := scaleMock.EXPECT().GetUserTasks().Return(userTaskResult, nil).Times(2)
	scaleMock.EXPECT().GetUserTasks().Return(userTaskResult2, nil).After(first).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
	}).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).MinTimes(2).MaxTimes(2)
	return scaleMock
}
func getScaleMock1(t GinkgoTInterface) *scale.MockCruiseControlScaler {
	mockCtrl := gomock.NewController(GinkgoT())
	scaleMock := scale.NewMockCruiseControlScaler(mockCtrl)
	scaleMock.EXPECT().IsUp().Return(true).AnyTimes()

	userTaskResult := []*scale.Result{scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskCompleted,
	})}
	scaleMock.EXPECT().GetUserTasks().Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
	}).AnyTimes()
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "2022-02-13T15:04:05Z",
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
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskCompletedWithError,
	})}
	scaleMock.EXPECT().GetUserTasks().Return(userTaskResult, nil).AnyTimes()
	scaleMock.EXPECT().Status().Return(scale.CruiseControlStatus{
		ExecutorReady: true,
	}).AnyTimes()
	scaleMock.EXPECT().RemoveBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "12345",
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)
	scaleMock.EXPECT().AddBrokersWithParams(gomock.All()).Return(scaleResultPointer(scale.Result{
		TaskID:    "2",
		StartedAt: "2022-02-13T15:04:05Z",
		State:     v1beta1.CruiseControlTaskActive,
	}), nil).Times(1)

	return scaleMock
}
