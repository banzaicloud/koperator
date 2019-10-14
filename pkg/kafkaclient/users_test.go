// Copyright © 2019 Banzai Cloud
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

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
)

func TestCreateUserACLs(t *testing.T) {
	client := newOpenedMockClient()

	if err := client.CreateUserACLs(v1alpha1.KafkaAccessTypeRead, "test-user", "test-topic"); err != nil {
		t.Error("Expected no error, got:", err)
	}

	if err := client.CreateUserACLs(v1alpha1.KafkaAccessTypeWrite, "test-user", "test-topic"); err != nil {
		t.Error("Expected no error, got:", err)
	}

	if err := client.CreateUserACLs(v1alpha1.KafkaAccessType(""), "test-user", "test-topic"); err == nil {
		t.Error("Expected error, got nil")
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if err := client.CreateUserACLs(v1alpha1.KafkaAccessTypeRead, "test-user", "test-topic"); err == nil {
		t.Error("Expected error, got nil")
	}
	if err := client.CreateUserACLs(v1alpha1.KafkaAccessTypeWrite, "test-user", "test-topic"); err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestDeleteUserACLs(t *testing.T) {
	client := newOpenedMockClient()

	if err := client.DeleteUserACLs("test-user"); err != nil {
		t.Error("Expected no error, got:", err)
	}

	if err := client.DeleteUserACLs("with-error"); err == nil {
		t.Error("Expected error, got nil")
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if err := client.DeleteUserACLs("test-userr"); err == nil {
		t.Error("Expected error, got nil")
	}
}
