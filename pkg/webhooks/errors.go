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

package webhooks

import (
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TODO (tinyzimmer): This may be better suited for the errorfactory package

func IsAdmissionCantConnect(err error) bool {
	if apierrors.IsInternalError(err) && strings.Contains(err.Error(), cantConnectErrorMsg) {
		return true
	}
	return false
}

func IsInvalidReplicationFactor(err error) bool {
	if apierrors.IsInvalid(err) && strings.Contains(err.Error(), invalidReplicationFactorErrMsg) {
		return true
	}
	return false
}
