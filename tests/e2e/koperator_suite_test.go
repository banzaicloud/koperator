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

//go:build e2e

package e2e

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/banzaicloud/koperator/tests/e2e/pkg/common/config"
	"github.com/banzaicloud/koperator/tests/e2e/pkg/tests"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var testPool tests.TestPool

func beforeSuite() (tests.TestPool, error) {
	k8sClusterPool := tests.K8sClusterPool{}
	if err := k8sClusterPool.FeedFomDirectory(viper.GetString(config.Tests.KubeConfigDirectoryPath)); err != nil {
		return nil, err
	}

	// k8sCluster, err := tests.New
	// if err != nil {
	// 	return nil, err
	// }
	//k8sClusterPool.AddK8sClusters(k8sCluster)

	classifier := tests.NewClassifier(k8sClusterPool, testCaseInstall)

	//classifier := tests.MockTestsMinimal()
	//classifier := tests.MockTestsProvider()
	//classifier := tests.MockTestsProviderMoreTestsThenProvider()
	//classifier := tests.MockTestsVersionOne()
	// classifier := tests.MockTestsVersion()
	//classifier := tests.MockTestsComplete()

	var testPool tests.TestPool
	testStrategy := viper.GetString(config.Tests.TestStrategy)

	switch testStrategy {
	case config.TestStrategyMinimal:
		testPool = classifier.Minimal()
	case config.TestStrategyVersionComplete:
		testPool = classifier.VersionComplete()
	case config.TestStrategyProviderComplete:
		testPool = classifier.ProviderComplete()
	case config.TestStrategyComplete:
		testPool = classifier.Complete()
	}

	return testPool, nil
}

func TestKoperator(t *testing.T) {
	var err error

	err = runGinkgoTests(t)
	if err != nil {
		t.Errorf("Koperator E2E start failed: %v", err)
	}
}

func runGinkgoTests(t *testing.T) error {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()

	// Run only selected tests by testID label e.g: "testID:4e980f5b5c"
	if labelFilter := viper.GetString(config.Tests.LabelFilter); labelFilter != "" {
		suiteConfig.LabelFilter = labelFilter
	}

	var err error
	// Generated and load tests into the pool
	testPool, err = beforeSuite()
	if err != nil {
		return fmt.Errorf("beforeSuite ran into err: %w", err)
	}

	testSuiteDuration := testPool.GetTestSuiteDurationSerial()
	if suiteConfig.ParallelProcess > 1 {
		testSuiteDuration = testPool.GetTestSuiteDurationParallel()
	}

	maxTimeout, err := time.ParseDuration(viper.GetString(config.Tests.MaxTimeout))
	if err != nil {
		return fmt.Errorf("could not parse MaxTimeout into time.Duration: %w", err)
	}
	// Protection against too long test suites
	if testSuiteDuration > maxTimeout {
		return fmt.Errorf("tests estimated duration: '%dsec' bigger then maxTimeout: '%dsec'", int(testSuiteDuration.Seconds()), int(maxTimeout.Seconds()))
	}

	// Calculated timeout can be overran with the specified time length
	allowedOverrun, err := time.ParseDuration(viper.GetString(config.Tests.AllowedOverrunDuration))
	if err != nil {
		return fmt.Errorf("could not parse AllowedOverrunDuration into time.Duration: %w", err)
	}
	// Set TestSuite timeout based on the generated tests
	suiteConfig.Timeout = testSuiteDuration + allowedOverrun

	if viper.GetBool(config.Tests.CreateTestReportFile) {
		if err := createTestReportFile(); err != nil {
			return err
		}
	}

	testDescription := fmt.Sprintf("\n%s\nConfigurations: \n%s%s\nPoolInfo: \n%s%s\n",
		sectionStringDelimiter(), config.Tests, sectionStringDelimiter(), testPool.PoolInfo(), sectionStringDelimiter())

	func() {
		defer ginkgo.GinkgoRecover()
		RunSpecs(t, testDescription, suiteConfig, reporterConfig)
	}()

	return nil

}

// Maybe this can be used to get more debug information into the test report.
// var _ = ReportAfterEach(func(report SpecReport) {
// 	fmt.Fprintf(os.Stderr, "SPEC REPORT: %s | %s\nFAILURE MESSAGE: %s\n OUTPUT: %s\n", report.State, report.FullText(), report.FailureMessage(), report.CapturedStdOutErr)
// })

// Root Describe container
var _ = Describe("", func() {
	// In the root container there is no Ordered decorator
	// ginkgo execute parallel the generated tests by K8sClusters
	testPool.BuildParallelByK8sCluster()
})

func sectionStringDelimiter() string {
	delimiter := ""
	for i := 0; i < 100; i++ {
		delimiter += "-"
	}
	return delimiter
}

func createTestReportFile() error {
	reportDir := viper.GetString(config.Tests.ReportDir)
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		if err := os.Mkdir(reportDir, os.FileMode(0o777)); err != nil {
			return fmt.Errorf("error while creating report directory %s err: %s", reportDir, err.Error())
		}
	}
	// Generate JUnit report once all tests have finished with customized settings
	_ = ginkgo.ReportAfterSuite("Koperator e2e", func(report ginkgo.Report) {
		err := reporters.GenerateJUnitReportWithConfig(
			report,
			filepath.Join(reportDir, fmt.Sprintf("e2e_%s_%v.xml", viper.GetString(config.Tests.TestStrategy), time.Now().Format(time.RFC3339))),
			reporters.JunitReportConfig{OmitSpecLabels: false, OmitLeafNodeType: false},
		)
		if err != nil {
			log.Printf("error creating junit report file %s", err.Error())
		}
	})
	return nil
}
