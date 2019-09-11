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

package kafkautil

import (
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("kafka_util")
var apiVersion = sarama.V2_1_0_0

// KafkaClient is the exported interface for kafka operations
type KafkaClient interface {
	ListTopics() (map[string]sarama.TopicDetail, error)
	CreateTopic(*CreateTopicOptions) error
	EnsurePartitionCount(string, int32) (bool, error)
	EnsureTopicConfig(string, map[string]*string) error
	DeleteTopic(string) error
	GetTopic(string) (*sarama.TopicDetail, error)
	DescribeTopic(string) (*sarama.TopicMetadata, error)
	CreateUserACLs(v1alpha1.KafkaAccessType, string, string) error
	DeleteUserACLs(string) error

	ResolveBrokerID(int32) string
	DescribeCluster() ([]*sarama.Broker, error)
	GetCA() (string, string)

	OfflineReplicaCount() (int, error)

	Close() error
}

type kafkaClient struct {
	KafkaClient
	opts    *KafkaConfig
	admin   sarama.ClusterAdmin
	client  sarama.Client
	timeout time.Duration
	brokers []*sarama.Broker
}

func New(opts *KafkaConfig) (client KafkaClient, err error) {
	kclient := &kafkaClient{
		opts:    opts,
		timeout: time.Duration(opts.OperationTimeout) * time.Second,
	}

	var config *sarama.Config
	if config, err = kclient.getSaramaConfig(); err != nil {
		return
	}

	if kclient.admin, err = sarama.NewClusterAdmin([]string{opts.BrokerURI}, config); err != nil {
		return
	}

	if kclient.brokers, err = kclient.DescribeCluster(); err != nil {
		kclient.admin.Close()
		return
	}

	if kclient.client, err = sarama.NewClient([]string{opts.BrokerURI}, config); err != nil {
		kclient.client.Close()
		return
	}

	return kclient, nil
}

func (k *kafkaClient) Close() error {
	k.client.Close()
	return k.admin.Close()
}

// NewFromCluster is a convenience wrapper around New() and ClusterConfig()
func NewFromCluster(k8sclient client.Client, cluster *v1alpha1.KafkaCluster) (client KafkaClient, err error) {
	opts, err := ClusterConfig(k8sclient, cluster)
	if err != nil {
		return
	}
	return New(opts)
}

func (k *kafkaClient) ResolveBrokerID(ID int32) string {
	for _, broker := range k.brokers {
		if broker.ID() == ID {
			return broker.Addr()
		}
	}
	// fall back to leader ID
	return strconv.Itoa(int(ID))
}

func (k *kafkaClient) DescribeCluster() (brokers []*sarama.Broker, err error) {
	brokers, _, err = k.admin.DescribeCluster()
	return
}

func (k *kafkaClient) getSaramaConfig() (config *sarama.Config, err error) {
	config = sarama.NewConfig()
	if k.opts.UseSSL {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = k.opts.TLSConfig
	}
	config.Version = apiVersion
	return
}
