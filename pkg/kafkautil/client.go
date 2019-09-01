package kafkautil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("kafka_util")
var apiVersion = sarama.V2_3_0_0

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

	Close() error
}

type kafkaClient struct {
	KafkaClient
	opts    *KafkaConfig
	admin   sarama.ClusterAdmin
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
		return
	}

	return kclient, nil
}

func (k *kafkaClient) Close() error {
	return k.admin.Close()
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
		var tlsConfig *tls.Config
		config.Net.TLS.Enable = true
		if k.opts.TLSConfig != nil {
			tlsConfig = k.opts.TLSConfig
		} else {
			tlsConfig, err = getTLSConfig(k.opts.SSLKeyFile, k.opts.SSLCertFile, k.opts.SSLCAFile)
			if err != nil {
				return config, err
			}
		}
		if k.opts.SSLInsecureSkipVerify {
			tlsConfig.InsecureSkipVerify = true
		}
		config.Net.TLS.Config = tlsConfig
	}
	config.Version = apiVersion
	return
}

func getTLSConfig(keypath, crtpath, capath string) (conf *tls.Config, err error) {
	conf = &tls.Config{}

	cert, err := tls.LoadX509KeyPair(crtpath, keypath)
	if err != nil {
		return
	}
	conf.Certificates = []tls.Certificate{cert}

	certPool := x509.NewCertPool()

	cabytes, err := ioutil.ReadFile(capath)
	if err != nil {
		return
	}

	if ok := certPool.AppendCertsFromPEM(cabytes); !ok {
		err = errors.New("Failed to load CA")
		return
	}

	conf.ClientCAs = certPool
	conf.RootCAs = certPool

	return
}
