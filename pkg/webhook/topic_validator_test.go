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

package webhook

import (
	"context"
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newMockCluster() *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-namespace"},
		Spec:       v1beta1.KafkaClusterSpec{},
	}
}

func newMockTopic() *v1alpha1.KafkaTopic {
	return &v1alpha1.KafkaTopic{
		ObjectMeta: metav1.ObjectMeta{Name: "test-topic", Namespace: "test-namespace"},
		Spec: v1alpha1.KafkaTopicSpec{
			Name:              "test-topic",
			Partitions:        2,
			ReplicationFactor: 1,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      "test-cluster",
				Namespace: "test-namespace",
			},
		},
	}
}

func TestValidateTopic(t *testing.T) {
	server := newMockServer()
	topic := newMockTopic()

	// Test non-existent kafka cluster
	res := server.validateKafkaTopic(topic)
	if res.Result.Reason != metav1.StatusReasonNotFound {
		t.Error("Expected not found cluster, got:", res.Result)
	}

	// test topic marked for deletion
	now := metav1.Now()
	topic.SetDeletionTimestamp(&now)
	if res = server.validateKafkaTopic(topic); !res.Allowed {
		t.Error("Expected allowed due to topic marked for deletion, got:", res.Result)
	}
	// remove deletion timestamp
	topic.SetDeletionTimestamp(nil)

	// test cluster marked for deletion
	cluster := newMockCluster()
	cluster.SetDeletionTimestamp(&now)
	server.client.Create(context.TODO(), cluster)
	if res = server.validateKafkaTopic(topic); !res.Allowed {
		t.Error("Expected allowed due to cluster marked for deletion, got:", res.Result)
	}

	// remove deletion timestamp from cluster
	cluster.SetDeletionTimestamp(nil)
	server.client.Update(context.TODO(), cluster)

	// test no rejection reasons
	if res = server.validateKafkaTopic(topic); !res.Allowed {
		t.Error("Expected allowed due to no issues, got:", res.Result)
	}

	// Rejection reasons

	// Replication factor larger than num brokers
	topic.Spec.ReplicationFactor = 2
	if res = server.validateKafkaTopic(topic); res.Allowed {
		t.Error("Expected not allowed due to replication factor larger than num brokers, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonBadRequest {
		t.Error("Expected bad request, got:", res.Result.Reason)
	}

	topic.Spec.ReplicationFactor = 1

	// Test overwrite attempt
	broker, _ := kafkaclient.NewMockFromCluster(server.client, cluster)
	broker.CreateTopic(&kafkaclient.CreateTopicOptions{Name: "test-topic", ReplicationFactor: 1, Partitions: 2})
	topic.Name = "test-topic"
	if res = server.validateKafkaTopic(topic); res.Allowed {
		t.Error("Expected not allowed due to existing topic with same name, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonAlreadyExists {
		t.Error("Expected not allowed for reason already exists, got:", res.Result)
	}

	// Add topic and test existing topic reason
	server.client.Create(context.TODO(), topic)

	// partition decrease attempt
	topic.Spec.Partitions = 1
	if res = server.validateKafkaTopic(topic); res.Allowed {
		t.Error("Expected not allowed due to partition decrease, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonInvalid {
		t.Error("Expected invalid status reason, got:", res.Result)
	}

	// replication factor change attempt
	topic.Spec.Partitions = 2
	topic.Spec.ReplicationFactor = 2
	if res = server.validateKafkaTopic(topic); res.Allowed {
		t.Error("Expected not allowed due to replication factor change, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonInvalid {
		t.Error("Expected invalid status reason, got:", res.Result)
	}
}
