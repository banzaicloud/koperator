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

package util

import (
	"reflect"
	"testing"
)

func TestMergeLabels(t *testing.T) {
	m1 := map[string]string{"key1": "value1"}
	m2 := map[string]string{"key2": "value2"}
	expected := map[string]string{"key1": "value1", "key2": "value2"}
	merged := MergeLabels(m1, m2)
	if !reflect.DeepEqual(merged, expected) {
		t.Error("Expected:", expected, "Got:", merged)
	}

	m1 = nil
	expected = m2
	merged = MergeLabels(m1, m2)
	if !reflect.DeepEqual(merged, expected) {
		t.Error("Expected:", expected, "Got:", merged)
	}
}

func TestStringSliceContains(t *testing.T) {
	slice := []string{"1", "2", "3"}
	if !StringSliceContains(slice, "1") {
		t.Error("Expected slice contains 1, got false")
	}
	if StringSliceContains(slice, "4") {
		t.Error("Expected slice not contains 4, got true")
	}
}
