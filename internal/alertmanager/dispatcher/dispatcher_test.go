// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

	"github.com/banzaicloud/koperator/internal/alertmanager/currentalert"
)

func Test_alertFilter(t *testing.T) {
	testAlerts := []model.Alert{
		{
			Labels: model.LabelSet{
				"test": "test_1",
			},
			Annotations: model.LabelSet{
				"command": currentalert.AddPvcCommand,
			},
		},
		{
			Labels: model.LabelSet{
				"test": "test_1",
			},
			Annotations: model.LabelSet{
				"command": currentalert.UpScaleCommand,
			},
		},
		{
			Labels: model.LabelSet{
				"test": "test_1",
			},
			Annotations: model.LabelSet{
				"command": currentalert.DownScaleCommand,
			},
		},
		{
			Labels: model.LabelSet{
				"test": "test_2",
			},
		},
		{
			Labels: model.LabelSet{
				"test": "test_3",
			},
			Annotations: model.LabelSet{
				"command": "fakeComand",
			},
		},
	}

	filteredAlerts := []model.Alert{
		{
			Labels: model.LabelSet{
				"test": "test_1",
			},
			Annotations: model.LabelSet{
				"command": currentalert.AddPvcCommand,
			},
		},
		{
			Labels: model.LabelSet{
				"test": "test_1",
			},
			Annotations: model.LabelSet{
				"command": currentalert.UpScaleCommand,
			},
		},
		{
			Labels: model.LabelSet{
				"test": "test_1",
			},
			Annotations: model.LabelSet{
				"command": currentalert.DownScaleCommand,
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := alertFilter(tt.test); !reflect.DeepEqual(got, tt.filtered) {
				t.Errorf("alertFilter() = %v, want %v", got, tt.filtered)
			}
		})
	}
}
