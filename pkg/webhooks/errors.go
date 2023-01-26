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

package webhooks

import (
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	cantConnectErrorMsg               = "failed to connect to kafka cluster"
	cantConnectAPIServerMsg           = "failed to connect to Kubernetes API server"
	invalidReplicationFactorErrMsg    = "replication factor is larger than the number of nodes in the kafka cluster"
	outOfRangeReplicationFactorErrMsg = "replication factor must be larger than 0 (or set it to be -1 to use the broker's default)"
	outOfRangePartitionsErrMsg        = "number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)"
	unsupportedRemovingStorageMsg     = "removing storage from a broker is not supported"

	// errorDuringValidationMsg is added to infrastructure errors (e.g. failed to connect), but not to field validation errors
	errorDuringValidationMsg = "error during validation"
)

func IsAdmissionCantConnect(err error) bool {
	if apierrors.IsInternalError(err) && strings.Contains(err.Error(), cantConnectErrorMsg) {
		return true
	}
	return false
}

func IsCantConnectAPIServer(err error) bool {
	if apierrors.IsInternalError(err) && strings.Contains(err.Error(), cantConnectAPIServerMsg) {
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

func IsOutOfRangeReplicationFactor(err error) bool {
	if apierrors.IsInvalid(err) && strings.Contains(err.Error(), outOfRangeReplicationFactorErrMsg) {
		return true
	}
	return false
}

func IsOutOfRangePartitions(err error) bool {
	if apierrors.IsInvalid(err) && strings.Contains(err.Error(), outOfRangePartitionsErrMsg) {
		return true
	}
	return false
}

func IsInvalidRemovingStorage(err error) bool {
	if apierrors.IsInvalid(err) && strings.Contains(err.Error(), unsupportedRemovingStorageMsg) {
		return true
	}
	return false
}

func IsErrorDuringValidation(err error) bool {
	if apierrors.IsInternalError(err) && strings.Contains(err.Error(), errorDuringValidationMsg) {
		return true
	}
	return false
}
