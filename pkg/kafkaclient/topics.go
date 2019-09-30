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
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
)

// CreateTopicOptions holds info about topic configuration
type CreateTopicOptions struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Config            map[string]*string
}

// ListTopics is used primarily for checking the existence of topics
func (k *kafkaClient) ListTopics() (map[string]sarama.TopicDetail, error) {
	return k.admin.ListTopics()
}

// GetTopic is used to check the existence, and retrieve details for a topic
func (k *kafkaClient) GetTopic(topicName string) (meta *sarama.TopicDetail, err error) {
	topics, err := k.ListTopics()
	if err != nil {
		return
	}
	found, exists := topics[topicName]
	if exists {
		meta = &found
	} else {
		meta = nil
	}
	return
}

// DescribeTopic is used during status syncs to retrieve topic metadata
func (k *kafkaClient) DescribeTopic(topic string) (meta *sarama.TopicMetadata, err error) {
	res, err := k.admin.DescribeTopics([]string{topic})
	if err != nil {
		return
	}
	if len(res) == 0 {
		return nil, errors.New("not found")
	}
	if res[0].Err != sarama.ErrNoError {
		err = res[0].Err
		return
	}
	meta = res[0]
	return
}

// CreateTopic creates a topic with the given options
func (k *kafkaClient) CreateTopic(opts *CreateTopicOptions) (err error) {
	err = k.admin.CreateTopic(opts.Name, &sarama.TopicDetail{
		NumPartitions:     opts.Partitions,
		ReplicationFactor: opts.ReplicationFactor,
		ConfigEntries:     opts.Config,
	}, false)
	if err != nil {
		err = errorfactory.New(errorfactory.CreateTopicError{}, err, "failed to create topic")
	}
	return
}

// DeleteTopic deletes a topic - when wait is specified, the method will not
// return until the topic doesn't appear in the cluster topic list.
func (k *kafkaClient) DeleteTopic(topicName string, wait bool) error {
	err := k.admin.DeleteTopic(topicName)
	if err != nil {
		return err
	}
	if wait {
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		select {
		case <-ticker.C:
			if topic, err := k.GetTopic(topicName); err != nil {
				return err
			} else if topic == nil {
				return nil
			} else {
				log.Info(fmt.Sprintf("Topic %s still going down for deletion", topicName))
			}
		}
	}
	return nil
}

// EnsurePartitionCount will check if a partition increase is requested and apply
// the changed.
func (k *kafkaClient) EnsurePartitionCount(topic string, desired int32) (changed bool, err error) {
	changed = false
	meta, err := k.admin.DescribeTopics([]string{topic})

	if err != nil {
		err = errorfactory.New(errorfactory.BrokersRequestError{}, err, "error describing topics")
		return
	}

	if len(meta) == 0 {
		err = errorfactory.New(errorfactory.TopicNotFound{}, err, fmt.Sprintf("could not find topic %s", topic))
		return
	}

	if desired != int32(len(meta[0].Partitions)) {
		// TODO (tinyzimmer): maybe let the user specify partition assignments
		assn := make([][]int32, 0)
		changed = true
		err = k.admin.CreatePartitions(topic, desired, assn, false)
	}
	return
}

// EnsureTopicConfig is an idempotent call to ensure topic configuration overrides
func (k *kafkaClient) EnsureTopicConfig(topic string, desiredConf map[string]*string) error {
	return k.admin.AlterConfig(sarama.TopicResource, topic, desiredConf, false)
}
