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
	emperror "emperror.dev/errors"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

type upScaleValidator struct {
	Alert *currentAlertStruct
}

func newUpScaleValidator(curerentAlert *currentAlertStruct) upScaleValidator {
	return upScaleValidator{
		Alert: curerentAlert,
	}
}

func (a upScaleValidator) validateAlert() error {
	if !checkLabelExists(a.Alert.Labels, v1beta1.KafkaCRLabelKey) {
		return emperror.New("kafka_cr label doesn't exist")
	}
	if a.Alert.Annotations["command"] != UpScaleCommand {
		return emperror.NewWithDetails("unsupported command", "comand", a.Alert.Annotations["command"])
	}

	return nil
}
