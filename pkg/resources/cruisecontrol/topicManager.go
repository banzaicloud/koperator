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

package cruisecontrol

import (
	"emperror.dev/emperror"
	"emperror.dev/errors"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/kafkautil"
	"github.com/go-logr/logr"

	"github.com/Shopify/sarama"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func generateCCTopic(cluster *banzaicloudv1alpha1.KafkaCluster, client client.Client, log logr.Logger) error {

	broker, err := kafkautil.NewFromCluster(client, cluster)
	if err != nil {
		return emperror.Wrap(err, "Failed to connect to kafka cluster")
	}
	defer broker.Close()

	err = broker.CreateTopic(&kafkautil.CreateTopicOptions{
		Name:              "__CruiseControlMetrics",
		Partitions:        12,
		ReplicationFactor: 3,
	})

	var tError *sarama.TopicError

	if err != nil && !(errors.As(err, &tError) && tError.Err == sarama.ErrTopicAlreadyExists) {
		return emperror.Wrap(err, "Error while creating CC topic")
	}

	return nil
}
