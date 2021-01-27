// Copyright © 2020 Banzai Cloud
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

package kafka

import (
	"testing"

	"github.com/banzaicloud/kafka-operator/pkg/util"
	properties "github.com/banzaicloud/kafka-operator/properties/pkg"
)

func TestShouldRefreshOnlyPerBrokerConfigs(t *testing.T) {
	testCases := []struct {
		Description    string
		CurrentConfigs string
		DesiredConfigs string
		Result         bool
	}{
		{
			Description: "configs did not change",
			CurrentConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=unmodified_value_2
`,
			DesiredConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=unmodified_value_2
`,
			Result: true,
		},
		{
			Description: "only non per-broker config changed",
			CurrentConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=modified_value_2
`,
			DesiredConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=modified_value_3
`,
			Result: false,
		},
		{
			Description: "only per-broker config changed",
			CurrentConfigs: `unmodified_config_1=unmodified_value_1
ssl.client.auth=modified_value_2
`,
			DesiredConfigs: `unmodified_config_1=unmodified_value_1
ssl.client.auth=modified_value_3
`,
			Result: true,
		},
		{
			Description: "per-broker and non per-broker configs have changed",
			CurrentConfigs: `modified_config_1=modified_value_1
ssl.client.auth=modified_value_3
`,
			DesiredConfigs: `modified_config_1=modified_value_2
ssl.client.auth=modified_value_4
`,
			Result: false,
		},
		{
			Description:    "security protocol map can be changed as a per-broker config",
			CurrentConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol2",
			DesiredConfigs: "listener.security.protocol.map=listener1:protocol1,listener3:protocol3",
			Result:         true,
		},
		{
			Description:    "security protocol map can't be changed as a per-broker config",
			CurrentConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol2",
			DesiredConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol3",
			Result:         false,
		},
		{
			Description:    "security protocol map added as config",
			CurrentConfigs: "",
			DesiredConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol2",
			Result:         true,
		},
	}
	logger := util.CreateLogger(false, false)
	for i, testCase := range testCases {
		current, err := properties.NewFromString(testCase.CurrentConfigs)
		if err != nil {
			t.Fatalf("failed to parse Properties from string: %s", testCase.CurrentConfigs)
		}
		desired, err := properties.NewFromString(testCase.DesiredConfigs)
		if err != nil {
			t.Fatalf("failed to parse Properties from string: %s", testCase.DesiredConfigs)
		}
		if ShouldRefreshOnlyPerBrokerConfigs(current, desired, logger) != testCase.Result {
			t.Errorf("test case %d failed: %s", i, testCase.Description)
		}
	}
}
