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
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/banzaicloud/koperator/tests/e2e/pkg/common"
	"github.com/banzaicloud/koperator/tests/e2e/pkg/common/config"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/dsl/decorators"
	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//	[]testCase
//		|
// Classifier ==> TestPool <-- []Test <-- K8sCluster <-- ClusterInfo
// 		^						 ^
//		|						 |
// 	K8sClusterPool 			   testCase

type TestPool []Test

// Equal returns true when the TestPools are equal
func (tests TestPool) Equal(other TestPool) bool {
	if len(tests) != len(other) {
		return false
	}

	tests.sort()
	other.sort()

	for i := range tests {
		if !tests[i].equal(other[i]) {
			return false
		}
	}
	return true
}

// PoolInfo returns a formatted string as information about the current testPool
func (tests TestPool) PoolInfo() string {
	testsByContextName := tests.getTestsByContextName()
	testsByProviders := tests.getTestsByProviders()
	testsByVersions := tests.getTestsByVersions()

	return fmt.Sprintf(`
NumberOfTests: %d
NumberOfSpecs: %d
NumberOfK8sClusters: %d
K8sVersions: %v
K8sProviders: %v
TestK8sMapping: %v
TestStrategy: %s
ExpectedDurationSerial: %s
ExpectedDurationParallel: %s
`, len(tests), tests.GetTestSuiteSpecsCount(), len(maps.Keys(testsByContextName)), maps.Keys(testsByVersions), maps.Keys(testsByProviders),
		tests, viper.GetString(config.Tests.TestStrategy), tests.GetTestSuiteDurationSerial().String(),
		tests.GetTestSuiteDurationParallel().String())
}

// BuildParallelByK8sCluster builds Describe container blocks from tests and K8sClusters.
// In this way we can have guarantee to the tests serial execution for a K8sCluster.
func (tests TestPool) BuildParallelByK8sCluster() {
	testsByClusterID := tests.getSortedTestsByClusterID()
	testsByClusterIDKeys := sortedKeys(testsByClusterID)
	describeID := 0
	// Because of the "Ordered" decorator testCases inside that container will run each after another
	// thus this K8s cluster (container) is dedicated for certain test for serial execution.
	for _, clusterID := range testsByClusterIDKeys {
		describeID += 1
		tests := testsByClusterID[clusterID]
		// Describe name has to be unique and consistent between processes for proper parallelization.
		Describe(fmt.Sprintf("%d:", describeID), decorators.Label(fmt.Sprintf("K8sID:%s", clusterID)), Ordered, func() {
			for _, test := range tests {
				test.Run()
			}
		})
	}
}

func (tests TestPool) sort() {
	sort.SliceStable(tests, func(i, j int) bool {
		return tests[i].less(tests[j])
	})
}

func (tests TestPool) getSortedTestsByClusterID() map[string][]Test {
	testsByClusterID := make(map[string][]Test)
	// Need to be sorted to achieve test specs tree consistency between processes
	// otherwise it can happen that specs order will be different for each process
	tests.sort()

	for _, test := range tests {
		testsByClusterID[test.k8sCluster.clusterInfo.clusterID] = append(testsByClusterID[test.k8sCluster.clusterInfo.clusterID], test)
	}

	return testsByClusterID
}

func (tests TestPool) getTestsByContextName() map[string][]Test {
	testsByContextName := make(map[string][]Test)
	for _, test := range tests {
		testsByContextName[test.k8sCluster.kubectlOptions.ContextName] = append(testsByContextName[test.k8sCluster.kubectlOptions.ContextName], test)
	}

	return testsByContextName
}

func (tests TestPool) getTestsByVersions() map[string][]Test {
	testsByVersions := make(map[string][]Test)
	for _, test := range tests {
		testsByVersions[test.k8sCluster.clusterInfo.version] = append(testsByVersions[test.k8sCluster.clusterInfo.version], test)
	}

	return testsByVersions
}

