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

package kafka

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources"
	"github.com/banzaicloud/koperator/pkg/resources/kafka/mocks"
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
								CruiseControlState:              v1beta1.GracefulDownscaleRunning,
								CruiseControlOperationReference: &corev1.LocalObjectReference{Name: "test-ccopref"},
							},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleSucceeded,
								VolumeStates: map[string]v1beta1.VolumeState{
									"/path1": {
										CruiseControlVolumeState:        v1beta1.GracefulDiskRebalanceRunning,
										CruiseControlOperationReference: &corev1.LocalObjectReference{Name: "test-ccopref"},
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
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlState: v1beta1.GracefulDownscaleRunning,
							},
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
								CruiseControlState:              v1beta1.GracefulUpscaleRunning,
								CruiseControlOperationReference: &corev1.LocalObjectReference{Name: "test-ccopref"},
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
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlState: v1beta1.GracefulUpscaleRunning,
							},
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
							Id: 3,
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
								CruiseControlState:              v1beta1.GracefulDownscaleRunning,
								CruiseControlOperationReference: &corev1.LocalObjectReference{Name: "test-ccopref"},
							},
						},
						"1": {
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlState:              v1beta1.GracefulDownscaleRunning,
								CruiseControlOperationReference: &corev1.LocalObjectReference{Name: "test-ccopref"},
							},
						},
						"2": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
						"3": {
							GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
						},
						"4": {
							GracefulActionState: v1beta1.GracefulActionState{
								CruiseControlState:              v1beta1.GracefulDownscaleRunning,
								CruiseControlOperationReference: &corev1.LocalObjectReference{Name: "test-ccopref"},
							},
						},
					},
				},
			},
			expectedIDs: []int32{0, 1, 2, 3, 4},
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

	//t.Parallel()

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
		brokersPVC               corev1.PersistentVolumeClaimList
		desiredBrokers           []v1beta1.Broker
		brokersState             map[string]v1beta1.BrokerState
		controllerBrokerID       int32
		expectedReorderedBrokers []v1beta1.Broker
	}{
		{
			testName: "all broker pods are up an running with no controller broker",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "2"}}},
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
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "2"}}},
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
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "2"}}},
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
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "2"}}},
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
		{
			testName: "some missing broker pods, newly added brokers, and missing bokers with pvc and incomplete downscale operation",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "2"}}},
				},
			},
			brokersPVC: corev1.PersistentVolumeClaimList{
				TypeMeta: metav1.TypeMeta{},
				ListMeta: metav1.ListMeta{},
				Items: []corev1.PersistentVolumeClaim{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "5"}},
						Status: corev1.PersistentVolumeClaimStatus{
							Phase: corev1.ClaimBound,
						},
					},
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
					GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
					ConfigurationBackup: "H4sIAAxcrGIAA6tWykxRsjKt5QIAiMWU3gkAAAA=",
				}},
			controllerBrokerID: 1,
			expectedReorderedBrokers: []v1beta1.Broker{
				{Id: 5}, // broker pod 5 missing and there is incomplete downscale operation thus should have highest prio
				{Id: 6}, // broker 6 is newly added thus should have higher prio
				{Id: 3}, // broker pod 3 missing thus should have higher prio
				{Id: 4}, // broker pod 4 missing thus should have higher prio
				{Id: 0},
				{Id: 2},
				{Id: 1}, // controller broker should be last
			},
		},
		{
			testName: "some missing broker pods, newly added brokers, and missing bokers without pvc and incomplete downscale operation",
			brokerPods: corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "0"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "1"}}},
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1beta1.BrokerIdLabelKey: "2"}}},
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
					GracefulActionState: v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulDownscaleRequired},
					ConfigurationBackup: "H4sIAAxcrGIAA6tWykxRsjKt5QIAiMWU3gkAAAA=",
				}},
			controllerBrokerID: 1,
			expectedReorderedBrokers: []v1beta1.Broker{
				{Id: 6}, // broker 6 is newly added thus should have higher prio
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

			runningBrokers := make(map[string]struct{})
			for _, b := range test.brokerPods.Items {
				brokerID := b.GetLabels()[v1beta1.BrokerIdLabelKey]
				runningBrokers[brokerID] = struct{}{}
			}
			boundPersistentVolumeClaims := make(map[string]struct{})
			for _, pvc := range test.brokersPVC.Items {
				brokerID := pvc.GetLabels()[v1beta1.BrokerIdLabelKey]
				if pvc.Status.Phase == corev1.ClaimBound {
					boundPersistentVolumeClaims[brokerID] = struct{}{}
				}
			}

			reorderedBrokers := reorderBrokers(runningBrokers, boundPersistentVolumeClaims, test.desiredBrokers, test.brokersState, test.controllerBrokerID, logr.Discard())

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

func TestReconcileKafkaPvcDiskRemoval(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName            string
		brokersDesiredPvcs  map[string][]*corev1.PersistentVolumeClaim
		existingPvcs        []*corev1.PersistentVolumeClaim
		kafkaClusterSpec    v1beta1.KafkaClusterSpec
		kafkaClusterStatus  v1beta1.KafkaClusterStatus
		expectedError       bool
		expectedDeletePvc   bool
		expectedVolumeState map[string]v1beta1.CruiseControlVolumeState
	}{
		{
			testName: "If no disk removed, do nothing",
			brokersDesiredPvcs: map[string][]*corev1.PersistentVolumeClaim{
				"0": {
					createPvc("test-pvc-1", "0", "/path/to/mount1"),
					createPvc("test-pvc-2", "0", "/path/to/mount2"),
				},
			},
			existingPvcs: []*corev1.PersistentVolumeClaim{
				createPvc("test-pvc-1", "0", "/path/to/mount1"),
				createPvc("test-pvc-2", "0", "/path/to/mount2"),
			},
			kafkaClusterStatus:  v1beta1.KafkaClusterStatus{},
			expectedError:       false,
			expectedDeletePvc:   false,
			expectedVolumeState: nil,
		},
		{
			testName: "If disk removed, mark it as GracefulDiskRemovalRequired and return error",
			brokersDesiredPvcs: map[string][]*corev1.PersistentVolumeClaim{
				"0": {
					createPvc("test-pvc-1", "0", "/path/to/mount1"),
				},
			},
			existingPvcs: []*corev1.PersistentVolumeClaim{
				createPvc("test-pvc-1", "0", "/path/to/mount1"),
				createPvc("test-pvc-2", "0", "/path/to/mount2"),
			},
			kafkaClusterStatus: v1beta1.KafkaClusterStatus{
				BrokersState: map[string]v1beta1.BrokerState{
					"0": {
						GracefulActionState: v1beta1.GracefulActionState{
							VolumeStates: map[string]v1beta1.VolumeState{
								"/path/to/mount2": {
									CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceSucceeded,
								},
							},
						},
					},
				},
			},
			expectedError:     true,
			expectedDeletePvc: false,
			expectedVolumeState: map[string]v1beta1.CruiseControlVolumeState{
				"/path/to/mount2": v1beta1.GracefulDiskRemovalRequired,
			},
		},
		{
			testName: "If disk is rebalancing, wait for it to finish",
			brokersDesiredPvcs: map[string][]*corev1.PersistentVolumeClaim{
				"0": {
					createPvc("test-pvc-1", "0", "/path/to/mount1"),
				},
			},
			existingPvcs: []*corev1.PersistentVolumeClaim{
				createPvc("test-pvc-1", "0", "/path/to/mount1"),
				createPvc("test-pvc-2", "0", "/path/to/mount2"),
			},
			kafkaClusterStatus: v1beta1.KafkaClusterStatus{
				BrokersState: map[string]v1beta1.BrokerState{
					"0": {
						GracefulActionState: v1beta1.GracefulActionState{
							VolumeStates: map[string]v1beta1.VolumeState{
								"/path/to/mount2": {
									CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceScheduled,
								},
							},
						},
					},
				},
			},
			expectedError:     true,
			expectedDeletePvc: false,
			expectedVolumeState: map[string]v1beta1.CruiseControlVolumeState{
				"/path/to/mount2": v1beta1.GracefulDiskRebalanceScheduled,
			},
		},
		{
			testName: "Wait for disk removal to finish",
			brokersDesiredPvcs: map[string][]*corev1.PersistentVolumeClaim{
				"0": {
					createPvc("test-pvc-1", "0", "/path/to/mount1"),
				},
			},
			existingPvcs: []*corev1.PersistentVolumeClaim{
				createPvc("test-pvc-1", "0", "/path/to/mount1"),
				createPvc("test-pvc-2", "0", "/path/to/mount2"),
			},
			kafkaClusterStatus: v1beta1.KafkaClusterStatus{
				BrokersState: map[string]v1beta1.BrokerState{
					"0": {
						GracefulActionState: v1beta1.GracefulActionState{
							VolumeStates: map[string]v1beta1.VolumeState{
								"/path/to/mount2": {
									CruiseControlVolumeState: v1beta1.GracefulDiskRemovalRunning,
								},
							},
						},
					},
				},
			},
			expectedError:     true,
			expectedDeletePvc: false,
			expectedVolumeState: map[string]v1beta1.CruiseControlVolumeState{
				"/path/to/mount2": v1beta1.GracefulDiskRemovalRunning,
			},
		},
		{
			testName: "If disk removal successful, do not return error and delete pvc and volume state",
			brokersDesiredPvcs: map[string][]*corev1.PersistentVolumeClaim{
				"0": {
					createPvc("test-pvc-1", "0", "/path/to/mount1"),
				},
			},
			existingPvcs: []*corev1.PersistentVolumeClaim{
				createPvc("test-pvc-1", "0", "/path/to/mount1"),
				createPvc("test-pvc-2", "0", "/path/to/mount2"),
			},
			kafkaClusterStatus: v1beta1.KafkaClusterStatus{
				BrokersState: map[string]v1beta1.BrokerState{
					"0": {
						GracefulActionState: v1beta1.GracefulActionState{
							VolumeStates: map[string]v1beta1.VolumeState{
								"/path/to/mount2": {
									CruiseControlVolumeState: v1beta1.GracefulDiskRemovalSucceeded,
								},
							},
						},
					},
				},
			},
			expectedError:       false,
			expectedDeletePvc:   true,
			expectedVolumeState: nil,
		},
		{
			testName: "If disk removal failed, and it is readded, mark the disk as rebalancing",
			brokersDesiredPvcs: map[string][]*corev1.PersistentVolumeClaim{
				"0": {
					createPvc("test-pvc-1", "0", "/path/to/mount1"),
					createPvc("test-pvc-2", "0", "/path/to/mount2"),
				},
			},
			existingPvcs: []*corev1.PersistentVolumeClaim{
				createPvc("test-pvc-1", "0", "/path/to/mount1"),
				createPvc("test-pvc-2", "0", "/path/to/mount2"),
			},
			kafkaClusterStatus: v1beta1.KafkaClusterStatus{
				BrokersState: map[string]v1beta1.BrokerState{
					"0": {
						GracefulActionState: v1beta1.GracefulActionState{
							VolumeStates: map[string]v1beta1.VolumeState{
								"/path/to/mount2": {
									CruiseControlVolumeState: v1beta1.GracefulDiskRemovalCompletedWithError,
								},
							},
						},
					},
				},
			},
			expectedError:     false,
			expectedDeletePvc: false,
			expectedVolumeState: map[string]v1beta1.CruiseControlVolumeState{
				"/path/to/mount2": v1beta1.GracefulDiskRebalanceRequired,
			},
		},
	}

	for _, test := range testCases {
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
					},
				},
			}

			// Set up the mockClient to return the provided test.existingPvcs
			mockClient.On(
				"List",
				context.TODO(),
				mock.IsType(&corev1.PersistentVolumeClaimList{}),
				client.InNamespace("kafka"),
				mock.AnythingOfType("client.MatchingLabels"),
			).Run(func(args mock.Arguments) {
				arg := args.Get(1).(*corev1.PersistentVolumeClaimList)

				// Convert []*corev1.PersistentVolumeClaim to []corev1.PersistentVolumeClaim
				pvcItems := make([]corev1.PersistentVolumeClaim, len(test.existingPvcs))
				for i, pvc := range test.existingPvcs {
					pvcItems[i] = *pvc
				}

				arg.Items = pvcItems
			}).Return(nil)

			// Mock the client.Delete call
			mockClient.On("Delete", context.TODO(), mock.AnythingOfType("*v1.PersistentVolumeClaim")).Return(nil)

			// Mock the status update call
			mockClient.On("Status").Return(mockClient)
			mockClient.On("Update", context.TODO(), mock.AnythingOfType("*v1beta1.KafkaCluster")).Run(func(args mock.Arguments) {
				arg := args.Get(1).(*v1beta1.KafkaCluster)
				r.KafkaCluster.Status = arg.Status
			}).Return(nil)

			// Set up the r.KafkaCluster.Status with the provided test.kafkaClusterStatus
			r.KafkaCluster.Status = test.kafkaClusterStatus

			// Call the reconcileKafkaPvc function with the provided test.brokersDesiredPvcs
			err := r.reconcileKafkaPvc(context.TODO(), logf.Log, test.brokersDesiredPvcs)

			// Test that the expected error is returned
			if test.expectedError {
				assert.NotNil(t, err, "Expected an error but got nil")
			} else {
				assert.Nil(t, err, "Expected no error but got an error")
			}

			// Test that PVC is deleted if expected
			if test.expectedDeletePvc {
				mockClient.AssertCalled(t, "Delete", context.TODO(), mock.AnythingOfType("*v1.PersistentVolumeClaim"))
			} else {
				mockClient.AssertNotCalled(t, "Delete", context.TODO(), mock.AnythingOfType("*v1.PersistentVolumeClaim"))
			}

			// Test that the expected volume state is set
			brokerState := r.KafkaCluster.Status.BrokersState["0"]
			if test.expectedVolumeState != nil {
				for mountPath, expectedState := range test.expectedVolumeState {
					actualState, exists := brokerState.GracefulActionState.VolumeStates[mountPath]
					assert.True(t, exists, "Expected volume state not found for mount path %s", mountPath)
					assert.Equal(t, expectedState, actualState.CruiseControlVolumeState, "Volume state mismatch for mount path %s", mountPath)
				}
			}
		})
	}
}

//nolint:unparam
func createPvc(name, brokerId, mountPath string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				v1beta1.BrokerIdLabelKey: brokerId,
			},
			Annotations: map[string]string{
				"mountPath": mountPath,
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
}
