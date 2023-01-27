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

package webhooks

import (
	"fmt"
	"testing"

	"emperror.dev/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestIsCantConnectAPIServer(t *testing.T) {
	testcases := []struct {
		testname string
		err      error
		want     bool
	}{
		{
			testname: "cantConnectAPIServer",
			err:      apierrors.NewInternalError(errors.Wrap(errors.New("..."), cantConnectAPIServerMsg)),
			want:     true,
		},
		{
			testname: "wrong-error-message",
			err:      apierrors.NewInternalError(errors.Wrap(errors.New("..."), "wrong-error-message")),
			want:     false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testname, func(t *testing.T) {
			if got := IsCantConnectAPIServer(tc.err); got != tc.want {
				t.Errorf("Check connection to API Server error message. Expected: %t ; Got: %t", tc.want, got)
			}
		})
	}
}

func TestIsOutOfRangeReplicationFactor(t *testing.T) {
	kafkaTopic := banzaicloudv1alpha1.KafkaTopic{ObjectMeta: metav1.ObjectMeta{Name: "test-KafkaTopic"}}
	var fieldErrs field.ErrorList
	fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("replicationFactor"), "-2", outOfRangeReplicationFactorErrMsg))
	err := apierrors.NewInvalid(
		kafkaTopic.GetObjectKind().GroupVersionKind().GroupKind(),
		kafkaTopic.Name, fieldErrs)

	if ok := IsOutOfRangeReplicationFactor(err); !ok {
		t.Errorf("Check Out of Range ReplicationFactor error message. Expected: %t ; Got: %t", true, ok)
	}
}

func TestIsOutOfRangePartitions(t *testing.T) {
	kafkaTopic := banzaicloudv1alpha1.KafkaTopic{ObjectMeta: metav1.ObjectMeta{Name: "test-KafkaTopic"}}
	var fieldErrs field.ErrorList
	fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("partitions"), "-2", outOfRangePartitionsErrMsg))
	err := apierrors.NewInvalid(
		kafkaTopic.GetObjectKind().GroupVersionKind().GroupKind(),
		kafkaTopic.Name, fieldErrs)

	if ok := IsOutOfRangePartitions(err); !ok {
		t.Errorf("Check Out of Range Partitions error message. Expected: %t ; Got: %t", true, ok)
	}
}

func TestIsInvalidRemovingStorage(t *testing.T) {
	testcases := []struct {
		testname  string
		fieldErrs field.ErrorList
		want      bool
	}{
		{
			testname:  "field.Invalid_removingStorage",
			fieldErrs: append(field.ErrorList{}, field.Invalid(field.NewPath("spec").Child("brokers").Index(0).Child("brokerConfigGroup"), "test-broker-config-group", unsupportedRemovingStorageMsg+", provided brokerConfigGroup not found")),
			want:      true,
		},
		{
			testname:  "field.NotFound_removingStorage",
			fieldErrs: append(field.ErrorList{}, field.NotFound(field.NewPath("spec").Child("brokers").Index(0).Child("storageConfig").Index(0), "/test/storageConfig/mount/path"+", "+unsupportedRemovingStorageMsg)),
			want:      true,
		},
		{
			testname:  "field.Invalid_wrong-error-message",
			fieldErrs: append(field.ErrorList{}, field.Invalid(field.NewPath("spec").Child("brokers").Index(0).Child("brokerConfigGroup"), "test-broker-config-group", "wrong-error-message"+", provided brokerConfigGroup not found")),
			want:      false,
		},
		{
			testname:  "field.NotFound_wrong-error-message",
			fieldErrs: append(field.ErrorList{}, field.NotFound(field.NewPath("spec").Child("brokers").Index(0).Child("storageConfig").Index(0), "/test/storageConfig/mount/path"+", "+"wrong-error-message")),
			want:      false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testname, func(t *testing.T) {
			kafkaCluster := banzaicloudv1beta1.KafkaCluster{ObjectMeta: metav1.ObjectMeta{Name: "test-KafkaCluster"}}

			err := apierrors.NewInvalid(
				kafkaCluster.GetObjectKind().GroupVersionKind().GroupKind(),
				kafkaCluster.Name, tc.fieldErrs)

			if got := IsInvalidRemovingStorage(err); got != tc.want {
				t.Errorf("Check Storage Removal Error message. Expected: %t ; Got: %t", tc.want, got)
			}
		})
	}
}

func TestIsErrorDuringValidation(t *testing.T) {
	testcases := []struct {
		testname string
		err      error
		want     bool
	}{
		{
			testname: "errorDuringValidation",
			err:      apierrors.NewInternalError(errors.WithMessage(errors.New("..."), errorDuringValidationMsg)),
			want:     true,
		},
		{
			testname: "wrong-error-message",
			err:      apierrors.NewInternalError(errors.WithMessage(errors.New("..."), "wrong-error-message")),
			want:     false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testname, func(t *testing.T) {
			if got := IsErrorDuringValidation(tc.err); got != tc.want {
				t.Errorf("Check overall Error During Validation error message. Expected: %t ; Got: %t", tc.want, got)
			}
		})
	}
}
