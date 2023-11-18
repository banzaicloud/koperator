// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("kafka_util")
var apiVersion = sarama.V2_6_0_0
var clientId = "koperator"

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
	CreateUserACLs(v1alpha1.KafkaAccessType, v1alpha1.KafkaPatternType, string, string) error
	ListUserACLs() ([]sarama.ResourceAcls, error)
	DeleteUserACLs(string, v1alpha1.KafkaPatternType) error

	Brokers() map[int32]string
	DescribeCluster() ([]*sarama.Broker, int32, error)

	// AllOfflineReplicas returns the list of unique offline replica (broker) ids
	AllOfflineReplicas() ([]int32, error)

	// OutOfSyncReplicas returns the list of unique out of sync replica (broker) ids
	OutOfSyncReplicas() ([]int32, error)

	AlterPerBrokerConfig(int32, map[string]*string, bool) error
	DescribePerBrokerConfig(int32, []string) ([]*sarama.ConfigEntry, error)

	AlterClusterWideConfig(map[string]*string, bool) error
	DescribeClusterWideConfig() ([]sarama.ConfigEntry, error)

	TopicMetaToStatus(meta *sarama.TopicMetadata) *v1alpha1.KafkaTopicStatus

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
		err = errorfactory.New(errorfactory.BrokersUnreachable{}, err, fmt.Sprintf("could not connect to kafka brokers: %s", k.opts.BrokerURI))
		return err
	}

	if k.brokers, _, err = k.DescribeCluster(); err != nil {
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
func NewFromCluster(k8sclient client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, func(), error) {
	var client KafkaClient
	var err error
	opts, err := ClusterConfig(k8sclient, cluster)
	if err != nil {
		return nil, nil, err
	}
	client = New(opts)
	err = client.Open()
	close := func() {
		if err := client.Close(); err != nil {
			log.Error(err, "Error closing Kafka client")
		} else {
			log.Info("Kafka client closed cleanly")
		}
	}
	return client, close, err
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

func (k *kafkaClient) DescribeCluster() (brokers []*sarama.Broker, controllerID int32, err error) {
	brokers, controllerID, err = k.admin.DescribeCluster()
	return
}

func (k *kafkaClient) getSaramaConfig() (config *sarama.Config) {
	config = sarama.NewConfig()
	if k.opts.UseSSL {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = k.opts.TLSConfig
	}
	config.Version = apiVersion
	config.ClientID = clientId
	return
}
