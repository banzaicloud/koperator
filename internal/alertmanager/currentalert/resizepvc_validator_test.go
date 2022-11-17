// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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

func TestResizePvcValidator_validateAlert(t *testing.T) {
	type fields struct {
		Alert *currentAlertStruct
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "resizePvc validate success",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command":     ResizePvcCommand,
						"incrementBy": "10Gi",
					},
				},
			},
		},
		{
			name: "resizePvc validate failed due to invalid incrementBy annotation",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command":     ResizePvcCommand,
						"incrementBy": "10Gb",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "resizePvc validate failed due to missing incrementBy annotation",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command": ResizePvcCommand,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "resizePvc validate failed due to missing label",
			fields: fields{
				Alert: &currentAlertStruct{
					Labels: model.LabelSet{
						"persistentvolumeclaim_missing": "test-pvc",
					},
					Annotations: model.LabelSet{
						"command": ResizePvcCommand,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "resizePvc validate failed due to unsupported command",
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
			a := resizePvcValidator{
				Alert: tt.fields.Alert,
			}
			if err := a.validateAlert(); (err != nil) != tt.wantErr {
				t.Errorf("resizePvcValidator.validateAlert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
