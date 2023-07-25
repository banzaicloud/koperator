// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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

package util

func CloneMap(original map[string]string) map[string]string {
	m := make(map[string]string, len(original))
	for k, v := range original {
		m[k] = v
	}
	return m
}

// MergeLabels merges two given labels
func MergeLabels(l ...map[string]string) map[string]string {
	res := make(map[string]string)

	for _, v := range l {
		for lKey, lValue := range v {
			res[lKey] = lValue
		}
	}
	return res
}

// LabelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func LabelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}

// StringSliceContains returns true if list contains s
func StringSliceContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
