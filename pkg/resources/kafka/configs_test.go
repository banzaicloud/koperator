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
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/kafka-operator/pkg/util"
)

func TestGetBrokerConfigsFromConfigMap(t *testing.T) {
	configMap := &corev1.ConfigMap{
		Data: map[string]string{
			"broker-config": `key1=value1
key2=value2
key3=value3`,
		},
	}
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	actual := getBrokerConfigsFromConfigMap(configMap)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("broker config extraction from configmap failed - expected: %s, actual: %s", expected, actual)
	}
}

func TestCollectTouchedConfigs(t *testing.T) {
	testCases := []struct {
		DesiredConfigs string
		CurrentConfigs string
		Result         []string
	}{
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value2
key3=value3`,
			Result: []string{},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2`,
			CurrentConfigs: `key1=value1
key2=value2
key3=value3`,
			Result: []string{"key3"},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value2`,
			Result: []string{"key3"},
		},
		{
			DesiredConfigs: `key1=value1
key2=value2
key3=value3`,
			CurrentConfigs: `key1=value1
key2=value4
key3=value3`,
			Result: []string{"key2"},
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
			Result: []string{"key1", "key4", "key5"},
		},
	}

	for _, testCase := range testCases {
		desired := &corev1.ConfigMap{
			Data: map[string]string{
				"broker-config": testCase.DesiredConfigs,
			},
		}
		current := &corev1.ConfigMap{
			Data: map[string]string{
				"broker-config": testCase.CurrentConfigs,
			},
		}
		touchedConfigs := collectTouchedConfigs(getBrokerConfigsFromConfigMap(current), getBrokerConfigsFromConfigMap(desired), util.CreateLogger(false, false))
		sort.Strings(touchedConfigs)
		if !reflect.DeepEqual(touchedConfigs, testCase.Result) {
			t.Errorf("comparison failed - expected: %s, actual: %s", testCase.Result, touchedConfigs)
		}
	}
}
