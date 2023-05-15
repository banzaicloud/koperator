// Copyright © 2022 Cisco Systems, Inc. and/or its affiliates
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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/koperator/pkg/scale"

	banzaiv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/kafka/mocks"
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

func TestCreateCCOperation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ttlSecondsAfterFinished := 60

	testCases := []struct {
		operationType      banzaiv1alpha1.CruiseControlTaskOperation
		brokerIDs          []string
		isJBOD             bool
		brokerIdsToLogDirs map[string][]string
		parameterCheck     func(t *testing.T, params map[string]string)
	}{
		{
			operationType:      banzaiv1alpha1.OperationAddBroker,
			brokerIDs:          []string{"1", "2", "3"},
			isJBOD:             false,
			brokerIdsToLogDirs: nil,
			parameterCheck: func(t *testing.T, params map[string]string) {
				assert.Equal(t, "1,2,3", params[scale.ParamBrokerID])
				assert.Equal(t, "true", params[scale.ParamExcludeDemoted])
				assert.Equal(t, "true", params[scale.ParamExcludeRemoved])
			},
		},
		{
			operationType:      banzaiv1alpha1.OperationRemoveBroker,
			brokerIDs:          []string{"1"},
			isJBOD:             false,
			brokerIdsToLogDirs: nil,
			parameterCheck: func(t *testing.T, params map[string]string) {
				assert.Equal(t, "1", params[scale.ParamBrokerID])
				assert.Equal(t, "true", params[scale.ParamExcludeDemoted])
				assert.Equal(t, "true", params[scale.ParamExcludeRemoved])
			},
		},
		{
			operationType: banzaiv1alpha1.OperationRemoveDisks,
			brokerIDs:     []string{"1", "2"},
			isJBOD:        false,
			brokerIdsToLogDirs: map[string][]string{
				"1": {"logdir1", "logdir2"},
				"2": {"logdir1"},
			},
			parameterCheck: func(t *testing.T, params map[string]string) {
				// can be in any order
				expectedString1 := "1-logdir1,1-logdir2,2-logdir1"
				expectedString2 := "2-logdir1,1-logdir1,1-logdir2"
				assert.Contains(t, []string{expectedString1, expectedString2}, params[scale.ParamBrokerIDAndLogDirs])
			},
		},
		{
			operationType:      banzaiv1alpha1.OperationRebalance,
			brokerIDs:          []string{"1", "2", "3"},
			isJBOD:             true,
			brokerIdsToLogDirs: nil,
			parameterCheck: func(t *testing.T, params map[string]string) {
				assert.Equal(t, "1,2,3", params[scale.ParamDestbrokerIDs])
				assert.Equal(t, "true", params[scale.ParamRebalanceDisk])
				assert.Equal(t, "true", params[scale.ParamExcludeDemoted])
				assert.Equal(t, "true", params[scale.ParamExcludeRemoved])
			},
		},
	}

	for _, testCase := range testCases {
		mockClient := new(mocks.Client)
		scheme := runtime.NewScheme()
		_ = v1beta1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		r := CruiseControlTaskReconciler{
			Client: mockClient,
			Scheme: scheme,
		}

		kafkaCluster := &v1beta1.KafkaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kafka",
				Namespace: "kafka",
			}}

		// Mock the Create call and capture the operation
		var createdOperation *banzaiv1alpha1.CruiseControlOperation
		mockClient.On("Create", ctx, mock.IsType(&banzaiv1alpha1.CruiseControlOperation{})).Run(func(args mock.Arguments) {
			createdOperation = args.Get(1).(*banzaiv1alpha1.CruiseControlOperation)
			createdOperation.ObjectMeta.Name = "generated-name"
		}).Return(nil)

		// Mock the Status call
		mockClient.On("Status").Return(mockClient)

		// Mock the Update call
		mockClient.On("Update", ctx, mock.IsType(&banzaiv1alpha1.CruiseControlOperation{})).Run(func(args mock.Arguments) {
			arg := args.Get(1).(*banzaiv1alpha1.CruiseControlOperation)
			createdOperation.Status = arg.Status
		}).Return(nil)

		_, err := r.createCCOperation(ctx, kafkaCluster, banzaiv1alpha1.ErrorPolicyRetry, &ttlSecondsAfterFinished, testCase.operationType, testCase.brokerIDs, testCase.isJBOD, testCase.brokerIdsToLogDirs)
		assert.NoError(t, err)

		// Use the captured operation for further assertions
		assert.Equal(t, testCase.operationType, createdOperation.Status.CurrentTask.Operation)
		testCase.parameterCheck(t, createdOperation.Status.CurrentTask.Parameters)
	}
}
