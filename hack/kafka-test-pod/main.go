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

package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
)

const kafkaTopic = "test-topic"

const certFile = "/etc/secrets/certs/tls.crt"
const keyFile = "/etc/secrets/certs/tls.key"
const caFile = "/etc/secrets/certs/ca.crt"

const certEnvVar = "KAFKA_TLS_CERT"
const keyEnvVar = "KAFKA_TLS_KEY"
const caEnvVar = "KAFKA_TLS_CA"

var brokerAddrs = []string{"kafka-headless.kafka.svc.cluster.local:9092"}

func main() {

	// Create an SSL configuration for connecting to Kafka
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = getTLSConfig()

	// Trap SIGINT and SIGTERM to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Consume or produce depending on the value of the environment variable
	if os.Getenv("KAFKA_MODE") == "consumer" {
		consume(config, signals)
	} else if os.Getenv("KAFKA_MODE") == "producer" {
		produce(config, signals)
	} else {
		log.Fatal("Invalid test mode:", os.Getenv("KAFKA_MODE"))
	}

}

func getEnvTLSConfig() (config *tls.Config, err error) {
	clientCert, err := tls.X509KeyPair(
		[]byte(os.Getenv(certEnvVar)),
		[]byte(os.Getenv(keyEnvVar)),
	)
	if err != nil {
		return
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM([]byte(os.Getenv(caEnvVar)))
	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}, nil
}

func getTLSConfig() *tls.Config {
	// First see if we have an environment configuration
	if config, err := getEnvTLSConfig(); err == nil {
		return config
	}
	// Create a TLS configuration from our secret for connecting to Kafka
	clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatal(err)
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCert)
	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}
}

func consume(config *sarama.Config, signals chan os.Signal) {
	// Get a consumer
	consumer, err := sarama.NewConsumer(brokerAddrs, config)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	// Setup a channel for messages from our topic
	partitionConsumer, err := consumer.ConsumePartition(kafkaTopic, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(err)
	}
	defer partitionConsumer.Close()

	consumed := 0
ConsumerLoop:
	// Consume the topic
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			log.Printf("Consumed message offset %d - %s\n", msg.Offset, string(msg.Value))
			consumed++
		case <-signals:
			log.Println("Shutting down")
			break ConsumerLoop
		}
	}
}

func produce(config *sarama.Config, signals chan os.Signal) {

	// Get a broker connection
	broker := sarama.NewBroker(brokerAddrs[0])
	if err := broker.Open(config); err != nil {
		log.Fatal(err)
	}
	defer broker.Close()

	// Create a time ticker to trigger a message every 5 seconds
	ticker := time.NewTicker(time.Duration(5) * time.Second)

ProducerLoop:
	// Send a message everytime the ticker is triggered
	for {
		select {
		case <-ticker.C:
			log.Println("Sending message to topic")
			msg := &sarama.ProduceRequest{}
			msg.AddMessage(kafkaTopic, 0, &sarama.Message{
				Value: []byte("hello world"),
			})
			if _, err := broker.Produce(msg); err != nil {
				log.Fatal(err)
			}
		case <-signals:
			log.Println("Shutting down")
			break ProducerLoop
		}
	}

}
