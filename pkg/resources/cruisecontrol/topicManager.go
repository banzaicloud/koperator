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

package cruisecontrol

import (
	"context"
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/webhooks"
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

const (
	ccMetricTopicAutoCreate             = "cruise.control.metrics.topic.auto.create"
	cruiseControlTopicFormat            = "%s-cruise-control-topic"
	cruiseControlTopicName              = "__CruiseControlMetrics"
	cruiseControlTopicPartitions        = 12
	cruiseControlTopicReplicationFactor = 3
)

func newCruiseControlTopic(cluster *v1beta1.KafkaCluster) *v1alpha1.KafkaTopic {
	var topicPartitions, topicReplicationFactor int32
	if cluster.Spec.CruiseControlConfig.TopicConfig != nil {
		topicPartitions = cluster.Spec.CruiseControlConfig.TopicConfig.Partitions
		topicReplicationFactor = cluster.Spec.CruiseControlConfig.TopicConfig.ReplicationFactor
	} else {
		topicPartitions = cruiseControlTopicPartitions
		topicReplicationFactor = cruiseControlTopicReplicationFactor
	}
	return &v1alpha1.KafkaTopic{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(cruiseControlTopicFormat, cluster.Name),
			map[string]string{
				v1beta1.AppLabelKey: "kafka",
				"clusterName":       cluster.Name,
				"clusterNamespace":  cluster.Namespace,
			},
			cluster,
		),
		Spec: v1alpha1.KafkaTopicSpec{
			Name:              cruiseControlTopicName,
			Partitions:        topicPartitions,
			ReplicationFactor: topicReplicationFactor,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}
}

func generateCCTopic(cluster *v1beta1.KafkaCluster, client client.Client, log logr.Logger) error {
	readOnlyConfigProperties, err := properties.NewFromString(cluster.Spec.ReadOnlyConfig)
	if err != nil {
		return errors.WrapIf(err, "could not parse broker config")
	}

	// for compatibility reasons the only case when we let CC to create its own kafka topics is
	// when we enable the creation explicitly
	if autoCreateProperty, present := readOnlyConfigProperties.Get(ccMetricTopicAutoCreate); present {
		if autoCreate, err := autoCreateProperty.Bool(); err != nil {
			return err
		} else if autoCreate {
			log.Info("CruiseControl topic has been created by CruiseControl")
			return nil
		}
	}

	existing := &v1alpha1.KafkaTopic{}
	topic := newCruiseControlTopic(cluster)
	if err := client.Get(context.TODO(), types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			// Attempt to create the topic
			if err := client.Create(context.TODO(), topic); err != nil {
				// If webhook was unable to connect to kafka - return not ready
				if webhooks.IsAdmissionCantConnect(err) {
					return errorfactory.New(errorfactory.ResourceNotReady{}, err, "topic admission failed to connect to kafka cluster")
				}
				// If less than the required brokers are available - return not ready
				if webhooks.IsAdmissionInvalidReplicationFactor(err) {
					return errorfactory.New(errorfactory.ResourceNotReady{}, err, fmt.Sprintf("not enough brokers available (at least %d needed) for CC topic", topic.Spec.ReplicationFactor))
				}
				return errorfactory.New(errorfactory.APIFailure{}, err, "could not create cruise control topic")
			}
		} else {
			// pass though any other api failure
			return errorfactory.New(errorfactory.APIFailure{}, err, "failed to lookup cruise control topic")
		}
	}

	log.Info("CruiseControl topic has been created by Operator")
	return nil
}
