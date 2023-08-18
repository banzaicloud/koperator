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

package tests

import (
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"

	"time"
)

// MockTestsMinimal returns a Classifier that has 3 different K8s clusters using 2 different K8s versions and 3 different providers.
// The Classifier contains 2 tests.
//
//	Expected (minimal strategy):
//	  - 2 testCases
//	  - 1 testCase on any of the available K8sClusters
//	  - runtime parallel: 1x(3) = 3sec (time of the longest testCase)
func MockTestsMinimal() Classifier {
	k8sClusterPool := K8sClusterPool{
		NewMockK8sCluster(
			"testContextPath1",
			"testContextName1",
			"1.24",
			"provider1",
			"clusterID1",
		),
		NewMockK8sCluster(
			"testContextPath2",
			"testContextName2",
			"1.25",
			"provider2",
			"clusterID2",
		),
		NewMockK8sCluster(
			"testContextPath3",
			"testContextName3",
			"1.24",
			"provider3",
			"clusterID3",
		),
	}
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsProvider returns a Classifier that has 3 different K8s clusters using 3 different K8s provider.
// The Classifier contains 2 tests.
//
//	Expected (provider strategy):
//	  - 6 testCases
//	  - 2 testCases on different providers
//	  - runtime parallel: 2 + 3 = 5sec
func MockTestsProvider() Classifier {
	k8sClusterPool := K8sClusterPool{
		NewMockK8sCluster(
			"testContextPath1",
			"testContextName1",
			"1.24",
			"provider1",
			"clusterID1",
		),
		NewMockK8sCluster(
			"testContextPath2",
			"testContextName2",
			"1.24",
			"provider2",
			"clusterID2",
		),
		NewMockK8sCluster(
			"testContextPath3",
			"testContextName3",
			"1.24",
			"provider3",
			"clusterID3",
		),
	}
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsProviderMoreTestsThenProvider returns a Classifier that has 2 different K8s clusters using 2 different K8s provider.
// The Classifier contains 3 tests.
//
//	Expected (provider strategy):
//	  - 6 testCases
//	  - 3 testCases on different providers
//	  - runtime parallel: 2 + 3 + 4 = 9sec
func MockTestsProviderMoreTestsThenProvider() Classifier {
	k8sClusterPool := K8sClusterPool{
		NewMockK8sCluster(
			"testContextPath1",
			"testContextName1",
			"1.24",
			"provider1",
			"clusterID1",
		),
		NewMockK8sCluster(
			"testContextPath2",
			"testContextName2",
			"1.24",
			"provider2",
			"clusterID2",
		),
	}
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2, mockTest3)
}

// MockTestsVersionOne returns a Classifier that has 2 different K8s clusters using same K8s versions and 2 different providers.
// The Classifier contains 2 tests.
//
//	Expected (version strategy):
//	  - 2 testCases
//	  - 1 testCase on any of the available K8sClusters
//	  - runtime parallel: 1 x 3 = 3sec
func MockTestsVersionOne() Classifier {
	k8sClusterPool := K8sClusterPool{
		NewMockK8sCluster(
			"testContextPath1",
			"testContextName1",
			"1.24",
			"provider1",
			"clusterID1",
		),
		NewMockK8sCluster(
			"testContextPath2",
			"testContextName2",
			"1.24",
			"provider2",
			"clusterID2",
		),
	}
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsVersion returns a Classifier that has 3 different K8s clusters using 2 different K8s versions and 2 different providers.
// The Classifier contains 2 tests.
//
//	Expected (version strategy):
//	  - 4 testCases
//	  - 2 testCase on any of the available K8sClusters
//	  - runtime parallel: 1 x 5 = 5sec
func MockTestsVersion() Classifier {
	k8sClusterPool := K8sClusterPool{
		NewMockK8sCluster(
			"testContextPath1",
			"testContextName1",
			"1.24",
			"provider1",
			"clusterID1",
		),
		NewMockK8sCluster(
			"testContextPath2",
			"testContextName2",
			"1.24",
			"provider2",
			"clusterID2",
		),
		NewMockK8sCluster(
			"testContextPath4",
			"testContextName4",
			"1.26",
			"provider3",
			"clusterID4",
		),
	}
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsComplete returns a Classifier that has 4 different K8s clusters using 2 different K8s versions and 3 different providers.
// The Classifier contains 2 tests.
//
//	Expected (complete strategy):
//	  - 6 testCases
//	  - 2 testCase on every different K8sClusters provider and version
//	  - runtime parallel: 4 + 5 = 9sec
func MockTestsComplete() Classifier {
	k8sClusterPool := K8sClusterPool{
		NewMockK8sCluster(
			"testContextPath1",
			"testContextName1",
			"1.24",
			"provider1",
			"clusterID1",
		),
		NewMockK8sCluster(
			"testContextPath2",
			"testContextName2",
			"1.24",
			"provider2",
			"clusterID2",
		),
		NewMockK8sCluster(
			"testContextPath3",
			"testContextName3",
			"1.25",
			"provider3",
			"clusterID3",
		),
		NewMockK8sCluster(
			"testContextPath4",
			"testContextName4",
			"1.25",
			"provider3",
			"clusterID4",
		),
	}
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

var mockTest1 = TestCase{
	SpecsCount: 2,
	Duration:   2 * time.Second,
	Name:       "MockTest1",
	TestFn:     testMockTest1,
}

func testMockTest1(kubectlOptions k8s.KubectlOptions) {
	It("MockTest1-1", func() {
		time.Sleep(time.Second * 1)
	})
	It("MockTest1-2", func() {
		time.Sleep(time.Second * 1)
	})
}

var mockTest2 = TestCase{
	SpecsCount: 3,
	Duration:   3 * time.Second,
	Name:       "MockTest2",
	TestFn:     testMockTest2,
}

func testMockTest2(kubectlOptions k8s.KubectlOptions) {
	It("MockTest2-1", func() {
		time.Sleep(time.Second * 1)
	})
	It("MockTest2-2", func() {
		time.Sleep(time.Second * 1)
	})
	It("MockTest2-3", func() {
		time.Sleep(time.Second * 1)
	})
}

var mockTest3 = TestCase{
	SpecsCount: 2,
	Duration:   4 * time.Second,
	Name:       "MockTest3",
	TestFn:     testMockTest3,
}

func testMockTest3(kubectlOptions k8s.KubectlOptions) {
	It("MockTest3-1", func() {
		time.Sleep(time.Second * 2)
	})
	It("MockTest3-2", func() {
		time.Sleep(time.Second * 2)
	})
}
