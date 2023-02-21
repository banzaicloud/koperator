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

package errorfactory

import (
	"errors"
	"reflect"
	"testing"

	emperrors "emperror.dev/errors"
)

var errorTypes = []error{
	ResourceNotReady{},
	APIFailure{},
	StatusUpdateError{},
	BrokersUnreachable{},
	BrokersNotReady{},
	BrokersRequestError{},
	CreateTopicError{},
	TopicNotFound{},
	GracefulUpscaleFailed{},
	TooManyResources{},
	InternalError{},
	FatalReconcileError{},
	CruiseControlNotReady{},
	CruiseControlTaskRunning{},
}

func TestNew(t *testing.T) {
	for _, errType := range errorTypes {
		errType := errType
		err := New(errType, errors.New("test-error"), "test-message")
		expected := "test-message: test-error"
		got := err.Error()
		if got != expected {
			t.Error("Expected:", expected, "got:", got)
		}
		if !emperrors.As(err, &errType) {
			t.Error("Expected:", reflect.TypeOf(errType), "got:", reflect.TypeOf(err))
		}
	}
}

func TestUnwrapMethod(t *testing.T) {
	// testError is a custom error type used to test the features of unwrapping errors using errors.Is() and errors.As()
	type testError struct{ error }
	var tstErr = testError{errors.New("inner-error")}

	for _, errType := range errorTypes {
		errType := errType
		err := New(errType, tstErr, "test-message")

		// This tests the use of errors.Is() using the Unwrap() method
		if ok := errors.Is(err, tstErr); !ok {
			t.Errorf("Type %T does not Unwrap() correctly using errors.Is(). Expected: %t ; Got: %t", errType, true, ok)
		}

		var c testError
		// This tests the use of errors.As() using the Unwrap() method
		if ok := errors.As(err, &c); !ok {
			t.Errorf("Type %T does not Unwrap() correctly using errors.As(). Expected: %t ; Got: %t", errType, true, ok)
			// This tests whether errors.As() succeeded in extracting the correct wrapped error into the given variable, not just the boolean return value
			if c != tstErr {
				t.Errorf("Type %T does not extract correctly the wrapped error using errors.As(). Expected: %v ; Got: %v", errType, tstErr, c)
			}
		}
	}
}
