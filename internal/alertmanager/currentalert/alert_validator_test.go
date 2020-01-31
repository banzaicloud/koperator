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

func TestAlertValidators_ValidateAlert(t *testing.T) {
	tests := []struct {
		name    string
		v       AlertValidators
		wantErr bool
	}{
		{
			name: "addPVC validate success",
			v: AlertValidators{
				addPVCValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"persistentvolumeclaim": "test-pvc",
						},
						Annotations: model.LabelSet{
							"command": AddPVCCommand,
						},
					},
				},
			},
		},
		{
			name: "addPVC validate failed due to missing label",
			v: AlertValidators{
				addPVCValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"persistentvolumeclaim_missing": "test-pvc",
						},
						Annotations: model.LabelSet{
							"command": AddPVCCommand,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "addPVC validate failed due to unsupported command",
			v: AlertValidators{
				addPVCValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"persistentvolumeclaim": "test-pvc",
						},
						Annotations: model.LabelSet{
							"command": "fake-command",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "downScale validate success",
			v: AlertValidators{
				downScaleValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"kafka_cr": "kafka",
						},
						Annotations: model.LabelSet{
							"command": DownScaleCommand,
						},
					},
				},
			},
		},
		{
			name: "downScale validate failed due to missing label",
			v: AlertValidators{
				downScaleValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"kafka_cr_missing": "kafka",
						},
						Annotations: model.LabelSet{
							"command": DownScaleCommand,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "downScale validate failed due to unsupported command",
			v: AlertValidators{
				downScaleValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"kafka_cr": "kafka",
						},
						Annotations: model.LabelSet{
							"command": "fake-command",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "upScale validate success",
			v: AlertValidators{
				upScaleValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"kafka_cr": "kafka",
						},
						Annotations: model.LabelSet{
							"command": UpScaleCommand,
						},
					},
				},
			},
		},
		{
			name: "upScale validate failed due to missing label",
			v: AlertValidators{
				upScaleValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"kafka_cr_missing": "kafka",
						},
						Annotations: model.LabelSet{
							"command": UpScaleCommand,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "upScale validate failed due to unsupported command",
			v: AlertValidators{
				upScaleValidator{
					Alert: &currentAlertStruct{
						Labels: model.LabelSet{
							"kafka_cr": "kafka",
						},
						Annotations: model.LabelSet{
							"command": "fake-command",
						},
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
			if err := tt.v.ValidateAlert(); (err != nil) != tt.wantErr {
				t.Errorf("AlertValidators.ValidateAlert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
