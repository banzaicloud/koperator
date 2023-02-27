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

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/webhooks"
)

func TestIsTopicManagedByKoperator(t *testing.T) {
	testCases := []struct {
		testName   string
		kafkaTopic v1alpha1.KafkaTopic
		expected   bool
	}{
		{
			testName: "no managedBy annotation",
			kafkaTopic: v1alpha1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: true,
		},
		{
			testName: "has managedBy annotation with koperator value",
			kafkaTopic: v1alpha1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{webhooks.TopicManagedByAnnotationKey: webhooks.TopicManagedByKoperatorAnnotationValue},
				},
			},
			expected: true,
		},
		{
			testName: "has managedBy annotation with not koperator value",
			kafkaTopic: v1alpha1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{webhooks.TopicManagedByAnnotationKey: "other"},
				},
			},
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, isTopicManagedByKoperator(&testCase.kafkaTopic))
		})
	}
}