func (tests TestPool) getTestsByProviders() map[string][]Test {
	testsByProviders := make(map[string][]Test)
	for _, test := range tests {
		testsByProviders[test.k8sCluster.clusterInfo.provider] = append(testsByProviders[test.k8sCluster.clusterInfo.provider], test)
	}

	return testsByProviders
}

// GetTestSuiteDurationParallel calculates the expected duration of the tests when
// suite is executed in parallel.
func (tests TestPool) GetTestSuiteDurationParallel() time.Duration {
	testsByClusterID := tests.getSortedTestsByClusterID()

	var max time.Duration
	for _, tests := range testsByClusterID {
		var localDuration time.Duration
		for _, test := range tests {
			localDuration += test.testCase.Duration
		}

		if localDuration > max {
			max = localDuration
		}
	}
	return max
}

// GetTestSuiteDurationSerial calculates the expected duration of the tests when
// test suite is executed in serial.
func (tests TestPool) GetTestSuiteDurationSerial() time.Duration {
	var allDuration time.Duration
	for _, test := range tests {
		allDuration += test.testCase.Duration
	}
	return allDuration
}

// GetTestSuiteSpecsCount returns the number of specs in the pool based on the testCase explicit information.
func (tests TestPool) GetTestSuiteSpecsCount() int {
	var specsCount int
	for _, test := range tests {
		specsCount += test.testCase.SpecsCount
	}
	return specsCount
}

// GetParallelTotal returns the maximum number of the possible parallelization
func (tests TestPool) GetParallelTotal() int {
	return len(maps.Keys(tests.getSortedTestsByClusterID()))
}

// NewClassifier creates a test classifier from K8sClusterPool and TestCases.
func NewClassifier(k8sClusterPool K8sClusterPool, testCases ...TestCase) Classifier {
	return Classifier{
		k8sClusterPool: k8sClusterPool,
		testCases:      testCases,
	}
}

// Classifier makes pairs from testCases and K8sClusters based on the specified test strategy
type Classifier struct {
	k8sClusterPool K8sClusterPool
	testCases      []TestCase
}

// Minimal creates a TestPool based on minimal test strategy.
// It means running every test case exactly once each on any provider and
// any version of the K8sCluster pool (the ones first available to use).
func (t Classifier) Minimal() TestPool {
	tests := make([]Test, 0, len(t.testCases))

	for i, testCase := range t.testCases {
		tests = append(tests, Test{
			testCase:   testCase,
			k8sCluster: t.k8sClusterPool[i%len(t.k8sClusterPool)],
		})
	}
	return tests
}

// VersionComplete creates a TestPool based on version complete test strategy.
// It means running every test case exactly len(versions) times each,
// using any provider and every version once per case of the K8sCluster pool.
func (t Classifier) VersionComplete() TestPool {
	k8sClustersByVersion := t.k8sClusterPool.getByVersions()
	tests := make(TestPool, 0, len(t.testCases)*len(maps.Keys(k8sClustersByVersion)))

	for _, k8sClusters := range k8sClustersByVersion {
		for i, testCase := range t.testCases {
			tests = append(tests, Test{
				testCase:   testCase,
				k8sCluster: k8sClusters[i%len(k8sClusters)],
			})
		}
	}
	return tests
}

// ProviderComplete creates a TestPool based on provider complete test strategy.
// It means running every test case exactly len(providers) times each,
// using any version and every provider once per case of the K8sCluster pool.
func (t Classifier) ProviderComplete() TestPool {
	k8sClustersByProviders := t.k8sClusterPool.getByProviders()
	tests := make(TestPool, 0, len(t.testCases)*len(maps.Keys(k8sClustersByProviders)))

	for _, k8sClusters := range k8sClustersByProviders {
		for i, testCase := range t.testCases {
			tests = append(tests, Test{
				testCase:   testCase,
				k8sCluster: k8sClusters[i%len(k8sClusters)],
			})
		}
	}
	return tests
}

