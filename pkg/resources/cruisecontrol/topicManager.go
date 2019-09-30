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
	"context"
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/webhook"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	cruiseControlTopicFormat            = "%s-cruise-control-topic"
	cruiseControlTopicName              = "__CruiseControlMetrics"
	cruiseControlTopicPartitions        = 12
	cruiseControlTopicReplicationFactor = 3
)

func newCruiseControlTopic(cluster *v1beta1.KafkaCluster) *v1alpha1.KafkaTopic {
	return &v1alpha1.KafkaTopic{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(cruiseControlTopicFormat, cluster.Name),
			map[string]string{
				"app":              "kafka",
				"clusterName":      cluster.Name,
				"clusterNamespace": cluster.Namespace,
			},
			cluster,
		),
		Spec: v1alpha1.KafkaTopicSpec{
			Name:              cruiseControlTopicName,
			Partitions:        cruiseControlTopicPartitions,
			ReplicationFactor: cruiseControlTopicReplicationFactor,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}
}

func generateCCTopic(cluster *v1beta1.KafkaCluster, client client.Client, log logr.Logger) error {
	existing := &v1alpha1.KafkaTopic{}
	topic := newCruiseControlTopic(cluster)
	if err := client.Get(context.TODO(), types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			// Attempt to create the topic
			if err := client.Create(context.TODO(), topic); err != nil {
				// If webhook was unable to connect to kafka - return not ready
				if webhook.IsAdmissionCantConnect(err) {
					return errorfactory.New(errorfactory.ResourceNotReady{}, err, "topic admission failed to connect to kafka cluster")
				}
				// If less than three brokers are available - return not ready
				if webhook.IsInvalidReplicationFactor(err) {
					return errorfactory.New(errorfactory.ResourceNotReady{}, err, "not enough brokers available (at least three needed) for CC topic")
				}
				return errorfactory.New(errorfactory.APIFailure{}, err, "could not create cruise control topic")
			}

		} else {
			// pass though any other api failure
			return errorfactory.New(errorfactory.APIFailure{}, err, "failed to lookup cruise control topic")
		}
	}
	return nil
}
