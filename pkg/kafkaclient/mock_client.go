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
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClusterAdmin struct {
	sarama.ClusterAdmin
	sarama.Client
	sync.Mutex
	failOps    bool
	mockTopics map[string]sarama.TopicDetail
	mockACLs   map[sarama.Resource]*sarama.ResourceAcls
}

func NewMockFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, func(), error) {
	cl := newOpenedMockClient()
	return cl, func() { cl.Close() }, nil
}

func newEmptyMockClusterAdmin(failOps bool) *mockClusterAdmin {
	return &mockClusterAdmin{
		mockTopics: make(map[string]sarama.TopicDetail, 0),
		mockACLs:   make(map[sarama.Resource]*sarama.ResourceAcls, 0),
		failOps:    failOps,
	}
}

func newMockClusterAdmin([]string, *sarama.Config) (sarama.ClusterAdmin, error) {
	return newEmptyMockClusterAdmin(false), nil
}

func newMockKafkaClient([]string, *sarama.Config) (sarama.Client, error) {
	return newEmptyMockClusterAdmin(false), nil
}

func newMockClusterAdminError([]string, *sarama.Config) (sarama.ClusterAdmin, error) {
	return newEmptyMockClusterAdmin(false), errors.New("bad client")
}

func newMockClusterAdminFailOps([]string, *sarama.Config) (sarama.ClusterAdmin, error) {
	return newEmptyMockClusterAdmin(true), nil
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
	m.Lock()
	defer m.Unlock()

	if m.failOps {
		return map[string]sarama.TopicDetail{}, errors.New("bad list topics")
	}
	return shallowCopy(m.mockTopics), nil
}

func (m *mockClusterAdmin) DescribeTopics(topics []string) ([]*sarama.TopicMetadata, error) {
	if m.failOps {
		return []*sarama.TopicMetadata{}, errors.New("bad describe topics")
	}
	switch topics[0] {
	case "test-topic", "already-created-topic":
		return []*sarama.TopicMetadata{
			&sarama.TopicMetadata{
				Name:       topics[0],
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
	m.Lock()
	defer m.Unlock()

	if m.failOps {
		return errors.New("bad create topic")
	}
	if _, ok := m.mockTopics[name]; ok {
		return errors.New("already exists")
	}
	m.mockTopics[name] = *detail
	return nil
}

func (m *mockClusterAdmin) DeleteTopic(name string) error {
	m.Lock()
	defer m.Unlock()

	if m.failOps {
		return errors.New("bad delete topic")
	}
	if _, ok := m.mockTopics[name]; ok {
		delete(m.mockTopics, name)
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
	m.Lock()
	defer m.Unlock()

	if m.failOps {
		return errors.New("bad create acl")
	}
	var resourceAcls *sarama.ResourceAcls
	if val, ok := m.mockACLs[resource]; ok {
		resourceAcls = val
	} else {
		resourceAcls = &sarama.ResourceAcls{
			Resource: resource,
		}
	}
	for _, v := range resourceAcls.Acls {
		if *v == acl {
			return nil
		}
	}
	resourceAcls.Acls = append(resourceAcls.Acls, &acl)
	m.mockACLs[resource] = resourceAcls
	return nil
}

func (m *mockClusterAdmin) ListAcls(filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	m.Lock()
	defer m.Unlock()

	acls := make([]sarama.ResourceAcls, len(m.mockACLs))
	for _, acl := range m.mockACLs {
		acls = append(acls, *acl)
	}
	return acls, nil
}

func (m *mockClusterAdmin) DeleteACL(filter sarama.AclFilter, validateOnly bool) ([]sarama.MatchingAcl, error) {
	m.Lock()
	defer m.Unlock()

	if m.failOps {
		return []sarama.MatchingAcl{}, errors.New("bad create acl")
	}
	switch *filter.Principal {
	case "test-user":
		return []sarama.MatchingAcl{sarama.MatchingAcl{}}, nil
	case "with-error":
		return []sarama.MatchingAcl{sarama.MatchingAcl{Err: sarama.ErrUnknown}}, nil
	default:
		// for mock it's enough to erase the whole map
		m.mockACLs = make(map[sarama.Resource]*sarama.ResourceAcls, 0)
		return []sarama.MatchingAcl{sarama.MatchingAcl{}}, nil
	}
}

func (m *mockClusterAdmin) DescribeConfig(resource sarama.ConfigResource) ([]sarama.ConfigEntry, error) {
	return []sarama.ConfigEntry{}, nil
}

func (m *mockClusterAdmin) Controller() (*sarama.Broker, error) {
	return &sarama.Broker{}, nil
}

func shallowCopy(original map[string]sarama.TopicDetail) map[string]sarama.TopicDetail {
	returnMap := make(map[string]sarama.TopicDetail, len(original))
	for k, v := range original {
		returnMap[k] = v
	}
	return returnMap
}
