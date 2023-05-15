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

package scale

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBrokerIDsAndLogDirToMap(t *testing.T) {
	testCases := []struct {
		testName            string
		brokerIDsAndLogDirs string
		want                map[int32][]string
		wantErr             bool
	}{
		{
			testName:            "valid input",
			brokerIDsAndLogDirs: "102-/kafka-logs3/kafka,101-/kafka-logs3/kafka,101-/kafka-logs2/kafka",
			want: map[int32][]string{
				101: {"/kafka-logs3/kafka", "/kafka-logs2/kafka"},
				102: {"/kafka-logs3/kafka"},
			},
			wantErr: false,
		},
		{
			testName:            "empty input",
			brokerIDsAndLogDirs: "",
			want:                map[int32][]string{},
			wantErr:             false,
		},
		{
			testName:            "invalid format",
			brokerIDsAndLogDirs: "1-dirA,2-dirB,1",
			want:                nil,
			wantErr:             true,
		},
		{
			testName:            "invalid broker ID",
			brokerIDsAndLogDirs: "1-dirA,abc-dirB,1-dirC",
			want:                nil,
			wantErr:             true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := parseBrokerIDsAndLogDirsToMap(tc.brokerIDsAndLogDirs)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
