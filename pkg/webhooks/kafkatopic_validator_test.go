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

package webhooks

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	//nolint:staticcheck

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/go-logr/logr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func newMockCluster() *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KafkaCluster",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-namespace"},
		Spec:       v1beta1.KafkaClusterSpec{},
	}
}

func newMockTopic() *v1alpha1.KafkaTopic {
	return &v1alpha1.KafkaTopic{
		ObjectMeta: metav1.ObjectMeta{Name: "test-topic", Namespace: "test-namespace"},
		Spec: v1alpha1.KafkaTopicSpec{
			Name:              "test-topic",
			Partitions:        0,
			ReplicationFactor: 0,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      "test-cluster",
				Namespace: "test-namespace",
			},
		},
	}
}

func newMockClients(cluster *v1beta1.KafkaCluster) (runtimeClient.WithWatch, kafkaclient.KafkaClient, func(client runtimeClient.Client, cluster *v1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error)) {
	scheme := runtime.NewScheme()

	v1beta1.AddToScheme(scheme)
	v1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	kafkaClient, _, _ := kafkaclient.NewMockFromCluster(client, cluster)
	returnMockedKafkaClient := func(client runtimeClient.Client, cluster *v1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error) {
		return kafkaClient, func() { kafkaClient.Close() }, nil
	}
	return client, kafkaClient, returnMockedKafkaClient
}

func TestValidateTopic(t *testing.T) {
	topic := newMockTopic()
	cluster := newMockCluster()
	client, kafkaClient, returnMockedKafkaClient := newMockClients(cluster)

	kafkaTopicValidator := KafkaTopicValidator{
		Client:              client,
		NewKafkaFromCluster: returnMockedKafkaClient,
	}

	// Test non-existent kafka cluster
	fieldErrorList, err := kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	// Test kafka topic with invalid partitions, and replicas, and not found cluster
	if len(fieldErrorList) != 3 {
		t.Errorf("there should be 3 invalid field, got %d", len(fieldErrorList))
	}
	if !strings.Contains(fieldErrorList.ToAggregate().Error(), "not exist") {
		t.Error("Expected not found cluster")
	}

	if err := kafkaTopicValidator.Client.Create(context.TODO(), cluster); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// set a valid partitions
	topic.Spec.Partitions = 2

	// Test kafka topic with invalid replication factor
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 1 {
		t.Error("Expected not allowed due to invalid replication factor, got allowed")
	}

	// set a valid replication factor
	topic.Spec.ReplicationFactor = 1

	// test topic marked for deletion
	now := metav1.Now()
	topic.SetDeletionTimestamp(&now)
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 0 {
		t.Error("Expected allowed due to topic marked for deletion")
	}
	// remove deletion timestamp
	topic.SetDeletionTimestamp(nil)

	// test cluster marked for deletion
	cluster.SetDeletionTimestamp(&now)

	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 0 {
		t.Error("Expected allowed due to cluster marked for deletion")
	}

	// remove deletion timestamp from cluster
	cluster.SetDeletionTimestamp(nil)
	if err := kafkaTopicValidator.Client.Update(context.TODO(), cluster); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// test no rejection reasons
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 0 {
		t.Error("Expected allowed due to no issues")
	}

	// Rejection reasons

	// Replication factor larger than num brokers
	topic.Spec.ReplicationFactor = 2
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 1 {
		t.Error("Expected not allowed due to replication factor larger than num brokers, got allowed")
	}

	topic.Spec.ReplicationFactor = 1

	// Test overwrite attempt
	err = kafkaClient.CreateTopic(&kafkaclient.CreateTopicOptions{Name: "test-topic", ReplicationFactor: 1, Partitions: 2})
	if err != nil {
		t.Error("creation of topic should have been successful")
	}
	topic.Name = "test-topic"
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 1 {
		t.Error("Expected not allowed due to existing topic with same name, got allowed")
	} else if !strings.Contains(fieldErrorList.ToAggregate().Error(), "topic already exists on kafka cluster") {
		t.Error("Expected not allowed for reason: already exists")
	}

	// Add topic and test existing topic reason
	if err := kafkaTopicValidator.Client.Create(context.TODO(), topic); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// partition decrease attempt
	topic.Spec.Partitions = 1
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 1 {
		t.Error("Expected not allowed due to partition decrease, got allowed")
	} else if !strings.Contains(fieldErrorList.ToAggregate().Error(), "kafka does not support decreasing partition count on an existing") {
		t.Error("Expected not allowed for reason: kafka does not support decreasing partition count")
	}

	// replication factor change attempt
	topic.Spec.Partitions = 2
	topic.Spec.ReplicationFactor = 2
	fieldErrorList, err = kafkaTopicValidator.validateKafkaTopic(context.Background(), topic, logr.Discard())
	if err != nil {
		t.Errorf("err should be nil, got: %s", err)
	}
	if len(fieldErrorList) != 1 {
		t.Error("Expected not allowed due to replication factor change, got allowed")
	} else if !strings.Contains(fieldErrorList.ToAggregate().Error(), "kafka does not support changing the replication factor") {
		t.Error("Expected not allowed for reason: kafka does not support changing the replication factor")
	}
}
