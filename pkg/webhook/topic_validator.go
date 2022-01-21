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

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
)

const (
	cantConnectErrorMsg            = "Failed to connect to kafka cluster"
	invalidReplicationFactorErrMsg = "Replication factor is larger than the number of nodes in the kafka cluster"
)

func (s *webhookServer) validateKafkaTopic(topic *banzaicloudv1alpha1.KafkaTopic) *admissionv1beta1.AdmissionResponse {
	ctx := context.TODO()
	log.Info(fmt.Sprintf("Doing pre-admission validation of kafka topic %s", topic.Spec.Name))

	// Get the referenced KafkaCluster
	clusterName := topic.Spec.ClusterRef.Name
	clusterNamespace := topic.Spec.ClusterRef.Namespace
	if clusterNamespace == "" {
		clusterNamespace = topic.GetNamespace()
	}
	var cluster *banzaicloudv1beta1.KafkaCluster
	var err error

	// Check if the cluster being referenced actually exists
	if cluster, err = k8sutil.LookupKafkaCluster(s.client, clusterName, clusterNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			if k8sutil.IsMarkedForDeletion(topic.ObjectMeta) {
				log.Info("Deleted as a result of a cluster deletion")
				return &admissionv1beta1.AdmissionResponse{
					Allowed: true,
				}
			}
			log.Error(err, "Referenced kafka cluster does not exist")
			return notAllowed(
				fmt.Sprintf("KafkaCluster '%s' in the namespace '%s' does not exist", topic.Spec.ClusterRef.Name, topic.Spec.ClusterRef.Namespace),
				metav1.StatusReasonNotFound,
			)
		}
		log.Error(err, "API failure while running topic validation")
		return notAllowed("API failure while validating topic, please try again", metav1.StatusReasonServiceUnavailable)
	}

	if k8sutil.IsMarkedForDeletion(cluster.ObjectMeta) {
		// Let this through, it's a delete topic request from a parent cluster being
		// deleted
		log.Info("Cluster is going down for deletion, assuming a delete topic request")
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	res := s.checkExistingKafkaTopicCRs(ctx, clusterNamespace, topic)
	if res != nil {
		return res
	}

	res = s.checkKafka(ctx, topic, cluster)
	if res != nil {
		return res
	}

	// everything looks a-okay
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}

// checkKafka creates a Kafka admin client and connects to the Kafka brokers to check
// whether the referred topic exists, and what are its properties
func (s *webhookServer) checkKafka(ctx context.Context, topic *banzaicloudv1alpha1.KafkaTopic,
	cluster *banzaicloudv1beta1.KafkaCluster) *admissionv1beta1.AdmissionResponse {
	// retrieve an admin client for the cluster
	broker, closeClient, err := s.newKafkaFromCluster(s.client, cluster)
	if err != nil {
		// Log as info to not cause stack traces when making CC topic
		log.Info(cantConnectErrorMsg, "error", err.Error())
		return notAllowed(fmt.Sprintf("%s: %s", cantConnectErrorMsg, topic.Spec.ClusterRef.Name), metav1.StatusReasonServiceUnavailable)
	}
	defer closeClient()

	existing, err := broker.GetTopic(topic.Spec.Name)
	if err != nil {
		log.Error(err, "Failed to list topics")
		return notAllowed(fmt.Sprintf("Failed to list topics for kafka cluster: %s", topic.Spec.ClusterRef.Name), metav1.StatusReasonInternalError)
	}

	// The topic exists
	if existing != nil {
		// Check if this is the correct CR for this topic
		topicCR := &banzaicloudv1alpha1.KafkaTopic{}
		if err := s.client.Get(ctx, types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, topicCR); err != nil {
			if apierrors.IsNotFound(err) {
				// User is trying to overwrite an existing topic - bad user
				log.Info("User attempted to create topic with name that already exists in the kafka cluster")
				return notAllowed(
					fmt.Sprintf("Topic '%s' already exists on kafka cluster '%s'", topic.Spec.Name, topic.Spec.ClusterRef.Name),
					metav1.StatusReasonAlreadyExists,
				)
			}
			log.Error(err, "API failure while running topic validation")
			return notAllowed("API failure while validating topic, please try again", metav1.StatusReasonServiceUnavailable)
		}

		// make sure the user isn't trying to decrease partition count
		if existing.NumPartitions > topic.Spec.Partitions {
			log.Info(fmt.Sprintf("Spec is requesting partition decrease from %v to %v, rejecting", existing.NumPartitions, topic.Spec.Partitions))
			return notAllowed("Kafka does not support decreasing partition count on an existing topic", metav1.StatusReasonInvalid)
		}

		// check if the user is trying to change the replication factor
		if existing.ReplicationFactor != int16(topic.Spec.ReplicationFactor) {
			log.Info(fmt.Sprintf("Spec is requesting replication factor change from %v to %v, rejecting", existing.ReplicationFactor, topic.Spec.ReplicationFactor))
			return notAllowed("Kafka does not support changing the replication factor on an existing topic", metav1.StatusReasonInvalid)
		}

		// the topic does not exist check if requesting a replication factor larger than the broker size
	} else if int(topic.Spec.ReplicationFactor) > broker.NumBrokers() {
		log.Info(fmt.Sprintf("Spec is requesting replication factor of %v, larger than cluster size of %v", topic.Spec.ReplicationFactor, broker.NumBrokers()))
		return notAllowed(invalidReplicationFactorErrMsg, metav1.StatusReasonBadRequest)
	}
	return nil
}

// checkExistingKafkaTopicCRs checks whether there's any other duplicate KafkaTopic CR exists
// that refers to the same KafkaCluster's same topic
func (s *webhookServer) checkExistingKafkaTopicCRs(ctx context.Context,
	clusterNamespace string, topic *banzaicloudv1alpha1.KafkaTopic) *admissionv1beta1.AdmissionResponse {
	// check KafkaTopic in the referred KafkaCluster's namespace
	kafkaTopicList := banzaicloudv1alpha1.KafkaTopicList{}
	err := s.client.List(ctx, &kafkaTopicList, client.MatchingFields{"spec.name": topic.Spec.Name})
	if err != nil {
		log.Info(fmt.Sprintf("Failed to list KafkaTopics. Error message: %s", err))
		return notAllowed("API failure while retrieving KafkaTopic list, please try again", metav1.StatusReasonServiceUnavailable)
	}

	var foundKafkaTopic *banzaicloudv1alpha1.KafkaTopic
	for i, kafkaTopic := range kafkaTopicList.Items {
		// filter the cr under admission
		if kafkaTopic.GetName() == topic.GetName() && kafkaTopic.GetNamespace() == topic.GetNamespace() {
			continue
		}

		referredNamespace := kafkaTopic.Spec.ClusterRef.Namespace
		referredName := kafkaTopic.Spec.ClusterRef.Name
		if referredName == topic.Spec.ClusterRef.Name {
			if (kafkaTopic.GetNamespace() == clusterNamespace && referredNamespace == "") || referredNamespace == clusterNamespace {
				foundKafkaTopic = &kafkaTopicList.Items[i]
				break
			}
		}
	}
	if foundKafkaTopic != nil {
		msg := fmt.Sprintf("KafkaTopic CR '%s' in namesapce '%s' is already referencing Kafka topic '%s'",
			foundKafkaTopic.Name, foundKafkaTopic.Namespace, foundKafkaTopic.Spec.Name)
		log.Info(msg)
		return notAllowed(msg, metav1.StatusReasonInvalid)
	}

	return nil
}
