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
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

const (
	maxReconcileDuration   = 10 * time.Second       // max amount of time to wait for a reconcile to execute
	reconcilePollingPeriod = 500 * time.Millisecond // poll the results of a reconcile every 500 millisecond for up to maxReconcileDuration
)

var _ = Describe("CruiseControlTaskReconciler", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
		opName             string = "operation1"
	)
	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("cruisecontroloperationttl-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		kafkaClusterCRName = fmt.Sprintf("kafkacluster-ttl-%v", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.Background(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.Background(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

	})

	When("there is a finished (completed) operation with TTL", Serial, func() {
		JustBeforeEach(func() {
			operation := generateCruiseControlOperation(opName, namespace, kafkaCluster.GetName())
			operation.Spec.TTLSecondsAfterFinished = util.IntPointer(5)
			err := k8sClient.Create(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationAddBroker,
				State:     v1beta1.CruiseControlTaskCompleted,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

		})
		It("it should remove the finished CruiseControlOperation", func() {
			Eventually(func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName,
				}, &operation)
				if err != nil && apierrors.IsNotFound(err) {
					return true
				}
				return false
			}, maxReconcileDuration, reconcilePollingPeriod).Should(BeTrue())
		})
	})
	When("there is a finished (completedWithError and errorPolicy: ignore) operation with TTL", Serial, func() {
		JustBeforeEach(func() {
			operation := generateCruiseControlOperation(opName, namespace, kafkaCluster.GetName())
			operation.Spec.TTLSecondsAfterFinished = util.IntPointer(5)
			operation.Spec.ErrorPolicy = v1alpha1.ErrorPolicyIgnore
			err := k8sClient.Create(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationAddBroker,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

		})
		It("it should  remove the finished CruiseControlOperation", func() {
			Eventually(func() bool {
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName,
				}, &operation)
				if err != nil && apierrors.IsNotFound(err) {
					return true
				}
				return false
			}, maxReconcileDuration, reconcilePollingPeriod).Should(BeTrue())
		})
	})
	When("there is a finished operation without TTL", Serial, func() {
		JustBeforeEach(func() {
			operation := generateCruiseControlOperation(opName, namespace, kafkaCluster.GetName())
			err := k8sClient.Create(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

			operation.Status.CurrentTask = &v1alpha1.CruiseControlTask{
				ID:        "12345",
				Operation: v1alpha1.OperationAddBroker,
				State:     v1beta1.CruiseControlTaskCompleted,
				Finished:  &metav1.Time{Time: time.Now().Add(-time.Second*v1alpha1.DefaultRetryBackOffDurationSec - 10)},
			}
			err = k8sClient.Status().Update(context.Background(), &operation)
			Expect(err).NotTo(HaveOccurred())

		})
		It("it should not remove the finished CruiseControlOperation", func() {
			counter := 0
			Eventually(func() bool {
				counter++
				operation := v1alpha1.CruiseControlOperation{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: kafkaCluster.Namespace,
					Name:      opName,
				}, &operation)
				Expect(err).NotTo(HaveOccurred())
				return counter >= 3
			}, maxReconcileDuration, reconcilePollingPeriod).Should(BeTrue())
		})
	})
})
