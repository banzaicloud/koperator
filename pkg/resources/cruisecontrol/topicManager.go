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
	"fmt"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/goph/emperror"
	kafkaGo "github.com/segmentio/kafka-go"
)

func generateCCTopic(cluster *banzaicloudv1alpha1.KafkaCluster) error {

	conn, err := kafkaGo.Dial("tcp",
		fmt.Sprintf("%s.%s.svc.cluster.local:%d", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort))
	if err != nil {
		return emperror.Wrap(err, "could not create topic for CC because kafka is unavailable")
	}
	tConfig := kafkaGo.TopicConfig{
		Topic:             "__CruiseControlMetrics",
		NumPartitions:     12,
		ReplicationFactor: 3,
	}

	err = conn.CreateTopics(tConfig)
	if err != nil {
		return emperror.Wrap(err, "could not create topic for CC")
	}
	return nil
}
