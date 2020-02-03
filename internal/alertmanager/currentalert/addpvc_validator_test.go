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

package currentalert

import (
	"testing"

	"github.com/prometheus/common/model"
)

func TestAddPvcValidator_validateAlert(t *testing.T) {
	type fields struct {
		Alert *currentAlertStruct
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "addPvc validate success",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command": AddPvcCommand,
					},
				},
			},
		},
		{
			name: "addPvc validate failed due to missing label",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim_missing": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command": AddPvcCommand,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "addPvc validate failed due to unsupported command",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command": "fake-command",
					},
				},
			},
			wantErr: true,
		},
	}

	t.Parallel()

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			a := addPvcValidator{
				Alert: tt.fields.Alert,
			}
			if err := a.validateAlert(); (err != nil) != tt.wantErr {
				t.Errorf("addPvcValidator.validateAlert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
