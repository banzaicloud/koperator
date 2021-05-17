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

package kafka

import (
	"context"
	"reflect"
	"testing"

	"errors"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources"
	mocks "github.com/banzaicloud/koperator/pkg/resources/kafka/mocks"
)

func TestGetBrokersWithPendingOrRunningCCTask(t *testing.T) {
	testCases := []struct {
		testName     string
		kafkaCluster v1beta1.KafkaCluster
		expectedIDs  []int32
	}{
		{
			testName: "pending and running CC rebalance tasks",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
						{
							Id: 4,
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState: map[string]v1beta1.BrokerState{
						"0": {
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlTaskId: "cc-task-id-1",
								CruiseControlState:  v1beta1.GracefulDownscaleRunning,
							},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded,
								VolumeStates: map[string]v1beta1.VolumeState{
									"/path1": {
										CruiseControlTaskId:      "1",
										CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRunning,
									}}},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded,
								VolumeStates: map[string]v1beta1.VolumeState{
									"/path1": {
										CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired,
									}}},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
						"4": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRunning},
						},
					},
				},
			},
			expectedIDs: []int32{0, 1, 2},
		},
		{
			testName: "no pending or running CC upscale tasks",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState: map[string]v1beta1.BrokerState{
						"0": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRequired},
						},
					},
				},
			},
		},
		{
			testName: "pending and running CC upscale tasks",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
						{
							Id: 4,
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState: map[string]v1beta1.BrokerState{
						"0": {
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlTaskId: "cc-task-id-1",
								CruiseControlState:  v1beta1.GracefulUpscaleRunning,
							},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRunning},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRequired},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRequired},
						},
						"4": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRunning},
						},
					},
				},
			},
			expectedIDs: []int32{0, 2},
		},
		{
			testName: "no pending or running CC downscale tasks",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState: map[string]v1beta1.BrokerState{
						"0": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
					},
				},
			},
		},
		{
			testName: "pending and running CC downscale tasks",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
						{
							Id: 4,
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState: map[string]v1beta1.BrokerState{
						"0": {
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlTaskId: "cc-task-id-1",
								CruiseControlState:  v1beta1.GracefulDownscaleRunning,
							},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRunning},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
						"4": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRunning},
						},
					},
				},
			},
			expectedIDs: []int32{0, 2},
		},
		{
			testName: "no pending or running CC rebalance tasks",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState: map[string]v1beta1.BrokerState{
						"0": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded,
								VolumeStates: map[string]v1beta1.VolumeState{
									"/path1": {
										CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceSucceeded,
									}}},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
					},
				},
			},
		},
	}

	t.Parallel()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			actual := GetBrokersWithPendingOrRunningCCTask(&test.kafkaCluster)

			if !reflect.DeepEqual(actual, test.expectedIDs) {
				t.Error("Expected:", test.expectedIDs, ", got:", actual)
			}
		})
	}
}

func TestReorderBrokers(t *testing.T) {
	testCases := []struct {
		testName                 string
		brokerPods               corev1.PodList
		desiredBrokers           []v1beta1.Broker
		brokersState             map[string]v1beta1.BrokerState
		controllerBrokerID       int32
		expectedReorderedBrokers []v1beta1.Broker
	}{
		{
			testName: "all broker pods are up an running with no controller broker",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "2"}}},
				},
			},
			desiredBrokers: []v1beta1.Broker{
				{Id: 0},
				{Id: 1},
				{Id: 2},
			},
			brokersState: map[string]v1beta1.BrokerState{
				"0": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"1": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"2": {ConfigurationState: v1beta1.ConfigOutOfSync},
			},
			controllerBrokerID: -1, // no controller broker
			expectedReorderedBrokers: []v1beta1.Broker{
				{Id: 0},
				{Id: 1},
				{Id: 2},
			},
		},
		{
			testName: "all broker pods are up an running with controller broker",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "2"}}},
				},
			},
			desiredBrokers: []v1beta1.Broker{
				{Id: 0},
				{Id: 1},
				{Id: 2},
			},
			brokersState: map[string]v1beta1.BrokerState{
				"0": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"1": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"2": {ConfigurationState: v1beta1.ConfigOutOfSync},
			},
			controllerBrokerID: 1,
			expectedReorderedBrokers: []v1beta1.Broker{
				{Id: 0},
				{Id: 2},
				{Id: 1}, // controller broker should be last
			},
		},
		{
			testName: "some missing broker pods",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "2"}}},
				},
			},
			desiredBrokers: []v1beta1.Broker{
				{Id: 0},
				{Id: 1},
				{Id: 2},
				{Id: 3},
				{Id: 4},
			},
			brokersState: map[string]v1beta1.BrokerState{
				"0": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"1": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"2": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"3": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"4": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"5": {
					ConfigurationState:  v1beta1.ConfigInSync,
					GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired}},
			},
			controllerBrokerID: 1,
			expectedReorderedBrokers: []v1beta1.Broker{
				{Id: 3}, // broker pod 3 missing thus should have higher prio
				{Id: 4}, // broker pod 4 missing thus should have higher prio
				{Id: 0},
				{Id: 2},
				{Id: 1}, // controller broker should be last
			},
		},
		{
			testName: "some missing broker pods and newly added brokers",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"brokerId": "2"}}},
				},
			},
			desiredBrokers: []v1beta1.Broker{
				{Id: 0},
				{Id: 1},
				{Id: 2},
				{Id: 3},
				{Id: 4},
				{Id: 6}, // Kafka cluster upscaled by adding broker 6 to it
			},
			brokersState: map[string]v1beta1.BrokerState{
				"0": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"1": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"2": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"3": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"4": {ConfigurationState: v1beta1.ConfigOutOfSync},
				"5": {
					ConfigurationState:  v1beta1.ConfigInSync,
					GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired}},
			},
			controllerBrokerID: 1,
			expectedReorderedBrokers: []v1beta1.Broker{
				{Id: 6}, // broker 6 is newly added thus should have the highest prio
				{Id: 3}, // broker pod 3 missing thus should have higher prio
				{Id: 4}, // broker pod 4 missing thus should have higher prio
				{Id: 0},
				{Id: 2},
				{Id: 1}, // controller broker should be last
			},
		},
	}

	t.Parallel()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			g := gomega.NewWithT(t)
			reorderedBrokers := reorderBrokers(test.brokerPods, test.desiredBrokers, test.brokersState, test.controllerBrokerID)

			g.Expect(reorderedBrokers).To(gomega.Equal(test.expectedReorderedBrokers))
		})
	}
}

