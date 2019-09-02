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

package webhook

import (
	"context"
	"fmt"

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/kafkautil"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (s *webhookServer) validateKafkaTopic(topic v1alpha1.KafkaTopic) (res *admissionv1beta1.AdmissionResponse) {
	log.Info(fmt.Sprintf("Doing pre-admission validation of kafka topic %s", topic.Spec.Name))

	// Get the referenced kafkacluster
	if topic.Spec.ClusterRef.Namespace == "" {
		topic.Spec.ClusterRef.Namespace = topic.Namespace
	}

	var cluster *v1alpha1.KafkaCluster
	var err error
	if cluster, err = k8sutil.LookupKafkaCluster(s.client, topic.Spec.ClusterRef); err != nil {
		log.Error(err, "Failed to lookup referenced cluster")
		return notAllowed(fmt.Sprintf("KafkaCluster '%s' in the namespace '%s' does not exist", topic.Spec.ClusterRef.Name, topic.Spec.ClusterRef.Namespace))
	}

	conf, err := kafkautil.ClusterConfig(s.client, cluster)
	if err != nil {
		log.Error(err, "Failed to get a kafka config for the cluster")
		return notAllowed(fmt.Sprintf("Failed to retrieve kafka configuration for cluster: %s", topic.Spec.ClusterRef.Name))
	}

	broker, err := kafkautil.New(conf)
	if err != nil {
		log.Error(err, "Failed to connect to kafka cluster")
		return notAllowed(fmt.Sprintf("Failed to connect to kafka cluster: %s", topic.Spec.ClusterRef.Name))
	}

	existing, err := broker.GetTopic(topic.Spec.Name)
	if err != nil {
		log.Error(err, "Failed to list topics")
		return notAllowed(fmt.Sprintf("Failed to list topics for kafka cluster: %s", topic.Spec.ClusterRef.Name))
	}

	// The topic exists
	if existing != nil {
		// Check if this is the correct CR for this topic
		topicCR := &v1alpha1.KafkaTopic{}
		if err := s.client.Get(context.TODO(), types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, topicCR); err != nil {
			if apierrors.IsNotFound(err) {
				// User is trying to overwrite an existing topic
				return notAllowed(fmt.Sprintf("Topic '%s' already exists on kafka cluster '%s'", topic.Spec.Name, topic.Spec.ClusterRef.Name))
			} else {
				return notAllowed("API failure while validating topic, please try again")
			}
		}

		// make sure the user isn't trying to decrease partition count
		if existing.NumPartitions > topic.Spec.Partitions {
			log.Info(fmt.Sprintf("Spec is requesting partition decrease from %v to %v, rejecting", existing.NumPartitions, topic.Spec.Partitions))
			return notAllowed("Kafka does not support decreasing partition count on an existing topic")
		}

		// check if the user is trying to change the replication factor
		if existing.ReplicationFactor != int16(topic.Spec.ReplicationFactor) {
			log.Info(fmt.Sprintf("Spec is requesting replication factor change from %v to %v, rejecting", existing.ReplicationFactor, topic.Spec.ReplicationFactor))
			return notAllowed("Kafka does not support changing the replication factor on an existing topic")
		}
	}

	// everything looks a-okay
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func notAllowed(msg string) *admissionv1beta1.AdmissionResponse {
	return &admissionv1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: msg,
		},
	}
}
