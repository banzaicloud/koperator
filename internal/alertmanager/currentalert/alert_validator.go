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

	"github.com/prometheus/common/model"
)

// AlertValidators validate alert
type AlertValidators []AlertValidator

// AlertValidator validates an alert.
type AlertValidator interface {
	// ValidateAlert validates an alert.
	validateAlert() error
}

// ValidateAlert validates
func (v AlertValidators) ValidateAlert() error {
	var violations []string

	for _, validator := range v {
		err := validator.validateAlert()
		if err != nil {
			violations = append(violations, err.Error())
		}
	}

	if len(violations) > 0 {
		return emperror.NewWithDetails("alert validation failed %v", violations)
	}

	return nil
}

func checkLabelExists(labelSet model.LabelSet, label model.LabelName) bool {
	if _, labelOK := labelSet[label]; labelOK {
		return true
	}

	return false
}
