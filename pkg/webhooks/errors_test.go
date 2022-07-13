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
	"fmt"
	"testing"

	"emperror.dev/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
)

func TestIsAdmissionConnectionError(t *testing.T) {
	err := apierrors.NewInternalError(errors.Wrap(errors.New("..."), cantConnectErrorMsg))

	if !IsAdmissionCantConnect(err) {
		t.Error("Expected is connection error to be true, got false")
	}

	err = apierrors.NewServiceUnavailable("some other reason")
	if IsAdmissionCantConnect(err) {
		t.Error("Expected is connection error to be false, got true")
	}

	err = apierrors.NewBadRequest(cantConnectErrorMsg)
	if IsAdmissionCantConnect(err) {
		t.Error("Expected is connection error to be false, got true")
	}
}

func TestIsInvalidReplicationFactor(t *testing.T) {
	kafkaTopic := banzaicloudv1alpha1.KafkaTopic{}
	var fieldErrs field.ErrorList
	logMsg := fmt.Sprintf("%s (available brokers: 2)", invalidReplicationFactorErrMsg)
	fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("replicationFactor"), "4", logMsg))
	err := apierrors.NewInvalid(
		kafkaTopic.GetObjectKind().GroupVersionKind().GroupKind(),
		kafkaTopic.Name, fieldErrs)

	if !IsInvalidReplicationFactor(err) {
		t.Error("Expected is invalid replication error to be true, got false")
	}

	err = apierrors.NewServiceUnavailable("some other reason")
	if IsInvalidReplicationFactor(err) {
		t.Error("Expected is invalid replication error to be false, got true")
	}

	err = apierrors.NewServiceUnavailable(invalidReplicationFactorErrMsg)
	if IsInvalidReplicationFactor(err) {
		t.Error("Expected is invalid replication error to be false, got true")
	}
}
