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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lestrrat-go/backoff/v2"
)

type MyError struct {
	error
	number int
}

func TestExponentialBackOff(t *testing.T) {
	p := backoff.Constant(
		backoff.WithInterval(time.Millisecond*10),
		backoff.WithMaxRetries(2),
	)
	flakyFunc := func(a int) (int, error) {
		switch {
		case a%3 == 0:
			return 3, errors.New("error 3")
		case a%4 == 0:
			return 4, nil
		case a%5 == 0:
			return 5, MyError{number: 5}
		}
		return 0, errors.New(`invalid`)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startNum := 3
	retryFunc := func() int {
		b := p.Start(ctx)
		var err error
		var x int
		for backoff.Continue(b) {
			x, err = flakyFunc(startNum)
			switch startNum {
			case 3:
				if x != 3 || err.Error() != "error 3" {
					t.Errorf("backOff error 3")
				}
			case 4:
				if x != 4 || err != nil {
					t.Errorf("backOff error 4")
				}
			case 5:
				if x != 5 || !errors.Is(err, MyError{number: 5}) {
					t.Errorf("backOff error 5")
				}
			default:
				t.Errorf("more retry then maxRetry")
			}
			startNum++
		}
		return startNum
	}

	if ret := retryFunc(); ret < 6 {
		t.Errorf("not enough iteration (%d) expected: 3", ret-3)
	}
}
