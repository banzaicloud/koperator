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

package e2e

import (
	"time"

	"github.com/banzaicloud/koperator/tests/e2e/pkg/tests"
	"github.com/gruntwork-io/terratest/modules/k8s"

	. "github.com/onsi/ginkgo/v2"
)

var mockTest1 = tests.TestCase{
	TestDuration: 4 * time.Second,
	TestName:     "MockTest1",
	TestFn:       testMockTest1,
}

func testMockTest1(kubectlOptions k8s.KubectlOptions) {
	It("MockTest1-1", func() {
		time.Sleep(time.Second * 2)
	})
	It("MockTest1-2", func() {
		time.Sleep(time.Second * 2)
	})
}

var mockTest2 = tests.TestCase{
	TestDuration: 5 * time.Second,
	TestName:     "MockTest2",
	TestFn:       testMockTest2,
}

func testMockTest2(kubectlOptions k8s.KubectlOptions) {
	It("MockTest2-1", func() {
		time.Sleep(time.Second * 1)
	})
	It("MockTest2-2", func() {
		time.Sleep(time.Second * 2)
	})
	It("MockTest2-3", func() {
		time.Sleep(time.Second * 2)
	})
}
