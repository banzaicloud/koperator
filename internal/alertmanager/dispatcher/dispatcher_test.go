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

package dispatcher

import (
	"reflect"
	"testing"

	"github.com/prometheus/common/model"
)

func Test_alertFilter(t *testing.T) {

	testAlerts := []model.Alert{
		{
			Labels: model.LabelSet{
				"namespace":  "kafka",
				"alertGroup": "kafka",
			},
			Annotations: model.LabelSet{
				"command": "testcommand",
			},
		},
		{
			Labels: model.LabelSet{
				"namespace":  "kafka",
				"alertGroup": "kafka",
			},
		},
		{
			Labels: model.LabelSet{
				"namespace":  "kafka",
				"alertGroup": "fake",
			},
			Annotations: model.LabelSet{
				"command": "testcommand",
			},
		},
		{
			Labels: model.LabelSet{
				"namespace": "kafka",
				"kafka_cr":  "kafka",
			},
			Annotations: model.LabelSet{
				"command": "testcommand",
			},
		},
	}

	filteredAlerts := []model.Alert{
		{
			Labels: model.LabelSet{
				"namespace":  "kafka",
				"alertGroup": "kafka",
			},
			Annotations: model.LabelSet{
				"command": "testcommand",
			},
		},
		{
			Labels: model.LabelSet{
				"namespace": "kafka",
				"kafka_cr":  "kafka",
			},
			Annotations: model.LabelSet{
				"command": "testcommand",
			},
		},
	}

	tests := []struct {
		name     string
		test     []model.Alert
		filtered []model.Alert
	}{
		{
			"alertFilter",
			testAlerts,
			filteredAlerts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alertFilter(tt.test); !reflect.DeepEqual(got, tt.filtered) {
				t.Errorf("alertFilter() = %v, want %v", got, tt.filtered)
			}
		})
	}
}
