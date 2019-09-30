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

package backoff

import (
	"errors"
	"testing"
	"time"

	"github.com/lestrrat-go/backoff"
)

func TestRetry(t *testing.T) {
	count := 0
	config := &ConstantBackoffConfig{
		Delay:          time.Duration(1) * time.Second,
		MaxElapsedTime: time.Duration(10) * time.Second,
		MaxRetries:     3,
	}
	policy := NewConstantBackoffPolicy(config)

	Retry(func() error {
		count = count + 1
		return errors.New("err")
	}, policy)

	if count != 4 {
		t.Error("Expected function to run 4 times, got:", count)
	}

	count = 0
	Retry(func() error {
		count = count + 1
		return nil
	}, policy)

	if count != 1 {
		t.Error("Expected function to run once, got:", count)
	}

	count = 0
	err := Retry(func() error {
		count = count + 1
		return MarkErrorPermanent(errors.New("permanent"))
	}, policy)
	if err == nil {
		t.Error("Expected permanent error, got nil")
	} else if !backoff.IsPermanentError(err) {
		t.Error("Expected true for is permanent error, got false")
	} else if count != 1 {
		t.Error("Expected function to only try once, got:", count)
	}
}
