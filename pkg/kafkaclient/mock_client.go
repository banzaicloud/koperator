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
	"errors"
	"time"

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var mockTopics = make(map[string]sarama.TopicDetail, 0)

type mockClusterAdmin struct {
	sarama.ClusterAdmin
	sarama.Client
	failOps bool
}

func NewMockFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, error) {
	return newOpenedMockClient(), nil
}

func newMockClusterAdmin([]string, *sarama.Config) (sarama.ClusterAdmin, error) {
	return &mockClusterAdmin{}, nil
}

func newMockKafkaClient([]string, *sarama.Config) (sarama.Client, error) {
	return &mockClusterAdmin{}, nil
}

func newMockClusterAdminError([]string, *sarama.Config) (sarama.ClusterAdmin, error) {
	return &mockClusterAdmin{}, errors.New("bad client")
}

func newMockClusterAdminFailOps([]string, *sarama.Config) (sarama.ClusterAdmin, error) {
	return &mockClusterAdmin{failOps: true}, nil
}

func newMockOpts() *KafkaConfig {
	return &KafkaConfig{
		OperationTimeout: kafkaDefaultTimeout,
	}
}

func newMockClient() *kafkaClient {
	return &kafkaClient{
		opts:            newMockOpts(),
		timeout:         time.Duration(kafkaDefaultTimeout) * time.Second,
		newClusterAdmin: newMockClusterAdmin,
		newClient:       newMockKafkaClient,
	}
}

func newOpenedMockClient() *kafkaClient {
	client := newMockClient()
	client.Open()
	return client
}

func (m *mockClusterAdmin) Close() error { return nil }

func (m *mockClusterAdmin) DescribeCluster() ([]*sarama.Broker, int32, error) {
	if m.failOps {
		return []*sarama.Broker{}, 0, errors.New("bad describe cluster")
	}
	return []*sarama.Broker{&sarama.Broker{}}, 0, nil
}

func (m *mockClusterAdmin) ListTopics() (map[string]sarama.TopicDetail, error) {
	if m.failOps {
		return map[string]sarama.TopicDetail{}, errors.New("bad list topics")
	}
	return mockTopics, nil
}

func (m *mockClusterAdmin) DescribeTopics(topics []string) ([]*sarama.TopicMetadata, error) {
	if m.failOps {
		return []*sarama.TopicMetadata{}, errors.New("bad describe topics")
	}
	switch topics[0] {
	case "test-topic":
		return []*sarama.TopicMetadata{
			&sarama.TopicMetadata{
				Name:       "test-topic",
				Partitions: []*sarama.PartitionMetadata{&sarama.PartitionMetadata{}},
				Err:        sarama.ErrNoError,
			},
		}, nil
	case "not-exists":
		return []*sarama.TopicMetadata{}, nil
	case "with-error":
		return []*sarama.TopicMetadata{
			&sarama.TopicMetadata{
				Name:       "with-error",
				Partitions: []*sarama.PartitionMetadata{&sarama.PartitionMetadata{}},
				Err:        sarama.ErrUnknown,
			},
		}, nil
	default:
		return []*sarama.TopicMetadata{}, nil
	}
}

func (m *mockClusterAdmin) CreateTopic(name string, detail *sarama.TopicDetail, validateOnly bool) error {
	if m.failOps {
		return errors.New("bad create topic")
	}
	if _, ok := mockTopics[name]; ok {
		return errors.New("already exists")
	}
	mockTopics[name] = *detail
	return nil
}

func (m *mockClusterAdmin) DeleteTopic(name string) error {
	if m.failOps {
		return errors.New("bad delete topic")
	}
	if _, ok := mockTopics[name]; ok {
		delete(mockTopics, name)
		return nil
	} else {
		return errors.New("does not exist")
	}
}

func (m *mockClusterAdmin) AlterConfig(resource sarama.ConfigResourceType, topic string, conf map[string]*string, validateOnly bool) error {
	if m.failOps {
		return errors.New("bad alter config")
	}
	return nil
}

func (m *mockClusterAdmin) CreatePartitions(topic string, count int32, assn [][]int32, validateOnly bool) error {
	return nil
}

func (m *mockClusterAdmin) CreateACL(resource sarama.Resource, acl sarama.Acl) error {
	if m.failOps {
		return errors.New("bad create acl")
	}
	return nil
}

func (m *mockClusterAdmin) DeleteACL(filter sarama.AclFilter, validateOnly bool) ([]sarama.MatchingAcl, error) {
	if m.failOps {
		return []sarama.MatchingAcl{}, errors.New("bad create acl")
	}
	switch *filter.Principal {
	case "test-user":
		return []sarama.MatchingAcl{sarama.MatchingAcl{}}, nil
	case "with-error":
		return []sarama.MatchingAcl{sarama.MatchingAcl{Err: sarama.ErrUnknown}}, nil
	default:
		return []sarama.MatchingAcl{sarama.MatchingAcl{}}, nil
	}
}
