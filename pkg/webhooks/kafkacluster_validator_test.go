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

package webhooks

import (
	"testing"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

//nolint: funlen
func TestCheckBrokerStorageRemoval(t *testing.T) {
	testCases := []struct {
		testName            string
		kafkaClusterSpecNew v1beta1.KafkaClusterSpec
		kafkaClusterSpecOld v1beta1.KafkaClusterSpec
		isValid             bool
	}{
		{
			testName: "there is no storage remove",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "there is no storage remove but there is broker remove",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
					{
						Id:                2,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "when there is storage remove but there is broker remove also",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
					{
						Id:                2,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs6"},
							},
						},
					},
				},
			},
			isValid: true,
		},
		{
			testName: "when there is storage remove from another brokerConfigBroup",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
					"default2": {
						StorageConfigs: []v1beta1.StorageConfig{
							//	v1beta1.StorageConfig{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default2",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: false,
		},
		{
			testName: "when there is storage remove",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							//v1beta1.StorageConfig{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: false,
		},
		{
			testName: "when added a new one",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							//v1beta1.StorageConfig{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "when only sequence has changed",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs3"},
							{MountPath: "logs2"},
							{MountPath: "logs1"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "when there is perBroker storageconfigs and there is no storage remove",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs6"},
							},
						},
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs6"},
							},
						},
					},
				},
			},
			isValid: true,
		},
		{
			testName: "when there is perBroker config and added new and removed old",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs6"},
							},
						},
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs8"},
							},
						},
					},
				},
			},
			isValid: false,
		},
		{
			testName: "when there is no such brokerConfigGroup",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "notExists",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs6"},
							},
						},
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs4"},
								{MountPath: "logs5"},
								{MountPath: "logs8"},
							},
						},
					},
				},
			},
			isValid: false,
		},
	}

	for _, testCase := range testCases {
		res, err := checkBrokerStorageRemoval(&testCase.kafkaClusterSpecOld, &testCase.kafkaClusterSpecNew)
		if err != nil {
			t.Errorf("err should be nil, got %s", err)
		}
		if res != nil && testCase.isValid {
			t.Errorf("Message: %s, testName: %s", res.Error(), testCase.testName)
		} else if res == nil && !testCase.isValid {
			t.Errorf("there should be storage removal, testName: %s", testCase.testName)
		}
	}
}
