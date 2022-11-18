// Copyright (c) 2019 [Cisco Systems, Inc.](https://www.cisco.com) and/or its affiliates
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

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrokersJBODSelector(t *testing.T) {
	testCases := []struct {
		testName               string
		capacityConfig         string
		brokerIDs              []string
		expectedBrokersJBOD    []string
		expectedBrokersNotJBOD []string
	}{
		{
			testName:               "All JBOD because there is no custom config",
			brokerIDs:              []string{"0", "1", "2"},
			capacityConfig:         "",
			expectedBrokersJBOD:    []string{"0", "1", "2"},
			expectedBrokersNotJBOD: []string(nil),
		},
		{
			testName:  "All JBOD",
			brokerIDs: []string{"0", "1"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "0",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"0", "1"},
			expectedBrokersNotJBOD: []string(nil),
		},
		{
			testName:  "All JBOD and brokerID selection",
			brokerIDs: []string{"1"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "0",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"1"},
			expectedBrokersNotJBOD: []string(nil),
		},
		{
			testName:  "All JBOD and more brokerID selection",
			brokerIDs: []string{"1", "3"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "0",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"1", "3"},
			expectedBrokersNotJBOD: []string(nil),
		},
		{
			testName:  "All JBOD and more brokerID selection v2",
			brokerIDs: []string{"3", "4"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "0",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"4", "3"},
			expectedBrokersNotJBOD: []string(nil),
		},
		{
			testName:  "JBOD and not JBOD",
			brokerIDs: []string{"0", "1", "2"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "0",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "2",
				"capacity": {
				  "DISK": "5000",
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"0", "1"},
			expectedBrokersNotJBOD: []string{"2"},
		},
		{
			testName:  "All JBOD with -1 default",
			brokerIDs: []string{"0", "1", "2"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "-1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"0", "1", "2"},
			expectedBrokersNotJBOD: []string(nil),
		},
		{
			testName:  "All not JBOD with -1 default",
			brokerIDs: []string{"0", "1", "2"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "-1",
				"capacity": {
				  "DISK": "54343",
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string(nil),
			expectedBrokersNotJBOD: []string{"0", "1", "2"},
		},
		{
			testName:  "not JBOD with -1 default but there is one JBOD overwrite default ",
			brokerIDs: []string{"0", "1", "2"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "-1",
				"capacity": {
				  "DISK": "54343",
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"1"},
			expectedBrokersNotJBOD: []string{"0", "2"},
		},
		{
			testName:  "JBOD with -1 default but there is one not JBOD overwrite default ",
			brokerIDs: []string{"0", "1", "2"},
			capacityConfig: `
			{
			"brokerCapacities":[
			  {
				"brokerId": "-1",
				"capacity": {
				  "DISK": {"/tmp/kafka-logs": "500000"},
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  },
			  {
				"brokerId": "1",
				"capacity": {
				  "DISK": "5000",
				  "CPU": "100",
				  "NW_IN": "50000",
				  "NW_OUT": "50000"
				}
			  }
			]
		  }`,
			expectedBrokersJBOD:    []string{"0", "2"},
			expectedBrokersNotJBOD: []string{"1"},
		},
	}
	for _, testCase := range testCases {
		brokersJBOD, brokersNotJBOD, err := brokersJBODSelector(testCase.brokerIDs, testCase.capacityConfig)
		assert.NoError(t, err)
		assert.ElementsMatch(t, testCase.expectedBrokersJBOD, brokersJBOD, "testName", testCase.testName)
		assert.ElementsMatch(t, testCase.expectedBrokersNotJBOD, brokersNotJBOD, "testName", testCase.testName)
	}
}
