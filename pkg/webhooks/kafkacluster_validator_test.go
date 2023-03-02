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
	"fmt"
	"testing"

	"github.com/banzaicloud/koperator/api/v1beta1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

func TestCheckUniqueListenerContainerPort(t *testing.T) {
	testCases := []struct {
		testName  string
		listeners v1beta1.ListenersConfig
		expected  field.ErrorList
	}{
		{
			testName: "unique values",
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
			expected: nil,
		},
		{
			testName: "non-unique containerPorts with only internalListeners",
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
			expected: append(field.ErrorList{},
				field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("internalListeners").Index(1).Child("containerPort"), int32(29092))),
		},
		{
			testName: "non-unique containerPorts with only externalListeners",
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
			expected: append(field.ErrorList{},
				field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(1).Child("containerPort"), int32(9094))),
		},
		{
			testName: "non-unique containerPorts across both listener types single error",
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
			expected: append(field.ErrorList{},
				field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(0).Child("containerPort"), int32(39098))),
		},
		{
			testName: "non-unique containerPorts across both listener types two errors",
			listeners: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal1", ContainerPort: 39098},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-internal2", ContainerPort: 39098},
					},
				},
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{Name: "test-external1", ContainerPort: 39098},
					},
				},
			},
			expected: append(field.ErrorList{},
				field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("internalListeners").Index(1).Child("containerPort"), int32(39098)),
				field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(0).Child("containerPort"), int32(39098)),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			got := checkUniqueListenerContainerPort(testCase.listeners)
			require.Equal(t, testCase.expected, got)
		})
	}
}

func TestCheckExternalListenerStartingPort(t *testing.T) {
	testCases := []struct {
		testName         string
		kafkaClusterSpec v1beta1.KafkaClusterSpec
		expected         field.ErrorList
	}{
		{
			// In this test case, all resulting external port numbers should be valid
			testName: "valid config: 3 brokers with 2 externalListeners",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{{Id: 900}, {Id: 901}, {Id: 902}},
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
			expected: nil,
		},
		{
			// In this test case, both externalListeners have an externalStartinPort that is already >65535
			// so both should generate field.Error's for all brokers/brokerIDs
			testName: "invalid config: 3 brokers with 2 out-of-range externalListeners",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{{Id: 900}, {Id: 901}, {Id: 902}},
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
			expected: append(field.ErrorList{},
				field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(0).Child("externalStartingPort"), int32(79090),
					invalidExternalListenerStartingPortErrMsg+": "+fmt.Sprintf("ExternalListener '%s' would generate external access port numbers (externalStartingPort + Broker ID) that are out of range (not between 1 and 65535) for brokers %v",
						"test-external1", []int32{900, 901, 902})),
				field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(1).Child("externalStartingPort"), int32(89090),
					invalidExternalListenerStartingPortErrMsg+": "+fmt.Sprintf("ExternalListener '%s' would generate external access port numbers (externalStartingPort + Broker ID) that are out of range (not between 1 and 65535) for brokers %v",
						"test-external2", []int32{900, 901, 902})),
			),
		},
		{
			// In this test case:
			// - external1 should be invalid for brokers [11, 102] but not [0] (sum is not >65535)
			// - external2 should be invalid for brokers [102] but not [0, 11]
			testName: "invalid config: 3 brokers with 2 at-the-limit externalListeners",
			kafkaClusterSpec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{{Id: 0}, {Id: 11}, {Id: 102}},
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
			expected: append(field.ErrorList{},
				field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(0).Child("externalStartingPort"), int32(65535),
					invalidExternalListenerStartingPortErrMsg+": "+fmt.Sprintf("ExternalListener '%s' would generate external access port numbers (externalStartingPort + Broker ID) that are out of range (not between 1 and 65535) for brokers %v",
						"test-external1", []int32{11, 102})),
				field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(1).Child("externalStartingPort"), int32(65434),
					invalidExternalListenerStartingPortErrMsg+": "+fmt.Sprintf("ExternalListener '%s' would generate external access port numbers (externalStartingPort + Broker ID) that are out of range (not between 1 and 65535) for brokers %v",
						"test-external2", []int32{102})),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			got := checkExternalListenerStartingPort(&testCase.kafkaClusterSpec)
			require.Equal(t, testCase.expected, got)
		})
	}
}
