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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

var _ = Describe("KafkaClusterEnvoyController", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-envoy-%v", count)
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

	When("envoy is not enabled", func() {
		It("does not create envoy related objects", func() {

		})
	})

	When("envoy is enabled", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.IngressController = "envoy"

			kafkaCluster.Spec.ListenersConfig = v1beta1.ListenersConfig{
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Name:          "test",
							ContainerPort: 9733,
						},
						ExternalStartingPort: 11202,
						HostnameOverride:     "test-host",
						// ServiceAnnotations:   nil,
					},
				},
				InternalListeners: []v1beta1.InternalListenerConfig{},
			}
		})

		It("creates envoy related objects", func() {
			// TODO assert loadbalancer
			var loadBalancer corev1.Service
			lbName := fmt.Sprintf("envoy-loadbalancer-test-%s", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: lbName}, &loadBalancer)
				return err
			}).Should(Succeed())

			// TODO assert configmap
			var configMap corev1.ConfigMap
			configMapName := fmt.Sprintf("envoy-config-test-%s", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: configMapName}, &configMap)
				return err
			}).Should(Succeed())

			// TODO assert deployment
			var deployment appsv1.Deployment
			deploymentName := fmt.Sprintf("envoy-config-test-%s", kafkaCluster.Name)
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, &deployment)
				return err
			}).Should(Succeed())
		})
	})
})
