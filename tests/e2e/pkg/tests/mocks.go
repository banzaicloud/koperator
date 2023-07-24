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

	//. "github.com/onsi/gomega"
	"time"
)

// MockTestsMinimal:
// 3 different provider 2 different version 3 different K8s cluster with 2 tests
// Expected: 2 testCase 1 testCase on any available K8sCluster
// 1x2 + 1x3 = 5 test all-together
// Runtime parallel: 1x(5) = 5sec (time of the longest testCase)
func MockTestsMinimal() Classifier {
	k8sClusterPool := K8sClusterPool{}
	k8sClusterPool.AddK8sClusters(
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
	)
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsProvider:
// 3 different provider 3 different K8s cluster with 2 tests
// Expected: 6 testCase 2 testCase on every provider
// 3x2 + 3x3 = 15 test all-together
// Runtime parallel: 3x(3) = 9sec (time of the longest testCase)
func MockTestsProvider() Classifier {
	k8sClusterPool := K8sClusterPool{}
	k8sClusterPool.AddK8sClusters(
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
	)
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsProviderMoreTestsThenProvider:
// 2 different provider 2 different K8s cluster with 3 tests
// Expected: 6 testCase 3 testCase on every provider
// 2x2 + 2x2 + 2x3 = 14 test all-together
// Runtime parallel: 4 + 4 + 5 = 13
func MockTestsProviderMoreTestsThenProvider() Classifier {
	k8sClusterPool := K8sClusterPool{}
	k8sClusterPool.AddK8sClusters(
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
	)
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2, mockTest3)
}

// MockTestsVersionOne:
// no different version 2 different K8s cluster with 2 tests
// Expected: 2 testCase -> 1 testCase on each K8sCluster
// 2x2  2x3 = 10 test all-together
// Runtime parallel: 1x5 = 5
func MockTestsVersionOne() Classifier {
	k8sClusterPool := K8sClusterPool{}
	k8sClusterPool.AddK8sClusters(
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
	)
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsVersion:
// 2 different version 3 different K8s cluster with 2 tests
// Expected: 4 testCase -> 2 testCase on each version
// 2x2  2x3 = 10 test all-together
// Runtime parallel: 4 + 5 = 9
func MockTestsVersion() Classifier {
	k8sClusterPool := K8sClusterPool{}
	k8sClusterPool.AddK8sClusters(
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
	)
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

// MockTestsVersion:
// 2 different version 2 different version 4 K8s cluster with 2 tests
// Expected: 4 testCase -> 2 testCase on each version
// 2x2  2x3 = 10 test all-together
// Runtime parallel: 4 + 5 = 9
func MockTestsComplete() Classifier {
	k8sClusterPool := K8sClusterPool{}
	k8sClusterPool.AddK8sClusters(
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
	)
	return NewClassifier(k8sClusterPool, mockTest1, mockTest2)
}

var mockTest1 = TestCase{
	TestDuration: 4 * time.Second,
	TestName:     "MockTest1",
	TestFn:       testMockTest1,
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
	TestDuration: 5 * time.Second,
	TestName:     "MockTest2",
	TestFn:       testMockTest2,
}

func testMockTest2(kubectlOptions k8s.KubectlOptions) {
	It("MockTest2-1", func() {
		time.Sleep(time.Second * 1)
	})
	It("MockTest2-2", func() {
		time.Sleep(time.Second * 1)
		//Expect(0).Should(Equal(1))
		AddReportEntry("Output:", CurrentSpecReport().CapturedGinkgoWriterOutput)
	})
	It("MockTest2-3", func() {
		time.Sleep(time.Second * 1)
	})
}

var mockTest3 = TestCase{
	TestDuration: 4 * time.Second,
	TestName:     "MockTest3",
	TestFn:       testMockTest3,
}

func testMockTest3(kubectlOptions k8s.KubectlOptions) {
	It("MockTest3-1", func() {
		time.Sleep(time.Second * 2)
	})
	It("MockTest3-2", func() {
		time.Sleep(time.Second * 2)
	})
}
