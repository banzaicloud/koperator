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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
)

func createCCRetryExecutionOperation(createTime time.Time, id string, operation v1alpha1.CruiseControlTaskOperation) *v1alpha1.CruiseControlOperation {
	return &v1alpha1.CruiseControlOperation{
		ObjectMeta: v1.ObjectMeta{
			CreationTimestamp: v1.Time{
				Time: createTime,
			},
		},
		Spec: v1alpha1.CruiseControlOperationSpec{
			ErrorPolicy: v1alpha1.ErrorPolicyRetry,
		},
		Status: v1alpha1.CruiseControlOperationStatus{
			CurrentTask: &v1alpha1.CruiseControlTask{
				ID:        id,
				Operation: operation,
				State:     v1beta1.CruiseControlTaskCompletedWithError,
			},
		},
	}
}

func TestSortOperations(t *testing.T) {
	timeNow := time.Now()
	testCases := []struct {
		testName       string
		ccOperations   []*v1alpha1.CruiseControlOperation
		expectedOutput []*v1alpha1.CruiseControlOperation
	}{
		{
			testName: "creation time",
			ccOperations: []*v1alpha1.CruiseControlOperation{
				createCCRetryExecutionOperation(timeNow.Add(3*time.Second), "1", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow, "2", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "3", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(2*time.Second), "4", v1alpha1.OperationAddBroker),
			},
			expectedOutput: []*v1alpha1.CruiseControlOperation{
				createCCRetryExecutionOperation(timeNow, "2", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "3", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(2*time.Second), "4", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(3*time.Second), "1", v1alpha1.OperationAddBroker),
			},
		},
		{
			testName: "mixed",
			ccOperations: []*v1alpha1.CruiseControlOperation{
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "1", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "2", v1alpha1.OperationRemoveBroker),
				createCCRetryExecutionOperation(timeNow, "3", v1alpha1.OperationRebalance),
				createCCRetryExecutionOperation(timeNow, "4", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow, "5", v1alpha1.OperationRemoveBroker),
			},
			expectedOutput: []*v1alpha1.CruiseControlOperation{
				createCCRetryExecutionOperation(timeNow, "4", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "1", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow, "5", v1alpha1.OperationRemoveBroker),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "2", v1alpha1.OperationRemoveBroker),
				createCCRetryExecutionOperation(timeNow, "3", v1alpha1.OperationRebalance),
			},
		},
		{
			testName: "mixed with remove disks",
			ccOperations: []*v1alpha1.CruiseControlOperation{
				createCCRetryExecutionOperation(timeNow, "1", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow, "4", v1alpha1.OperationRebalance),
				createCCRetryExecutionOperation(timeNow.Add(2*time.Second), "3", v1alpha1.OperationRemoveDisks),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "2", v1alpha1.OperationRemoveBroker),
			},
			expectedOutput: []*v1alpha1.CruiseControlOperation{
				createCCRetryExecutionOperation(timeNow, "1", v1alpha1.OperationAddBroker),
				createCCRetryExecutionOperation(timeNow.Add(time.Second), "2", v1alpha1.OperationRemoveBroker),
				createCCRetryExecutionOperation(timeNow.Add(2*time.Second), "3", v1alpha1.OperationRemoveDisks),
				createCCRetryExecutionOperation(timeNow, "4", v1alpha1.OperationRebalance),
			},
		},
	}
	for _, testCase := range testCases {
		sortedCCOperations := sortOperations(testCase.ccOperations)
		sortedRetryOutput := sortedCCOperations[ccOperationRetryExecution]
		assert.Equal(t, sortedRetryOutput, testCase.expectedOutput, "test", testCase.testName)
	}
}
