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
	"log"
	"math/rand"
	"os"
	"path/filepath"
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

type TestPool []TestType

func (tests TestPool) PoolInfo() string {
	testsByContextName := tests.getTestsByContextName()

	return fmt.Sprintf(`
NumberOfTests: %d
NumberOfK8sClusters: %d
NumberOfK8sVersions:
NumberOfK8sProviders:
TestK8sMapping: %v
TestStrategy: %s
ExpectedDurationSerial: %f
ExpectedDurationParallel: %f
`, len(tests), len(maps.Keys(testsByContextName)), tests, viper.GetString(config.Tests.TestStrategy), tests.GetTestSuiteDurationSerial().Seconds(), tests.GetTestSuiteDurationParallel().Seconds())
}

func (tests TestPool) BuildParallelByK8sCluster() {
	testsByClusterID := tests.getTestsByClusterID()
	// Because of the "Ordered" decorator testCases inside that container will run each after another
	// so this K8s cluster is dedicated for tests for serial test execution
	for clusterID, tests := range testsByClusterID {
		Describe(clusterID, decorators.Label(fmt.Sprintf("clusterID:%s", clusterID)), Ordered, func() {
			for _, t := range tests {
				t.Run()
			}
		})
	}
}

func (tests TestPool) getTestsByClusterID() map[string][]TestType {
	testsByClusterID := make(map[string][]TestType)
	for _, test := range tests {
		testsByClusterID[test.k8sCluster.clusterInfo.clusterID] = append(testsByClusterID[test.k8sCluster.clusterInfo.clusterID], test)
	}

	return testsByClusterID
}

func (tests TestPool) getTestsByContextName() map[string][]TestType {
	testsByContextName := make(map[string][]TestType)
	for _, test := range tests {
		testsByContextName[test.k8sCluster.kubectlOptions.ContextName] = append(testsByContextName[test.k8sCluster.kubectlOptions.ContextName], test)
	}

	return testsByContextName
}

func (tests TestPool) GetTestSuiteDurationParallel() time.Duration {
	testsByClusterID := tests.getTestsByClusterID()

	var max time.Duration
	for _, tests := range testsByClusterID {
		var localDuration time.Duration
		for _, test := range tests {
			localDuration += test.testCase.TestDuration
		}

		if localDuration > max {
			max = localDuration
		}
	}
	return max
}

func (tests TestPool) GetTestSuiteDurationSerial() time.Duration {
	var allDuration time.Duration
	for _, test := range tests {
		allDuration += test.testCase.TestDuration
	}
	return allDuration
}

func NewClassifier(k8sClusterPool K8sClusterPool, testCases ...TestCase) Classifier {
	return Classifier{
		k8sClusterPool: k8sClusterPool,
		testCases:      testCases,
	}
}

type Classifier struct {
	k8sClusterPool K8sClusterPool
	testCases      []TestCase
}

func (t Classifier) Minimal() []TestType {
	t.k8sClusterPool.mixPool()
	tests := make([]TestType, 0, len(t.testCases))

	for i, testCase := range t.testCases {
		tests = append(tests, TestType{
			testCase:   testCase,
			k8sCluster: t.k8sClusterPool.k8sClusters[i%len(t.k8sClusterPool.k8sClusters)],
		})
	}
	return tests
}

func (t Classifier) VersionComplete() []TestType {
	t.k8sClusterPool.mixPool()
	k8sClustersByVersion := t.k8sClusterPool.getByVersions()
	tests := make([]TestType, 0, len(t.testCases)*len(maps.Keys(k8sClustersByVersion)))

	for _, k8sClusters := range k8sClustersByVersion {
		for i, testCase := range t.testCases {
			tests = append(tests, TestType{
				testCase:   testCase,
				k8sCluster: k8sClusters[i%len(k8sClusters)],
			})
		}
	}
	return tests
}

func (t Classifier) ProviderComplete() []TestType {
	t.k8sClusterPool.mixPool()
	k8sClustersByProviders := t.k8sClusterPool.getByProviders()
	tests := make([]TestType, 0, len(t.testCases)*len(maps.Keys(k8sClustersByProviders)))

	for _, k8sClusters := range k8sClustersByProviders {
		for i, testCase := range t.testCases {
			tests = append(tests, TestType{
				testCase:   testCase,
				k8sCluster: k8sClusters[i%len(k8sClusters)],
			})
		}
	}
	return tests
}

