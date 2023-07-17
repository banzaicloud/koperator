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
	"time"

	"emperror.dev/errors"
	"golang.org/x/exp/maps"
)

// annotationsWhithValidations is a map whose keys are KafkaUserSpec annotation keys, and values are validators
type annotationsWhithValidations map[string]annotationValidator

func (a annotationsWhithValidations) validate(as map[string]string) error {
	for key, value := range as {
		validator, ok := a[key]
		if !ok {
			return a.getNotSupportedAnnotationError()
		}
		err := validator.validate(value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a annotationsWhithValidations) getNotSupportedAnnotationError() error {
	return fmt.Errorf("kafkauser annotations contain a not supported annotation for the signer, supported annotations: %v", maps.Keys(a))
}

// annotationValidator is used to implement a single annotation validator
type annotationValidator interface {
	validate(string) error
}

// newCertManagerSignerAnnotations returns annotationsWhithValidations for cert-manager pki backend signer
func newCertManagerSignerAnnotations() annotationsWhithValidations {
	var c certManagerRequestDurationValidator
	return annotationsWhithValidations{
		"experimental.cert-manager.io/request-duration": &c,
	}
}

// certManagerRequestDurationValidator implements annotationValidator interface
type certManagerRequestDurationValidator struct{}

func (c certManagerRequestDurationValidator) validate(a string) error {
	_, err := time.ParseDuration(a)
	if err != nil {
		return errors.WrapIf(err, "could not parse certificate request duration")
	}
	return nil
}
