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

package e2e

import (
	"context"
	"crypto/tls"
	"fmt"

	"emperror.dev/errors"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	"github.com/twmb/franz-go/pkg/kgo"

	koperatorUtil "github.com/banzaicloud/koperator/pkg/util"
)

// producingMessagesExternally produces messages based on the parameters into the kafka cluster synchronously.
func producingMessagesExternally(externalKafkaAddresses []string, topicName string, messages []string, clientOptions ...kgo.Opt) error { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	By(fmt.Sprintf("Producing messages: '%s' to externalKafkaAddresses: '%s' topicName: '%s'", messages, externalKafkaAddresses, topicName))

	clientOptions = append(clientOptions, kgo.SeedBrokers(externalKafkaAddresses...))

	cl, err := kgo.NewClient(
		clientOptions...,
	)
	if err != nil {
		return err
	}
	defer cl.Close()

	ctx := context.Background()
	ctxProducerTimeout, cancelCtxProducerTimeout := context.WithTimeout(ctx, externalProducerTimeout)
	defer cancelCtxProducerTimeout()

	for i := range messages {
		record := &kgo.Record{Topic: topicName, Value: []byte(messages[i])}
		// Exits at the first error occurrence
		err = cl.ProduceSync(ctxProducerTimeout, record).FirstErr()
		if err != nil {
			return err
		}
	}
	return nil
}

// consumingMessagesExternally consuming messages based on parameters from Kafka cluster.
// It returns messages as string slice.
func consumingMessagesExternally(externalKafkaAddresses []string, topicName string, clientOptions ...kgo.Opt) ([]string, error) { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	By(fmt.Sprintf("Consuming messages from externalKafkaAddresses: '%s' topicName: '%s'", externalKafkaAddresses, topicName))

	clientOptions = append(clientOptions, kgo.SeedBrokers(externalKafkaAddresses...), kgo.ConsumeTopics(topicName))

	cl, err := kgo.NewClient(
		clientOptions...,
	)
	if err != nil {
		return nil, err
	}

	defer cl.Close()

	ctx := context.Background()
	ctxConsumerTimeout, cancelCtxConsumerTimeout := context.WithTimeout(ctx, externalProducerTimeout)
	defer cancelCtxConsumerTimeout()

	// Send fetch request for the Kafka cluster
	fetches := cl.PollFetches(ctxConsumerTimeout)
	errs := fetches.Errors()

	if len(errs) > 0 {
		var combinedError error
		for _, err := range errs {
			combinedError = errors.Append(combinedError,
				fmt.Errorf("error: '%w', when consuming message from topic: '%s' partition: '%d'", err.Err, err.Topic, err.Partition))
		}
		return nil, combinedError
	}

	iter := fetches.RecordIter()
	var messages []string
	for !iter.Done() {
		consumedRecord := iter.Next()
		messages = append(messages, string(consumedRecord.Value))
	}

	return messages, nil
}

func getTLSConfigFromSecret(kubectlOptions k8s.KubectlOptions, secretName string) (*tls.Config, error) { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	By(fmt.Sprintf("Getting TLS config from secret name: '%s' namespace: '%s'", secretName, kubectlOptions.Namespace))
	tlsSecret, err := k8s.GetSecretE(GinkgoT(), &kubectlOptions, secretName)
	if err != nil {
		return nil, fmt.Errorf("could not get TLS secret for kafka client: %w", err)
	}
	tlsConfig, err := koperatorUtil.CreateTLSConfigFromSecret(tlsSecret)
	if err != nil {
		return nil, fmt.Errorf("could not create TLS config from secret: %w", err)
	}
	return tlsConfig, nil
}
