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
)

func TestListTopics(t *testing.T) {
	client := newOpenedMockClient()
	if topics, err := client.ListTopics(); err != nil {
		t.Error("Expected no error, got:", err)
	} else {
		for topic, _ := range topics {
			if topic != "test-topic" {
				t.Error("Expected test-topic from mock, got:", topic)
			}
		}
	}
}

func TestGetTopic(t *testing.T) {
	client := newOpenedMockClient()

	if topic, err := client.GetTopic("test-topic"); err != nil {
		t.Error("Expected nil, got:", err)
	} else if topic != nil {
		t.Error("Expected nil got:", topic)
	}

	client.admin.CreateTopic("test-topic", &sarama.TopicDetail{}, false)

	if topic, err := client.GetTopic("test-topic"); err != nil {
		t.Error("Expected to get test-topic without error, got:", err)
	} else if topic == nil {
		t.Error("Expected to get empty details, got nil")
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if _, err := client.GetTopic("test-topic"); err == nil {
		t.Error("Expected error on GetTopic, got nil")
	}
}

func TestDescribeTopic(t *testing.T) {
	client := newOpenedMockClient()

	if meta, err := client.DescribeTopic("test-topic"); err != nil {
		t.Error("Expected no error on DescribeTopic, got:", err)
	} else if meta.Name != "test-topic" {
		t.Error("Expected topic named 'test-topic', got:", meta.Name)
	}

	if _, err := client.DescribeTopic("with-error"); err == nil {
		t.Error("Expected error for bad topic, got nil")
	}

	if _, err := client.DescribeTopic("other"); err == nil {
		t.Error("Expected error got nil")
	} else if err.Error() != "not found" {
		t.Error("Expected error not found, got:", err.Error())
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if _, err := client.DescribeTopic("test-topic"); err == nil {
		t.Error("Expected error on DescribeTopic, got nil")
	}
}

func TestCreateTopic(t *testing.T) {
	client := newOpenedMockClient()
	if err := client.CreateTopic(&CreateTopicOptions{
		Name:              "new-topic",
		Partitions:        1,
		ReplicationFactor: 1,
	}); err != nil {
		t.Error("Expected no error, got:", err)
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if err := client.CreateTopic(&CreateTopicOptions{
		Name:              "new-topic",
		Partitions:        1,
		ReplicationFactor: 1,
	}); err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestDeleteTopic(t *testing.T) {
	client := newOpenedMockClient()
	if err := client.DeleteTopic("test-topic", false); err != nil {
		t.Error("Expected no error, got:", err)
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if err := client.DeleteTopic("test-topic", false); err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestEnsureTopicConfig(t *testing.T) {
	client := newOpenedMockClient()
	if err := client.EnsureTopicConfig("test-topic", map[string]*string{}); err != nil {
		t.Error("Expected no error, got:", err)
	}
	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if err := client.EnsureTopicConfig("test-topic", map[string]*string{}); err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestEnsurePartitionCount(t *testing.T) {
	client := newOpenedMockClient()
	if changed, err := client.EnsurePartitionCount("test-topic", 1); err != nil {
		t.Error("Expected no error, got:", err)
	} else if changed {
		t.Error("Expected no changed to be false, got true")
	}
	if changed, _ := client.EnsurePartitionCount("test-topic", 2); !changed {
		t.Error("Expected to attempt partition increase, got no change")
	}
	if _, err := client.EnsurePartitionCount("not-exists", 9000); err == nil {
		t.Error("Expected error for non-existant topic, got nil")
	}

	client.admin, _ = newMockClusterAdminFailOps([]string{}, sarama.NewConfig())
	if _, err := client.EnsurePartitionCount("test-topic", 1); err == nil {
		t.Error("Expected error, got nil")
	}
}
