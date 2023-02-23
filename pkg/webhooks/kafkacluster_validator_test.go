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

package webhooks

import (
	"strings"
	"testing"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

// nolint: funlen
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
			t.Errorf("testName: %s, err should be nil, got %s", testCase.testName, err)
		}
		if res != nil && testCase.isValid {
			t.Errorf("Message: %s, testName: %s", res.Error(), testCase.testName)
		} else if res == nil && !testCase.isValid {
			t.Errorf("there should be storage removal, testName: %s", testCase.testName)
		}
	}
}

func TestCheckExternalListenerStartingPort(t *testing.T) {
	testCases := []struct {
		testName         string
		kafkaClusterSpec v1beta1.KafkaClusterSpec
		isValid          bool
	}{
		{
			// In this test case, all resulting external port numbers should be valid
			testName: "valid-2-brokers-2-externalListeners",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 900},
					{Id: 901},
					{Id: 902},
				},
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external1"},
							ExternalStartingPort: 19090,
						},
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external2"},
							ExternalStartingPort: 29090,
						},
					},
				},
			},
			isValid: true,
		},
		{
			// In this test case, both externalListeners have an externalStartinPort that is already >65535
			// so both should generate field.Error's for all brokers/brokerIDs
			testName: "invalid-2-brokers-2-huge-externalListeners",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 900},
					{Id: 901},
					{Id: 902},
				},
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external1"},
							ExternalStartingPort: 79090,
						},
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external2"},
							ExternalStartingPort: 89090,
						},
					},
				},
			},
			isValid: false,
		},
		{
			// In this test case:
			// - external1 should be invalid for brokers [11, 102] but not [0] (sum is not >65535)
			// - external2 should be invalid for brokers [102] but not [0, 11]
			testName: "partially-invalid-2-brokers-2-atlimit-externalListeners",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 0},
					{Id: 11},
					{Id: 102},
				},
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external1"},
							ExternalStartingPort: 65535,
						},
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external2"},
							ExternalStartingPort: 65434,
						},
					},
				},
			},
			isValid: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			got := checkExternalListenerStartingPort(&testCase.kafkaClusterSpec)
			switch {
			case testCase.isValid && got != nil:
				t.Errorf("Message: %s, testName: %s", got.ToAggregate().Error(), testCase.testName)
			case !testCase.isValid && got == nil:
				t.Errorf("Message: %s, testName: %s", got.ToAggregate().Error(), testCase.testName)
			}
		})
	}
}

// This test enforces the use of the dedicated sentinel error string in the checkExternalListenerStartingPort
func TestCheckExternalListenerStartingPort_errorstring(t *testing.T) {
	testCases := []struct {
		testName         string
		kafkaClusterSpec v1beta1.KafkaClusterSpec
	}{
		{
			// In this test case, the resulting field.ErrorList should have 1 element
			testName: "invalid-ports",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 1},
				},
				ListenersConfig: v1beta1.ListenersConfig{
					ExternalListeners: []v1beta1.ExternalListenerConfig{
						{
							CommonListenerSpec:   v1beta1.CommonListenerSpec{Name: "test-external1"},
							ExternalStartingPort: 65535,
						},
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			got := checkExternalListenerStartingPort(&testCase.kafkaClusterSpec)
			for _, fldErr := range got {
				if !strings.Contains(fldErr.Error(), invalidExternalListenerStartingPortErrMsg) {
					t.Errorf("Error %q does not contain the sentinel error string %q", fldErr, invalidExternalListenerStartingPortErrMsg)
				}
			}
		})
	}
}

func TestCheckUniqueListenerContainerPort(t *testing.T) {
	testCases := []struct {
		testName  string
		listeners v1beta1.ListenersConfig
		isValid   bool
	}{
		{
			testName: "unique_values",
			listeners: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal1", ContainerPort: 29092},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal2", ContainerPort: 29093},
					},
				},
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-external1", ContainerPort: 9094},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-external2", ContainerPort: 9095},
					},
				},
			},
			isValid: true,
		},
		{
			testName: "non-unique_values_inside_internalListener",
			listeners: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal1", ContainerPort: 29092},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal2", ContainerPort: 29092},
					},
				},
			},
			isValid: false,
		},
		{
			testName: "non-unique_values_inside_externalListener",
			listeners: v1beta1.ListenersConfig{
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-external1", ContainerPort: 9094},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-external2", ContainerPort: 9094},
					},
				},
			},
			isValid: false,
		},
		{
			testName: "non-unique_values_across_listener_types",
			listeners: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal1", ContainerPort: 39098},
					},
				},
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-external1", ContainerPort: 39098},
					},
				},
			},
			isValid: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			got := checkUniqueListenerContainerPort(testCase.listeners)
			switch {
			case testCase.isValid && got != nil:
				t.Errorf("Message: %s, testName: %s", got.ToAggregate().Error(), testCase.testName)
			case !testCase.isValid && got == nil:
				t.Errorf("Message: %s, testName: %s", got.ToAggregate().Error(), testCase.testName)
			}
		})
	}
}