// Complete creates a TestPool based on complete test strategy.
// It means running every test case exactly len(providers)*len(versions) times each,
// using every provider and every version combination once of the pool.
func (t Classifier) Complete() TestPool {
	k8sClustersByProvidersVersions := t.k8sClusterPool.getByProvidersVersions()
	var tests TestPool

	for _, byVersions := range k8sClustersByProvidersVersions {
		for _, k8sClusters := range byVersions {
			for i, testCase := range t.testCases {
				tests = append(tests, Test{
					testCase:   testCase,
					k8sCluster: k8sClusters[i%len(k8sClusters)],
				})
			}
		}
	}
	return tests
}

// K8sClusterPool is a representation of the K8sClusters.
type K8sClusterPool []K8sCluster

// FeedFomDirectory creates K8sClusters from kubeconfigs in the specified directory and add them into the pool.
func (k8sPool *K8sClusterPool) FeedFomDirectory(kubeConfigDirectoryPath string) error {
	files, err := os.ReadDir(kubeConfigDirectoryPath)
	if err != nil {
		return fmt.Errorf("unable to read kubeConfig directory '%s' error: %w", kubeConfigDirectoryPath, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("kubeConfig directory '%s' is empty", kubeConfigDirectoryPath)
	}

	for _, file := range files {
		if file != nil {
			kubectlPath := filepath.Join(kubeConfigDirectoryPath, file.Name())
			k8sClusters, err := NewK8sClustersFromParams(kubectlPath)

			if err != nil {
				return fmt.Errorf("could not get K8sClusters from file '%s' err: %w", kubectlPath, err)
			}
			*k8sPool = append(*k8sPool, k8sClusters...)
		}
	}
	return nil
}

func (k8sPool K8sClusterPool) getByVersions() map[string][]K8sCluster {
	byVersions := make(map[string][]K8sCluster)

	for _, k8sCluster := range k8sPool {
		byVersions[k8sCluster.clusterInfo.version] = append(byVersions[k8sCluster.clusterInfo.version], k8sCluster)
	}

	return byVersions
}

func (k8sPool K8sClusterPool) getByProviders() map[string][]K8sCluster {
	byProviders := make(map[string][]K8sCluster)

	for _, k8sCluster := range k8sPool {
		byProviders[k8sCluster.clusterInfo.provider] = append(byProviders[k8sCluster.clusterInfo.provider], k8sCluster)
	}

	return byProviders
}

func (k8sPool K8sClusterPool) getByProvidersVersions() map[string]map[string][]K8sCluster {
	byProvidersVersions := make(map[string]map[string][]K8sCluster)
	byProviders := k8sPool.getByProviders()

	for provider, k8sClusters := range byProviders {
		for _, k8sCluster := range k8sClusters {
			if byProvidersVersions[provider] == nil {
				byProvidersVersions[provider] = make(map[string][]K8sCluster)
			}
			byProvidersVersions[provider][k8sCluster.clusterInfo.version] = append(byProvidersVersions[provider][k8sCluster.clusterInfo.version], k8sCluster)
		}
	}
	return byProvidersVersions
}

// TestCase is a representation of an E2E test case.
//   - SpecsCount is the number of the gingko specs in the test.
//   - Duration specifies the expected length of the test.
//   - Name is the name of the test.
//   - TestFn specifies the test function.
type TestCase struct {
	SpecsCount int
	Duration   time.Duration
	Name       string
	TestFn     func(kubectlOptions k8s.KubectlOptions)
}

// NewTest creates a Test from a TestCase and from a K8sCluster.
// The testID for Test is generated automatically based on Name and K8s clusterInfo.
func NewTest(testCase TestCase, k8sCluster K8sCluster) Test {
	test := Test{
		testCase:   testCase,
		k8sCluster: k8sCluster,
	}
	test.generateTestID()

	return test
}

// Test represents an E2E test.
// It is a combination of a testCase and a k8sCluster.
type Test struct {
	testID     string
	testCase   TestCase
	k8sCluster K8sCluster
}

func (t Test) equal(test Test) bool {
	return t.TestID() == test.TestID()
}

func (t Test) less(test Test) bool {
	return t.TestID() < test.TestID()
}

// generateTestID generates the test UID based on the name of the test and the K8sCluster.
func (t *Test) generateTestID() string {
	text := fmt.Sprintf("%v%v", t.testCase.Name, t.k8sCluster)
	hash := md5.Sum([]byte(text))
	t.testID = hex.EncodeToString(hash[:5])

	return t.testID
}

// TestID returns the test UID based on the name of the test and the K8sCluster.
func (t *Test) TestID() string {
	return t.testID
}

func (t Test) String() string {
	return fmt.Sprintf("%s(%s)",
		t.testCase.Name, t.k8sCluster)
}

// Run encapsulates the testCase in a Describe container with
// K8s cluster health check and testID label.
// When K8s cluster is not available the further tests will be skipped.
func (t Test) Run() {
	Describe(fmt.Sprint(t), Ordered, decorators.Label(fmt.Sprintf("testID:%s", t.TestID())), func() {
		kubectlOptions, err := t.k8sCluster.KubectlOptions()

		if err == nil {
			_, err = k8s.ListPodsE(
				GinkgoT(),
				k8s.NewKubectlOptions(kubectlOptions.ContextName, kubectlOptions.ConfigPath, "kube-system"),
				metav1.ListOptions{},
			)
		}

		It("Checking K8s cluster health", func() {
			if err != nil {
				Fail(fmt.Sprintf("skipping testCase... K8s cluster connection problem: %v", err))
			}
		})
		t.testCase.TestFn(kubectlOptions)
	})
}

func newK8sClusterInfo(kubectlOptions k8s.KubectlOptions) (k8sClusterInfo, error) {
	version, err := k8s.GetKubernetesClusterVersionWithOptionsE(GinkgoT(), &kubectlOptions)
	if err != nil {
		return k8sClusterInfo{}, fmt.Errorf("could not get version: %w", err)
	}
	version, err = versionIdentifier(version)
	if err != nil {
		return k8sClusterInfo{}, fmt.Errorf("could not get version: %w", err)
	}
	kubeSystemNS, err := k8s.GetNamespaceE(GinkgoT(), &kubectlOptions, "kube-system")
	if err != nil {
		return k8sClusterInfo{}, fmt.Errorf("could not get UUID: %w", err)
	}
	nodes, err := k8s.GetNodesE(GinkgoT(), &kubectlOptions)
	if err != nil {
		return k8sClusterInfo{}, fmt.Errorf("could not get node count: %w", err)
	}
	if len(nodes) == 0 {
		return k8sClusterInfo{}, errors.New("node count is: 0")
	}

	provider, err := providerIdentifier(nodes[0])
	if err != nil {
		return k8sClusterInfo{}, fmt.Errorf("could not provider: %w", err)
	}

	return k8sClusterInfo{
		workerCount: len(nodes),
		clusterID:   string(kubeSystemNS.UID),
		provider:    string(provider),
		version:     version,
	}, nil
}

// k8sClusterInfo contains K8s cluster informations about K8sCluster
type k8sClusterInfo struct {
	workerCount int
	clusterID   string
	provider    string
	version     string
}

// NewK8sCluster retrieves the K8s cluster information and creates the K8sCluster resource.
// When it cannot get K8s cluster information it returns error.
func NewK8sCluster(kubectlOptions k8s.KubectlOptions) (K8sCluster, error) {
	clusterInfo, err := newK8sClusterInfo(kubectlOptions)
	if err != nil {
		return K8sCluster{}, fmt.Errorf("could not get clusterInfo for K8s cluster (path: '%s' context: '%s') err: %w",
			kubectlOptions.ConfigPath, kubectlOptions.ContextName, err)
	}
	return K8sCluster{
		clusterInfo:    clusterInfo,
		kubectlOptions: kubectlOptions,
	}, nil
}

// NewK8sClusterWithProvider creates K8sCluster from kubeconfig path and kubecontext with the
// specified provider information
func NewK8sClusterWithProvider(kubectlOptions k8s.KubectlOptions, provider string) (K8sCluster, error) {
	clusterInfo, err := newK8sClusterInfo(kubectlOptions)
	if err != nil {
		return K8sCluster{}, fmt.Errorf("could not get clusterInfo for K8s cluster (path: '%s' context: '%s') err: %w",
			kubectlOptions.ConfigPath, kubectlOptions.ContextName, err)
	}

	if provider != "" {
		clusterInfo.provider = provider
	}

	return K8sCluster{
		clusterInfo:    clusterInfo,
		kubectlOptions: kubectlOptions,
	}, nil
}

// NewMockK8sCluster creates K8sCluster objects from the provided argument for testing purpose
func NewMockK8sCluster(kubeConfigPath, kubeContext string, version, provider, clusterID string) K8sCluster {
	return K8sCluster{
		clusterInfo: k8sClusterInfo{
			version:   version,
			provider:  provider,
			clusterID: clusterID,
		},
		kubectlOptions: k8s.KubectlOptions{
			ConfigPath:  kubeConfigPath,
			ContextName: kubeContext,
		},
	}
}

// NewK8sClusterFromParams creates K8sCluster from kubectlConfigPath path and kubectlContext.
// Cluster information is fetched from the K8s cluster automatically.
func NewK8sClusterFromParams(kubectlConfigPath, kubectlContext string) (K8sCluster, error) {
	return NewK8sCluster(*k8s.NewKubectlOptions(kubectlContext, kubectlConfigPath, ""))
}

// NewK8sClustersFromParams creates K8sClusters based on kubeconfig path.
// When multiple context is found in the kubeconfig it creates multiple K8sClusters from all of them.
// Cluster information is fetched from the K8s cluster automatically at creation time.
func NewK8sClustersFromParams(kubectlConfigPath string) ([]K8sCluster, error) {
	kubeContexts, err := common.GetKubeContexts(kubectlConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not get kubecontexts: %w", err)
	}

	var k8sClusters []K8sCluster

	for _, kubeContext := range kubeContexts {
		k8sCluster, err := NewK8sCluster(*k8s.NewKubectlOptions(kubeContext, kubectlConfigPath, ""))
		if err != nil {
			return nil, err
		}
		k8sClusters = append(k8sClusters, k8sCluster)
	}
	return k8sClusters, nil
}

// NewK8sClusterFromCurrentConfig creates K8sCluster from the local KUBECONFIG env and it's current context.
// Cluster information is fetched from the K8s cluster automatically.
func NewK8sClusterFromCurrentConfig() (K8sCluster, error) {
	kubectlOptions, err := common.KubectlOptionsForCurrentContext()
	if err != nil {
		return K8sCluster{}, err
	}

	clusterInfo, err := newK8sClusterInfo(kubectlOptions)
	if err != nil {
		return K8sCluster{}, fmt.Errorf("could not get clusterInfo for K8s cluster (path: '%s' context: '%s') err: %w",
			kubectlOptions.ConfigPath, kubectlOptions.ContextName, err)
	}

	return K8sCluster{
		clusterInfo:    clusterInfo,
		kubectlOptions: kubectlOptions,
	}, nil
}

// K8sCluster represents a K8s cluster.
// It contains informations and access related configurations about K8s cluster.
type K8sCluster struct {
	clusterInfo    k8sClusterInfo
	kubectlOptions k8s.KubectlOptions
}

func (c K8sCluster) isKubectlOptionsFilled() bool {
	return c.kubectlOptions.ConfigPath != "" && c.kubectlOptions.ContextName != ""
}

// KubectlOptions returns K8s access related configurations for kubectl.
func (c K8sCluster) KubectlOptions() (k8s.KubectlOptions, error) {
	if !c.isKubectlOptionsFilled() {
		return k8s.KubectlOptions{}, errors.New("kubectlOptions is unfilled")
	}
	return c.kubectlOptions, nil
}

func (c K8sCluster) String() string {
	if !c.isKubectlOptionsFilled() {
		return "Unknown"
	}
	return fmt.Sprintf(c.kubectlOptions.ContextName)
}
