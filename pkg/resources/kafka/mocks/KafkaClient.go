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

package mocks

import (
	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/mock"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
)

// KafkaClient is a Testify mock for Kafka client
type KafkaClient struct {
	mock.Mock
}

func (k *KafkaClient) NumBrokers() int {
	args := k.Called()
	return args.Int(0)
}

func (k *KafkaClient) ListTopics() (map[string]sarama.TopicDetail, error) {
	args := k.Called()
	return args.Get(0).(map[string]sarama.TopicDetail), args.Error(1)
}

func (k *KafkaClient) CreateTopic(createTopicOptions *kafkaclient.CreateTopicOptions) error {
	args := k.Called(createTopicOptions)
	return args.Error(0)
}

func (k *KafkaClient) EnsurePartitionCount(topic string, partitionCount int32) (bool, error) {
	args := k.Called(topic, partitionCount)
	return args.Bool(0), args.Error(1)
}

func (k *KafkaClient) EnsureTopicConfig(topic string, config map[string]*string) error {
	args := k.Called(topic, config)
	return args.Error(0)
}

func (k *KafkaClient) DeleteTopic(topic string, force bool) error {
	args := k.Called(topic, force)
	return args.Error(0)
}

func (k *KafkaClient) GetTopic(topic string) (*sarama.TopicDetail, error) {
	args := k.Called(topic)
	return args.Get(0).(*sarama.TopicDetail), args.Error(1)
}

func (k *KafkaClient) DescribeTopic(topic string) (*sarama.TopicMetadata, error) {
	args := k.Called(topic)
	return args.Get(0).(*sarama.TopicMetadata), args.Error(1)
}

func (k *KafkaClient) CreateUserACLs(accessType v1alpha1.KafkaAccessType, patternType v1alpha1.KafkaPatternType, dn string, topic string) (err error) {
	args := k.Called(accessType, patternType, dn, topic)
	return args.Error(0)
}

func (k *KafkaClient) ListUserACLs() (acls []sarama.ResourceAcls, err error) {
	args := k.Called()
	return args.Get(0).([]sarama.ResourceAcls), args.Error(1)
}

func (k *KafkaClient) DeleteUserACLs(dn string) (err error) {
	args := k.Called(dn)
	return args.Error(0)
}

func (k *KafkaClient) Brokers() map[int32]string {
	args := k.Called()
	return args.Get(0).(map[int32]string)
}

func (k *KafkaClient) DescribeCluster() ([]*sarama.Broker, int32, error) {
	args := k.Called()
	return args.Get(0).([]*sarama.Broker), args.Get(1).(int32), args.Error(2)
}

func (k *KafkaClient) AllOfflineReplicas() ([]int32, error) {
	args := k.Called()
	return args.Get(0).([]int32), args.Error(1)
}

func (k *KafkaClient) OutOfSyncReplicas() ([]int32, error) {
	args := k.Called()
	return args.Get(0).([]int32), args.Error(1)
}

func (k *KafkaClient) AlterPerBrokerConfig(brokerId int32, config map[string]*string, validateOnly bool) error {
	args := k.Called(brokerId, config, validateOnly)
	return args.Error(0)
}

func (k *KafkaClient) DescribePerBrokerConfig(brokerId int32, configNames []string) ([]*sarama.ConfigEntry, error) {
	args := k.Called(brokerId, configNames)
	return args.Get(0).([]*sarama.ConfigEntry), args.Error(1)
}

func (k *KafkaClient) AlterClusterWideConfig(config map[string]*string, validateOnly bool) error {
	args := k.Called(config, validateOnly)
	return args.Error(0)
}

func (k *KafkaClient) DescribeClusterWideConfig() ([]sarama.ConfigEntry, error) {
	args := k.Called()
	return args.Get(0).([]sarama.ConfigEntry), args.Error(1)
}

func (k *KafkaClient) TopicMetaToStatus(topicMeta *sarama.TopicMetadata) *v1alpha1.KafkaTopicStatus {
	args := k.Called(topicMeta)
	return args.Get(0).(*v1alpha1.KafkaTopicStatus)
}

func (k *KafkaClient) Open() error {
	args := k.Called()
	return args.Error(0)
}

func (k *KafkaClient) Close() error {
	args := k.Called()
	return args.Error(0)
}