func (t Classifier) Complete() []TestType {
	t.k8sClusterPool.mixPool()
	k8sClustersByProvidersVersions := t.k8sClusterPool.getByProvidersVersions()
	var tests []TestType

	for _, byVersions := range k8sClustersByProvidersVersions {
		for _, k8sClusters := range byVersions {
			for i, testCase := range t.testCases {
				tests = append(tests, TestType{
					testCase:   testCase,
					k8sCluster: k8sClusters[i%len(k8sClusters)],
				})
			}
		}
	}
	return tests
}

type K8sClusterPool struct {
	k8sClusters []K8sCluster
}

func (t *K8sClusterPool) mixPool() {
	rand.Shuffle(len(t.k8sClusters), func(i, j int) { t.k8sClusters[i], t.k8sClusters[j] = t.k8sClusters[j], t.k8sClusters[i] })
}

func (t *K8sClusterPool) AddK8sClusters(cluster ...K8sCluster) {
	t.k8sClusters = append(t.k8sClusters, cluster...)
}

func (t *K8sClusterPool) FeedFomDirectory(kubeConfigDirectoryPath string) error {
	files, err := os.ReadDir(kubeConfigDirectoryPath)
	if err != nil {
		return fmt.Errorf("unable to read kubeConfig directory '%s' error: %w", kubeConfigDirectoryPath, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("kubeConfig directory '%s' is empty", kubeConfigDirectoryPath)
	}

	for _, file := range files {
		if file != nil {
			kubeContext, err := common.GetDefaultKubeContext(file.Name())
			if err != nil {
				err := fmt.Errorf("could not fetch default kubeContext from file: %s, err: %w", file.Name(), err)
				log.Print(err)
				continue
			}
			kubectlPath := filepath.Join(kubeConfigDirectoryPath, file.Name())
			k8sCluster, err := NewK8sClusterFromParams(kubectlPath, kubeContext, true)
			if err != nil {
				return fmt.Errorf("could not create K8sCluster structure from file '%s' err: %w", kubectlPath, err)
			}
			t.AddK8sClusters(k8sCluster)
		}
	}
	return nil
}

func (t K8sClusterPool) getByVersions() map[string][]K8sCluster {
	byVersions := make(map[string][]K8sCluster)

	for _, k8sCluster := range t.k8sClusters {
		byVersions[k8sCluster.clusterInfo.version] = append(byVersions[k8sCluster.clusterInfo.version], k8sCluster)
	}

	return byVersions
}

func (t K8sClusterPool) getByProviders() map[string][]K8sCluster {
	byProviders := make(map[string][]K8sCluster)

	for _, k8sCluster := range t.k8sClusters {
		byProviders[k8sCluster.clusterInfo.provider] = append(byProviders[k8sCluster.clusterInfo.provider], k8sCluster)
	}

	return byProviders
}

func (t K8sClusterPool) getByProvidersVersions() map[string]map[string][]K8sCluster {
	byProvidersVersions := make(map[string]map[string][]K8sCluster)
	byProviders := t.getByProviders()

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

type TestCase struct {
	TestDuration time.Duration
	TestName     string
	TestFn       func(kubectlOptions k8s.KubectlOptions)
}

func NewTest(testCase TestCase, k8sCluster K8sCluster) TestType {
	testIDseed := fmt.Sprintf("%v%v", testCase.TestName, k8sCluster.clusterInfo)
	hash := md5.Sum([]byte(testIDseed))
	testID := hex.EncodeToString(hash[:5])
	return TestType{
		testID:     testID,
		testCase:   testCase,
		k8sCluster: k8sCluster,
	}
}

type TestType struct {
	testID     string
	testCase   TestCase
	k8sCluster K8sCluster
}

func (t TestType) Equal(test TestType) bool {
	return t.TestID() == test.TestID()
}

func (t TestType) Less(test TestType) bool {
	return t.TestID() < test.TestID()
}

func (t *TestType) TestID() string {
	if t.testID == "" {
		text := fmt.Sprintf("%v%v", t.testCase.TestName, t.k8sCluster)
		hash := md5.Sum([]byte(text))
		t.testID = hex.EncodeToString(hash[:5])
	}
	return t.testID
}

func (t TestType) String() string {
	return fmt.Sprintf("%s(%s)",
		t.testCase.TestName, t.k8sCluster)
}

// Run encapsulates the testCase with K8s health check and details into a describe container
// When K8s cluster is not available the further tests are skipped
func (t TestType) Run() {
	Describe(fmt.Sprint(t), Ordered, decorators.Label(fmt.Sprintf("testID:%s", t.TestID())), func() {
		var err error
		kubectlOptions, err := t.k8sCluster.KubectlOptions()
		//TODO (marbarta): this should be removed after testing
		if err != nil {
			kubectlOptions, err = common.KubectlOptionsForCurrentContext()
		}

		if err == nil {
			_, err = k8s.ListPodsE(
				GinkgoT(),
				k8s.NewKubectlOptions(kubectlOptions.ContextName, kubectlOptions.ConfigPath, "kube-system"),
				metav1.ListOptions{},
			)
		}

		// It("Checking K8s cluster health", func() {
		// 	if err != nil {
		// 		Fail(fmt.Sprintf("skipping testCase... K8s cluster connection problem: %v", err))
		// 	}
		// })

		t.testCase.TestFn(kubectlOptions)
	})
}

func newK8sClusterInfo(kubectlOptions k8s.KubectlOptions) (K8sClusterInfo, error) {
	// TODO (marbarta): placeholder to get retrieve cluster information from kubeconfig
	return K8sClusterInfo{
		workerCount: 3,
		multiAZ:     false,
		clusterID:   "testCLusterID",
		provider:    "testProvider",
		version:     "1.25",
	}, nil
}

type K8sClusterInfo struct {
	workerCount int
	multiAZ     bool
	clusterID   string
	provider    string
	version     string
}

// NewK8sCluster retrieve the K8s cluster information and creates the K8sCluster resource
// When K8s cluster information is not available it return error
func NewK8sCluster(kubectlOptions k8s.KubectlOptions, reusable bool) (K8sCluster, error) {
	clusterInfo, err := newK8sClusterInfo(kubectlOptions)
	if err != nil {
		return K8sCluster{}, fmt.Errorf("could not get clusterInfo: %w", err)
	}
	return K8sCluster{
		clusterInfo:    clusterInfo,
		reusable:       reusable,
		kubectlOptions: kubectlOptions,
	}, nil
}

func NewMockK8sCluster(kubeConfigPath, kubeContext string, reusable bool, version, provider, clusterID string) K8sCluster {
	return K8sCluster{
		clusterInfo: K8sClusterInfo{
			version:   version,
			provider:  provider,
			clusterID: clusterID,
		},
		reusable: reusable,
		kubectlOptions: k8s.KubectlOptions{
			ConfigPath:  kubeConfigPath,
			ContextName: kubeContext,
		},
	}
}

func NewK8sClusterFromParams(kubectlConfigPath, kubectlContext string, reusable bool) (K8sCluster, error) {
	return NewK8sCluster(*k8s.NewKubectlOptions(kubectlContext, kubectlConfigPath, ""), reusable)
}

func NewK8sClusterFromCurrentConfig(reusable bool) (K8sCluster, error) {
	kubectlOptions, err := common.KubectlOptionsForCurrentContext()
	if err != nil {
		return K8sCluster{}, err
	}

	clusterInfo, err := newK8sClusterInfo(kubectlOptions)
	if err != nil {
		return K8sCluster{}, fmt.Errorf("could not get clusterInfo: %w", err)
	}

	return K8sCluster{
		clusterInfo:    clusterInfo,
		reusable:       reusable,
		kubectlOptions: kubectlOptions,
	}, nil
}

type K8sCluster struct {
	reusable       bool
	clusterInfo    K8sClusterInfo
	kubectlOptions k8s.KubectlOptions
}

func (c K8sCluster) isKubectlOptionsFilled() bool {
	return c.kubectlOptions.ConfigPath != "" && c.kubectlOptions.ContextName != ""
}

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
	// return fmt.Sprintf("Provider: %s Version: %s KubeContextPath: %s KubeContext: %s",
	// 	c.clusterInfo.provider, c.clusterInfo.version, c.kubectlOptions.ConfigPath, c.kubectlOptions.ContextName)
	return fmt.Sprintf(c.clusterInfo.clusterID)
}
