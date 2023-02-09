// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func TestCheckIfNodePortSvcNeeded(t *testing.T) {
	testReconciler := Reconciler{}

	testCases := []struct {
		testName            string
		kafkaCluster        *v1beta1.KafkaCluster
		isNodePortSvcNeeded bool
	}{
		{
			testName: "KafkaCluster with all external listeners using NodePort",
			kafkaCluster: &v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					ListenersConfig: v1beta1.ListenersConfig{
						ExternalListeners: []v1beta1.ExternalListenerConfig{
							{
								AccessMethod: corev1.ServiceTypeNodePort,
							},
							{
								AccessMethod: corev1.ServiceTypeNodePort,
							},
						},
					},
				},
			},
			isNodePortSvcNeeded: true,
		},
		{
			testName: "KafkaCluster with all external listeners using non-NodePort",
			kafkaCluster: &v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					ListenersConfig: v1beta1.ListenersConfig{
						ExternalListeners: []v1beta1.ExternalListenerConfig{
							{
								AccessMethod: corev1.ServiceTypeLoadBalancer,
							},
							{
								AccessMethod: corev1.ServiceTypeClusterIP,
							},
						},
					},
				},
			},
			isNodePortSvcNeeded: false,
		},
		{
			testName: "KafkaCluster with external listeners of mixed usages of NodePort and non-NodePort",
			kafkaCluster: &v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					ListenersConfig: v1beta1.ListenersConfig{
						ExternalListeners: []v1beta1.ExternalListenerConfig{
							{
								AccessMethod: corev1.ServiceTypeLoadBalancer,
							},
							{
								AccessMethod: corev1.ServiceTypeNodePort,
							},
						},
					},
				},
			},
			isNodePortSvcNeeded: true,
		},
	}

	for _, test := range testCases {
		testReconciler.KafkaCluster = test.kafkaCluster
		got := testReconciler.checkIfNodePortSvcNeeded()
		t.Run(test.testName, func(t *testing.T) {
			if got != test.isNodePortSvcNeeded {
				t.Errorf("Expected: %v  Got: %v", test.isNodePortSvcNeeded, got)
			}
		})
	}
}

func TestGetNonNodePortSvc(t *testing.T) {
	testCases := []struct {
		testName         string
		services         corev1.ServiceList
		filteredServices corev1.ServiceList
	}{
		{
			testName: "Services with mixed NodePort and non-NodePort services",
			services: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "LoadBalancer service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ClusterIP service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
			filteredServices: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "LoadBalancer service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ClusterIP service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
		},
		{
			testName: "Services with only NodePort services",
			services: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
						},
					},
				},
			},
			filteredServices: corev1.ServiceList{},
		},
		{
			testName: "Services with only non-NodePort services",
			services: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "LoadBalancer service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ClusterIP service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
			filteredServices: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "LoadBalancer service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ClusterIP service",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			got := getNonNodePortSvc(test.services)
			require.Equal(t, test.filteredServices, got)
		})
	}
}
