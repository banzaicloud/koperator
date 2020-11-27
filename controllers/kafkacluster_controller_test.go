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

package controllers

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

var _ = Describe("KafkaCluster", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		// create default Kafka cluster spec
		kafkaCluster = &v1beta1.KafkaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("kafkacluster-%v", count),
				Namespace: namespace,
			},
			Spec: v1beta1.KafkaClusterSpec{
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{},
					InternalListeners: []v1beta1.InternalListenerConfig{},
				},
				Brokers:     []v1beta1.Broker{},
				ZKAddresses: []string{},
			},
		}
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should pass", func() {
		// Expect(nil).To(BeNil())
	})
})

var _ = Describe("CruiseControlReconciler", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
		ccConfig     v1beta1.CruiseControlConfig
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-cc-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		// create default Kafka cluster spec
		kafkaCluster = &v1beta1.KafkaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("kafkacluster-%v", count),
				Namespace: namespace,
			},
			Spec: v1beta1.KafkaClusterSpec{
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{},
					InternalListeners: []v1beta1.InternalListenerConfig{},
				},
				Brokers:             []v1beta1.Broker{},
				ZKAddresses:         []string{},
				CruiseControlConfig: ccConfig,
			},
		}
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
	})

	It("reconciles without error when not enabled", func() {

	})

	When("CC is enabled", func() {
		BeforeEach(func() {
			// TODO test all of this
			ccConfig = v1beta1.CruiseControlConfig{
				CruiseControlTaskSpec:    v1beta1.CruiseControlTaskSpec{},
				CruiseControlEndpoint:    "test",
				Resources:                nil,
				ServiceAccountName:       "",
				ImagePullSecrets:         nil,
				NodeSelector:             nil,
				Tolerations:              nil,
				Config:                   "",
				CapacityConfig:           "",
				ClusterConfig:            "",
				Log4jConfig:              "",
				Image:                    "",
				TopicConfig:              nil,
				CruiseControlAnnotations: nil,
				InitContainers:           nil,
				Volumes:                  nil,
				VolumeMounts:             nil,
			}
		})

		It("reconciles correctly", func() {

		})
	})
})
