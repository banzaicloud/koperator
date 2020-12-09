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

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/kafka-operator/pkg/util"
)

func TestShouldRefreshOnlyPerBrokerConfigs(t *testing.T) {
	testCases := []struct {
		Description    string
		CurrentConfigs map[string]string
		DesiredConfigs map[string]string
		Result         bool
	}{
		{
			Description: "configs did not change",
			CurrentConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "unmodified_value_2",
			},
			DesiredConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "unmodified_value_2",
			},
			Result: true,
		},
		{
			Description: "only non per-broker config changed",
			CurrentConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "modified_value_2",
			},
			DesiredConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"unmodified_config_2": "modified_value_3",
			},
			Result: false,
		},
		{
			Description: "only per-broker config changed",
			CurrentConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"ssl.client.auth":     "modified_value_2",
			},
			DesiredConfigs: map[string]string{
				"unmodified_config_1": "unmodified_value_1",
				"ssl.client.auth":     "modified_value_3",
			},
			Result: true,
		},
		{
			Description: "per-broker and non per-broker configs have changed",
			CurrentConfigs: map[string]string{
				"modified_config_1": "modified_value_1",
				"ssl.client.auth":   "modified_value_3",
			},
			DesiredConfigs: map[string]string{
				"modified_config_1": "modified_value_2",
				"ssl.client.auth":   "modified_value_4",
			},
			Result: false,
		},
		{
			Description: "security protocol map can be changed as a per-broker config",
			CurrentConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol2",
			},
			DesiredConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener3:protocol3",
			},
			Result: true,
		},
		{
			Description: "security protocol map can't be changed as a per-broker config",
			CurrentConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol2",
			},
			DesiredConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol3",
			},
			Result: false,
		},
		{
			Description:    "security protocol map added as config",
			CurrentConfigs: map[string]string{},
			DesiredConfigs: map[string]string{
				"listener.security.protocol.map": "listener1:protocol1,listener2:protocol2",
			},
			Result: true,
		},
	}
	logger := util.CreateLogger(false, false)
	for i, testCase := range testCases {
		if ShouldRefreshOnlyPerBrokerConfigs(testCase.CurrentConfigs, testCase.DesiredConfigs, logger) != testCase.Result {
			t.Errorf("test case %d failed: %s", i, testCase.Description)
		}
	}
}

func TestCollectTouchedConfigs(t *testing.T) {
	testCases := []struct {
		DesiredConfigs string
		CurrentConfigs string
		Result         map[string]struct{}
	}{
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value2
key3=value3`,
			Result: map[string]struct{}{},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2`,
			CurrentConfigs: `key1=value1
key2=value2
key3=value3`,
			Result: map[string]struct{}{
				"key3": {},
			},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value2`,
			Result: map[string]struct{}{
				"key3": {},
			},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value4
key3=value3`,
			Result: map[string]struct{}{
				"key2": {},
			},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3
key4=value4`,
			CurrentConfigs: `key1=value6
key2=value2
key3=value3
key5=value5`,
			Result: map[string]struct{}{
				"key1": {},
				"key4": {},
				"key5": {},
			},
		},
	}

	logger := util.CreateLogger(false, false)
	for _, testCase := range testCases {
		desired := &corev1.ConfigMap{
			Data: map[string]string{
				ConfigPropertyName: testCase.DesiredConfigs,
			},
		}
		current := &corev1.ConfigMap{
			Data: map[string]string{
				ConfigPropertyName: testCase.CurrentConfigs,
			},
		}
		touchedConfigs := collectTouchedConfigs(
			util.ParsePropertiesFormat(current.Data[ConfigPropertyName]),
			util.ParsePropertiesFormat(desired.Data[ConfigPropertyName]), logger)
		if !reflect.DeepEqual(touchedConfigs, testCase.Result) {
			t.Errorf("comparison failed - expected: %s, actual: %s", testCase.Result, touchedConfigs)
		}
	}
}