func TestGetServerPasswordKeysAndUsers(t *testing.T) { //nolint funlen
	t.Parallel()
	testCases := []struct {
		testName string
		// secrets is pairing listenerName with secret
		secrets           map[string]corev1.Secret
		internalListeners []v1beta1.InternalListenerConfig
		externalListeners []v1beta1.ExternalListenerConfig
		SSLSecrets        *v1beta1.SSLSecrets
	}{
		{
			testName: "secretmissingdatafield",
			secrets: map[string]corev1.Secret{
				"internal_auto": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
			},
			internalListeners: []v1beta1.InternalListenerConfig{{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:                            v1beta1.SecurityProtocol("ssl"),
					Name:                            "internal_auto",
					ContainerPort:                   9092,
					UsedForInnerBrokerCommunication: false,
				},
			},
			},
			SSLSecrets: &v1beta1.SSLSecrets{},
		},
		{
			testName: "autogenerated",
			secrets: map[string]corev1.Secret{
				"internal_auto": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
				"external_auto": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
			},
			externalListeners: []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "external_auto",
						ContainerPort: 9092,
					},
				},
			},
			internalListeners: []v1beta1.InternalListenerConfig{{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:                            v1beta1.SecurityProtocol("ssl"),
					Name:                            "internal_auto",
					ContainerPort:                   9092,
					UsedForInnerBrokerCommunication: false,
				},
			},
			},
			SSLSecrets: &v1beta1.SSLSecrets{},
		},
		{
			testName: "autogenerated-and-custom",
			secrets: map[string]corev1.Secret{
				"external_auto": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
				"internal_auto_1": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
				"internal_auto_2": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
				"internal_auto_3": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
				"internal_custom": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "customcert",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("custompass")},
				},
				"external_custom": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "customcert-external",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("custompass_external")},
				},
			},
			externalListeners: []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "external_custom",
						ContainerPort: 9092,
						ServerSSLCertSecret: &corev1.LocalObjectReference{
							Name: "customcert-external",
						},
					},
				},
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "external_auto",
						ContainerPort: 9092,
					},
				},
			},
			internalListeners: []v1beta1.InternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:                            v1beta1.SecurityProtocol("ssl"),
						Name:                            "internal_auto_1",
						ContainerPort:                   9092,
						UsedForInnerBrokerCommunication: false,
					},
				},
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:                            v1beta1.SecurityProtocol("ssl"),
						Name:                            "internal_auto_2",
						ContainerPort:                   9092,
						UsedForInnerBrokerCommunication: false,
					},
				},
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:                            v1beta1.SecurityProtocol("ssl"),
						Name:                            "internal_auto_3",
						ContainerPort:                   9092,
						UsedForInnerBrokerCommunication: false,
					},
				},
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "internal_custom",
						ContainerPort: 9093,
						ServerSSLCertSecret: &corev1.LocalObjectReference{
							Name: "customcert",
						},
						UsedForInnerBrokerCommunication: false,
					},
				},
			},
			SSLSecrets: &v1beta1.SSLSecrets{},
		},
		{
			testName: "custom",
			secrets: map[string]corev1.Secret{
				"internal_generated": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-server-certificate",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("generatedpass")},
				},
				"internal_custom_1": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "customcert",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("custompass")},
				},
				"internal_custom_2": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "customcert",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("custompass")},
				},
				"external_custom": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "customcert-external",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("custompass_external")},
				},
			},
			externalListeners: []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "external_custom",
						ContainerPort: 9092,
						ServerSSLCertSecret: &corev1.LocalObjectReference{
							Name: "customcert-external",
						},
					},
				},
			},
			internalListeners: []v1beta1.InternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "internal_custom_1",
						ContainerPort: 9092,
						ServerSSLCertSecret: &corev1.LocalObjectReference{
							Name: "customcert",
						},
						UsedForInnerBrokerCommunication: false,
					},
				},
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "internal_custom_2",
						ContainerPort: 9093,
						ServerSSLCertSecret: &corev1.LocalObjectReference{
							Name: "customcert",
						},
						UsedForInnerBrokerCommunication: false,
					},
				},
			},
			SSLSecrets: &v1beta1.SSLSecrets{},
		},
		{
			testName: "missingsecret",
			secrets: map[string]corev1.Secret{
				"internal_custom_1": {

					ObjectMeta: metav1.ObjectMeta{
						Name:      "customcert",
						Namespace: "kafka",
					},
					Data: map[string][]byte{v1alpha1.PasswordKey: []byte("custompass")},
				},
			},
			internalListeners: []v1beta1.InternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:                            v1beta1.SecurityProtocol("ssl"),
						Name:                            "internal_custom_1",
						ContainerPort:                   9092,
						UsedForInnerBrokerCommunication: false,
					},
				},
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:          v1beta1.SecurityProtocol("ssl"),
						Name:          "internal_custom_2",
						ContainerPort: 9093,
						ServerSSLCertSecret: &corev1.LocalObjectReference{
							Name: "customcert",
						},
						UsedForInnerBrokerCommunication: false,
					},
				},
			},
		},
	}
	for _, test := range testCases {
		test := test
		mockClient := new(mocks.Client)

		t.Run(test.testName, func(t *testing.T) {
			r := Reconciler{
				Reconciler: resources.Reconciler{
					Client: mockClient,
					KafkaCluster: &v1beta1.KafkaCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kafka",
							Namespace: "kafka",
						},
						Spec: v1beta1.KafkaClusterSpec{
							ListenersConfig: v1beta1.ListenersConfig{
								ExternalListeners: test.externalListeners,
								InternalListeners: test.internalListeners,
								SSLSecrets:        test.SSLSecrets,
							},
							Brokers: []v1beta1.Broker{},
						},
					},
				},
			}
			expectedServerPasses := make(map[string]string)
			serverSecret := &corev1.Secret{}
			// expectedGetCount is counting the expected Client.Get calls
			// client.Get shouldn't be called more than once even if multiple listeners are using autogenerated SSL cert
			expectedGetCount := 0
			first := true
			for _, listener := range test.internalListeners {
				if listener.CommonListenerSpec.Type.IsSSL() {
					if listener.CommonListenerSpec.GetServerSSLCertSecretName() != "" {
						expectedGetCount++
					} else if first {
						first = false
						expectedGetCount++
					}
					// generate the expected listener-password pairs
					expectedServerPasses[listener.Name] = string(test.secrets[listener.Name].Data[v1alpha1.PasswordKey])
				}
			}

			for _, listener := range test.externalListeners {
				if listener.CommonListenerSpec.Type.IsSSL() {
					if listener.CommonListenerSpec.GetServerSSLCertSecretName() != "" {
						expectedGetCount++
					} else if first {
						first = false
						expectedGetCount++
					}

					expectedServerPasses[listener.Name] = string(test.secrets[listener.Name].Data[v1alpha1.PasswordKey])
				}
			}

			for _, secret := range test.secrets {
				secretDeepCopy := secret.DeepCopy()
				mockCall := mockClient.On("Get", context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: "kafka"}, serverSecret)
				mockCall.RunFn = func(args mock.Arguments) {
					arg := args.Get(2).(*corev1.Secret)
					arg.Name = secretDeepCopy.Name
					arg.Data = secretDeepCopy.Data
					if test.testName != "secretmissingdatafield" {
						arg.Data[v1alpha1.TLSJKSKeyStore] = []byte("sdf")
					}
					arg.Data[v1alpha1.TLSJKSTrustStore] = []byte("sdf")
				}
				mockCall.Return(nil)
			}
			mockClient.On("Get", context.TODO(), mock.AnythingOfType("types.NamespacedName"), serverSecret).Return(errors.New("server secret not ready"))

			serverPasswords, _, err := r.getServerPasswordKeysAndUsers()
			if test.testName != "missingsecret" && test.testName != "secretmissingdatafield" {
				assert.Nil(t, err, err)
				assert.NotEqual(t, len(serverPasswords), 0, "serverPasswords shouldn't be empty")
				// compare the func generated with the expected
				eq := reflect.DeepEqual(serverPasswords, expectedServerPasses)
				assert.Equal(t, eq, true, "generated serverPasswords should equal with the expected")
				// client.Get shouldn't be called more than once even if multiple listeners are using autogenerated SSL cert
				mockClient.AssertNumberOfCalls(t, "Get", expectedGetCount)
			} else {
				assert.Equal(t, len(serverPasswords), 0, "serverPasswords should be empty")
				assert.NotNil(t, err, err)
			}
		})
	}
}
