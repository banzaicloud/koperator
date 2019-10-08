// Copyright © 2019 Banzai Cloud
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

import "testing"

func TestParsePropertiesFormat(t *testing.T) {
	testProp := `
broker.id=1
advertised.listener=broker-1:29092
empty.config=
`
	props := ParsePropertiesFormat(testProp)

	if props["broker.id"] != "1" || props["advertised.listener"] != "broker-1:29092" || props["empty.config"] != "" {
		t.Error("Error properties not loaded correctly")
	}

}

func TestMonitoringAnnotations(t *testing.T) {
	annotations := MonitoringAnnotations(8888)
	if annotations["prometheus.io/port"] != "8888" {
		t.Error("Error port not converted correctly")
	}
}

func TestMergeAnnotations(t *testing.T) {
	annotations := MonitoringAnnotations(8888)
	annotations2 := map[string]string{"thing": "1", "other_thing": "2"}

	combined := MergeAnnotations(annotations, annotations2)

	if len(combined) != 4 {
		t.Error("Annotations didn't combine correctly")
	}
}
