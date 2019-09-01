package kafkautil

import (
	"errors"
	"fmt"

	"github.com/Shopify/sarama"
)

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
	}
	return
}

// DescribeTopic is used during status syncs to retrieve topic metadata
func (k *kafkaClient) DescribeTopic(topic string) (meta *sarama.TopicMetadata, err error) {
	res, err := k.admin.DescribeTopics([]string{topic})
	if err != nil {
		return
	}
	if res[0].Err != sarama.ErrNoError {
		err = res[0].Err
		return
	}
	meta = res[0]
	return
}

// CreateTopic creates a topic with the given options
func (k *kafkaClient) CreateTopic(opts *CreateTopicOptions) error {
	return k.admin.CreateTopic(opts.Name, &sarama.TopicDetail{
		NumPartitions:     opts.Partitions,
		ReplicationFactor: opts.ReplicationFactor,
		ConfigEntries:     opts.Config,
	}, false)
}

// DeleteTopic deletes a topic
func (k *kafkaClient) DeleteTopic(topic string) error {
	return k.admin.DeleteTopic(topic)
}

// EnsurePartitionCount will check if a partition increase is requested and apply
// the changed.
func (k *kafkaClient) EnsurePartitionCount(topic string, desired int32) (changed bool, err error) {
	changed = false
	meta, err := k.admin.DescribeTopics([]string{topic})

	if err != nil {
		return
	}
	if len(meta) == 0 {
		err = errors.New(fmt.Sprint("No topic", topic, "found"))
		return
	}

	if desired != int32(len(meta[0].Partitions)) {
		// TODO: maybe let the user specify partition assignments
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
