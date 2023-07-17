// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package v1alpha1

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

const (
	testNamespace             = "test-namespace"
	testCertManagerSignerName = CertManagerSignerNamePrefix + "/test-issuer"
)

var (
	testKafkaUser = &KafkaUser{
		TypeMeta: metav1.TypeMeta{
			Kind: "KafkaUser",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: testNamespace,
		},
		Spec: KafkaUserSpec{
			SecretName: "test-secret",
			PKIBackendSpec: &PKIBackendSpec{
				PKIBackend: string(v1beta1.PKIBackendK8sCSR),
				SignerName: testCertManagerSignerName,
			},
		},
	}
)

func TestKafkaUserSpecValidateAnnotationsCorrectAnnotationsList(t *testing.T) {
	testAnnotations := map[string]string{
		"experimental.cert-manager.io/request-duration": "2880h",
	}
	kafkaUser := testKafkaUser
	kafkaUser.Spec.Annotations = testAnnotations

	err := kafkaUser.Spec.ValidateAnnotations()
	if err != nil {
		t.Error(err)
	}
}

func TestKafkaUserSpecValidateAnnotationsCorrectAnnotationIncorrectValue(t *testing.T) {
	testAnnotations := map[string]string{
		"experimental.cert-manager.io/request-duration": "2880",
	}
	expectedErrorMessage := "could not parse certificate request duration: time: missing unit in duration \"2880\""
	kafkaUser := testKafkaUser
	kafkaUser.Spec.Annotations = testAnnotations

	err := kafkaUser.Spec.ValidateAnnotations()
	if !reflect.DeepEqual(err.Error(), expectedErrorMessage) {
		t.Error("Expected:", expectedErrorMessage, "Got:", err.Error())
	}
}

func TestKafkaUserSpecValidateAnnotationsIncorrectAnnotationsList(t *testing.T) {
	testAnnotations := map[string]string{
		"test-annotation": "test",
	}
	expected := newCertManagerSignerAnnotations().getNotSupportedAnnotationError()
	kafkaUser := testKafkaUser
	kafkaUser.Spec.Annotations = testAnnotations

	err := kafkaUser.Spec.ValidateAnnotations()
	if !reflect.DeepEqual(err.Error(), expected.Error()) {
		t.Error("Expected:", expected.Error(), "Got:", err.Error())
	}
}
