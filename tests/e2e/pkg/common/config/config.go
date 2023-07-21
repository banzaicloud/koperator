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

package config

import "github.com/spf13/viper"

type TestStrategy struct{}

const (
	TestStrategyMinimal          = "minimal"
	TestStrategyVersionComplete  = "version"
	TestStrategyProviderComplete = "provider"
	TestStrategyComplete         = "complete"
)

var Tests = struct {
	ReportDir              string
	CreateTestReportFile   string
	MaxTimeout             string
	AllowedOverrunDuration string
	TestStrategy           string
	LabelFilter            string
}{
	CreateTestReportFile:   "tests.CreateTestReportFile",
	MaxTimeout:             "tests.MaxTimeout",
	AllowedOverrunDuration: "tests.AllowedOverrunDuration",
	TestStrategy:           "tests.TestStrategy",
	ReportDir:              "tests.ReportDir",
	LabelFilter:            "tests.LabelFilter",
}

func init() {
	viper.AutomaticEnv()

	viper.BindEnv(Tests.CreateTestReportFile, "CREATE_TEST_REPORT_FILE")
	viper.SetDefault(Tests.CreateTestReportFile, true)

	viper.BindEnv(Tests.ReportDir, "REPORT_DIR")
	viper.SetDefault(Tests.ReportDir, "reports")

	viper.BindEnv(Tests.MaxTimeout, "MAX_TIMEOUT")
	viper.SetDefault(Tests.MaxTimeout, "10s")

	viper.BindEnv(Tests.AllowedOverrunDuration, "ALLOWED_OVERRUN_DURATION")
	viper.SetDefault(Tests.AllowedOverrunDuration, "5s")

	viper.BindEnv(Tests.TestStrategy, "TEST_STRATEGY")
	viper.SetDefault(Tests.TestStrategy, TestStrategyMinimal)

	viper.BindEnv(Tests.LabelFilter, "TEST_MODE")

}
