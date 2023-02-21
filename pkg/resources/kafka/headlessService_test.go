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

func TestIsNodePortAccessMethodInUseAmongExternalListeners(t *testing.T) {
	testCases := []struct {
		testName                                                string
		externalListeners                                       []v1beta1.ExternalListenerConfig
		expectedNodePortAccessMethodInUseAmongExternalListeners bool
	}{
		{
			testName: "All external listeners use NodePort access method",
			externalListeners: []v1beta1.ExternalListenerConfig{
				{
					AccessMethod: corev1.ServiceTypeNodePort,
				},
				{
					AccessMethod: corev1.ServiceTypeNodePort,
				},
				{
					AccessMethod: corev1.ServiceTypeNodePort,
				},
			},
			expectedNodePortAccessMethodInUseAmongExternalListeners: true,
		},
		{
			testName: "All external listeners use non-NodePort access method",
			externalListeners: []v1beta1.ExternalListenerConfig{
				{
					AccessMethod: corev1.ServiceTypeLoadBalancer,
				},
				{
					AccessMethod: corev1.ServiceTypeClusterIP,
				},
			},
			expectedNodePortAccessMethodInUseAmongExternalListeners: false,
		},
		{
			testName: "External listeners with mixed usages of NodePort and non-NodePort access methods",
			externalListeners: []v1beta1.ExternalListenerConfig{
				{
					AccessMethod: corev1.ServiceTypeLoadBalancer,
				},
				{
					AccessMethod: corev1.ServiceTypeNodePort,
				},
			},
			expectedNodePortAccessMethodInUseAmongExternalListeners: true,
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			actualNodePortAccessMethodInUseAmongExternalListeners := isNodePortAccessMethodInUseAmongExternalListeners(test.externalListeners)

			require.Equal(t, test.expectedNodePortAccessMethodInUseAmongExternalListeners, actualNodePortAccessMethodInUseAmongExternalListeners)
		})
	}
}

func TestNonNodePortServices(t *testing.T) {
	testCases := []struct {
		testName                    string
		services                    corev1.ServiceList
		expectedNonNodePortServices corev1.ServiceList
	}{
		{
			testName: "Services with mixed NodePort and non-NodePort services that are evenly distributed",
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
							Name: "Non-NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
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
							Name: "Non-NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
			expectedNonNodePortServices: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
		},
		{
			testName: "Services with mixed NodePort and non-NodePort services that are non-evenly distributed",
			services: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
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
							Name: "Non-NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
			expectedNonNodePortServices: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 2",
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "NodePort service 3",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
						},
					},
				},
			},
			expectedNonNodePortServices: corev1.ServiceList{},
		},
		{
			testName: "Services with only non-NodePort services",
			services: corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 1",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 3",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
				},
			},
			expectedNonNodePortServices: corev1.ServiceList{
				Items: []corev1.Service{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Non-NodePort service 1",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
				},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 2",
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "Non-NodePort service 3",
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
			actualNonNodePortServices := nonNodePortServices(test.services)

			require.Equal(t, test.expectedNonNodePortServices, actualNonNodePortServices)
		})
	}
}
