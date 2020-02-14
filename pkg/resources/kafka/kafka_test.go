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
	"reflect"
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
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
