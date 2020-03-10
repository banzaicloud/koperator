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
	"fmt"
	"strconv"

	"emperror.dev/errors"
	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
)

func (k *kafkaClient) AlterClusterWideConfig(configChange map[string]*string) error {
	return k.admin.AlterConfig(sarama.BrokerResource, "", configChange, false)
}

func (k *kafkaClient) DescribeClusterWideConfig() ([]sarama.ConfigEntry, error) {
	return k.admin.DescribeConfig(sarama.ConfigResource{Type: sarama.BrokerResource, Name: "", ConfigNames: []string{}})
}

func (k *kafkaClient) AlterPerBrokerConfig(brokerId int32, configChange map[string]*string) error {
	broker := k.GetBroker(brokerId)
	if broker == nil {
		return errorfactory.New(errorfactory.BrokersNotReady{}, errors.New("brokerNotReady"), fmt.Sprintf("could not get %d broker", brokerId))
	}
	err := broker.Open(k.getSaramaConfig())
	if err != nil {
		return errors.WrapIf(err, "could not open connection to broker")
	}
	defer broker.Close()

	_, err = broker.AlterConfigs(&sarama.AlterConfigsRequest{
		ValidateOnly: false,
		Resources: []*sarama.AlterConfigsResource{
			{
				Type:          sarama.BrokerResource,
				Name:          strconv.Itoa(int(brokerId)),
				ConfigEntries: configChange,
			},
		},
	})
	return err
}

func (k *kafkaClient) DescribePerBrokerConfig(brokerId int32, config []string) ([]*sarama.ConfigEntry, error) {
	broker := k.GetBroker(brokerId)
	if broker == nil {
		return nil, errorfactory.New(errorfactory.BrokersNotReady{}, errors.New("brokerNotReady"), fmt.Sprintf("could not get %d broker", brokerId))
	}
	err := broker.Open(k.getSaramaConfig())
	if err != nil {
		return nil, errors.WrapIf(err, "could not open connection to broker")
	}
	defer broker.Close()

	currentConfig, err := broker.DescribeConfigs(&sarama.DescribeConfigsRequest{
		Resources: []*sarama.ConfigResource{{Type: sarama.BrokerResource, Name: strconv.Itoa(int(brokerId)), ConfigNames: config}},
	})
	if err != nil {
		return nil, err
	}
	return currentConfig.Resources[0].Configs, nil
}
