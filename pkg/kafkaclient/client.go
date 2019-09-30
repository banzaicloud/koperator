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
	"time"

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("kafka_util")
var apiVersion = sarama.V2_1_0_0

// KafkaClient is the exported interface for kafka operations
type KafkaClient interface {
	NumBrokers() int
	ListTopics() (map[string]sarama.TopicDetail, error)
	CreateTopic(*CreateTopicOptions) error
	EnsurePartitionCount(string, int32) (bool, error)
	EnsureTopicConfig(string, map[string]*string) error
	DeleteTopic(string, bool) error
	GetTopic(string) (*sarama.TopicDetail, error)
	DescribeTopic(string) (*sarama.TopicMetadata, error)
	CreateUserACLs(v1alpha1.KafkaAccessType, string, string) error
	DeleteUserACLs(string) error

	Brokers() map[int32]string
	DescribeCluster() ([]*sarama.Broker, error)

	OfflineReplicaCount() (int, error)
	AllReplicaInSync() (bool, error)

	AlterPerBrokerConfig(int32, map[string]*string) error
	DescribePerBrokerConfig(int32, []string) ([]*sarama.ConfigEntry, error)

	AlterClusterWideConfig(map[string]*string) error
	DescribeClusterWideConfig() ([]sarama.ConfigEntry, error)

	Open() error
	Close() error
}

type kafkaClient struct {
	KafkaClient
	opts    *KafkaConfig
	admin   sarama.ClusterAdmin
	client  sarama.Client
	timeout time.Duration
	brokers []*sarama.Broker

	// client funcs for mocking
	newClusterAdmin func([]string, *sarama.Config) (sarama.ClusterAdmin, error)
	newClient       func([]string, *sarama.Config) (sarama.Client, error)
}

func New(opts *KafkaConfig) KafkaClient {
	kclient := &kafkaClient{
		opts:    opts,
		timeout: time.Duration(opts.OperationTimeout) * time.Second,
	}
	kclient.newClusterAdmin = sarama.NewClusterAdmin
	kclient.newClient = sarama.NewClient
	return kclient
}

func (k *kafkaClient) Open() error {
	var err error
	config := k.getSaramaConfig()
	if k.admin, err = k.newClusterAdmin([]string{k.opts.BrokerURI}, config); err != nil {
		err = errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
		return err
	}

	if k.brokers, err = k.DescribeCluster(); err != nil {
		k.admin.Close()
		err = errorfactory.New(errorfactory.BrokersNotReady{}, err, "could not describe kafka cluster")
		return err
	}

	if k.client, err = k.newClient([]string{k.opts.BrokerURI}, config); err != nil {
		return err
	}

	return nil
}

func (k *kafkaClient) Close() error {
	k.client.Close()
	return k.admin.Close()
}

// NewFromCluster is a convenience wrapper around New() and ClusterConfig()
func NewFromCluster(k8sclient client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, error) {
	var client KafkaClient
	var err error
	opts, err := ClusterConfig(k8sclient, cluster)
	if err != nil {
		return nil, err
	}
	client = New(opts)
	err = client.Open()
	return client, err
}

func (k *kafkaClient) Brokers() map[int32]string {
	out := make(map[int32]string, 0)
	for _, broker := range k.brokers {
		out[broker.ID()] = broker.Addr()
	}
	return out
}

func (k *kafkaClient) NumBrokers() int {
	return len(k.brokers)
}

func (k *kafkaClient) GetBroker(id int32) (broker *sarama.Broker) {
	for _, x := range k.brokers {
		if x.ID() == id {
			broker = x
		}
	}
	return
}

func (k *kafkaClient) DescribeCluster() (brokers []*sarama.Broker, err error) {
	brokers, _, err = k.admin.DescribeCluster()
	return
}

func (k *kafkaClient) getSaramaConfig() (config *sarama.Config) {
	config = sarama.NewConfig()
	if k.opts.UseSSL {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = k.opts.TLSConfig
	}
	config.Version = apiVersion
	return
}
