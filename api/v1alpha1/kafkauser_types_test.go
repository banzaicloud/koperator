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
	"fmt"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

const (
	testNamespace             = "test-namespace"
	testCertManagerSignerName = CertManagerSignerNamePrefix + "/test-issuer"
)

func TestKafkaUserSpecValidateAnnotations(t *testing.T) {
	t.Parallel()
	type args struct {
		annotations map[string]string
	}
	kafkaUser := &KafkaUser{
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

	tests := []struct {
		name   string
		args   args
		wanted error
	}{
		{
			name: "no invalid annotations",
			args: args{
				annotations: map[string]string{
					"experimental.cert-manager.io/request-duration": "2880h",
				},
			},
			wanted: nil,
		},
		{
			name: "valid annotation, invalid value",
			args: args{
				annotations: map[string]string{
					"experimental.cert-manager.io/request-duration": "2880",
				},
			},
			wanted: fmt.Errorf("could not parse certificate request duration: time: missing unit in duration \"2880\""),
		},
		{
			name: "invalid annotation",
			args: args{
				annotations: map[string]string{
					"test-annotation": "test",
				},
			},
			wanted: newCertManagerSignerAnnotationsWithValidators().getNotSupportedAnnotationError(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kafkaUser.Spec.Annotations = tt.args.annotations
			found := kafkaUser.Spec.ValidateAnnotations()

			if tt.wanted != nil && found != nil {
				assert.Equal(t, tt.wanted.Error(), found.Error())
			} else {
				assert.Equal(t, tt.wanted, found)
			}
		})
	}
}
