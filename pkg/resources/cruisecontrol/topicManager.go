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
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This function may be close to being able to be replaced with just submitting a CR
// to ourselves
func generateCCTopic(cluster *v1beta1.KafkaCluster, client client.Client, log logr.Logger) error {

	broker, err := kafkaclient.NewFromCluster(client, cluster)
	if err != nil {
		return err
	}
	defer broker.Close()

	return broker.CreateTopic(&kafkaclient.CreateTopicOptions{
		Name:              "__CruiseControlMetrics",
		Partitions:        12,
		ReplicationFactor: 3,
	})

}
