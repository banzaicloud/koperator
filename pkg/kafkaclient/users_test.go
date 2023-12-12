// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

package kafkaclient

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/banzaicloud/koperator/api/v1alpha1"
)

func TestCreateUserACLs(t *testing.T) {
	client := newOpenedMockClient()

	validAccessTypes := []v1alpha1.KafkaAccessType{
		"read",
		"write"}
	invalidAccessTypes := []v1alpha1.KafkaAccessType{
		"",
		"helloWorld"}
	allAccessTypes := append(validAccessTypes, invalidAccessTypes...)

	validPatternTypes := []v1alpha1.KafkaPatternType{
		"any",
		"literal",
		"match",
		"prefixed",
		""}
	invalidPatternTypes := []v1alpha1.KafkaPatternType{
		"helloWorld"}
	allPatternTypes := append(validPatternTypes, invalidPatternTypes...)

	// Test all valid combinations of accessType and patternType
	for _, accessType := range validAccessTypes {
		for _, patternType := range validPatternTypes {
			if err := client.CreateUserACLs(accessType, patternType, "test-user", "test-topic"); err != nil {
				t.Error("Expected no error, got:", err)
			}
		}
	}
	// Test invalid accessTypes against all patternTypes
	for _, accessType := range invalidAccessTypes {
		for _, patternType := range allPatternTypes {
			if err := client.CreateUserACLs(accessType, patternType, "test-user", "test-topic"); err == nil {
				t.Error("Expected error, got nil")
			}
		}
	}
	// Test invalid patternTypes against all accessTypes
	for _, patternType := range invalidPatternTypes {
		for _, accessType := range allAccessTypes {
			if err := client.CreateUserACLs(accessType, patternType, "test-user", "test-topic"); err == nil {
				t.Error("Expected error, got nil")
			}
		}
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	// Test all combinations of accessType and patternType
	for _, accessType := range allAccessTypes {
		for _, patternType := range allPatternTypes {
			if err := client.CreateUserACLs(accessType, patternType, "test-user", "test-topic"); err == nil {
				t.Error("Expected error, got nil")
			}
		}
	}
}

func TestDeleteUserACLs(t *testing.T) {
	client := newOpenedMockClient()

	validPatternTypes := []v1alpha1.KafkaPatternType{
		"any",
		"literal",
		"match",
		"prefixed",
		""}

	for _, patternType := range validPatternTypes {
		if err := client.DeleteUserACLs("test-user", patternType); err != nil {
			t.Error("Expected no error, got:", err)
		}

		if err := client.DeleteUserACLs("with-error", patternType); err == nil {
			t.Error("Expected error, got nil")
		}
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if err := client.DeleteUserACLs("test-userr", ""); err == nil {
		t.Error("Expected error, got nil")
	}
}
