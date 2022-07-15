// Copyright © 2022 Banzai Cloud
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

package webhooks

import (
	"context"
	"fmt"

	"emperror.dev/errors"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/util"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-kafka.banzaicloud-io-v1alpha1-kafkatopic,mutating=false,failurePolicy=fail,groups=kafka.banzaicloud.io,resources=kafkatopics,versions=v1alpha1,name=kafkatopics.kafka.banzaicloud.io,sideEffects=None,admissionReviewVersions=v1

type KafkaTopicValidator struct {
	Client              client.Client
	NewKafkaFromCluster func(client.Client, *banzaicloudv1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error)
	Log                 logr.Logger
}

func (s KafkaTopicValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return s.validate(ctx, obj)
}

func (s KafkaTopicValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return s.validate(ctx, newObj)
}

func (s KafkaTopicValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (s *KafkaTopicValidator) validate(ctx context.Context, obj runtime.Object) error {
	kafkaTopic := obj.(*banzaicloudv1alpha1.KafkaTopic)
	log := s.Log.WithValues("name", kafkaTopic.GetName(), "namespace", kafkaTopic.GetNamespace())
	fieldErrs, err := s.validateKafkaTopic(ctx, kafkaTopic, log)
	if err != nil {
		errMsg := fmt.Sprintf("error during validating kafkaTopic %s", kafkaTopic.Name)
		log.Error(err, errMsg)
		return apiErrors.NewInternalError(errors.WithMessage(err, errMsg))
	}
	if len(fieldErrs) == 0 {
		return nil
	}
	log.Info("rejected", "invalid field(s)", fieldErrs.ToAggregate().Error())
	return apiErrors.NewInvalid(
		kafkaTopic.GetObjectKind().GroupVersionKind().GroupKind(),
		kafkaTopic.Name, fieldErrs)
}

func (s *KafkaTopicValidator) validateKafkaTopic(ctx context.Context, topic *banzaicloudv1alpha1.KafkaTopic, log logr.Logger) (field.ErrorList, error) {
	var allErrs field.ErrorList
	var logMsg string
	// First check if the kafkatopic is valid
	if topic.Spec.Partitions < banzaicloudv1alpha1.MinPartitions || topic.Spec.Partitions == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("partitions"), topic.Spec.Partitions, outOfRangePartitionsErrMsg))
	}

	if topic.Spec.ReplicationFactor < banzaicloudv1alpha1.MinReplicationFactor || topic.Spec.ReplicationFactor == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replicationFactor"), topic.Spec.ReplicationFactor, outOfRangeReplicationFactorErrMsg))
	}

	// Get the referenced KafkaCluster
	clusterName := topic.Spec.ClusterRef.Name
	clusterNamespace := topic.Spec.ClusterRef.Namespace

	if clusterNamespace == "" {
		clusterNamespace = topic.GetNamespace()
	}
	var cluster *banzaicloudv1beta1.KafkaCluster
	var err error

	// Check if the cluster being referenced actually exists
	if cluster, err = k8sutil.LookupKafkaCluster(ctx, s.Client, clusterName, clusterNamespace); err != nil {
		if !apiErrors.IsNotFound(err) {
			return nil, errors.Wrap(err, cantConnectAPIServerMsg)
		}
		if k8sutil.IsMarkedForDeletion(topic.ObjectMeta) {
			log.Info("Deleted as a result of a cluster deletion")
			return nil, nil
		}
		logMsg = fmt.Sprintf("kafkaCluster '%s' in the namespace '%s' does not exist", topic.Spec.ClusterRef.Name, topic.Spec.ClusterRef.Namespace)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterRef").Child("name"), clusterName, logMsg))
	}
	if k8sutil.IsMarkedForDeletion(cluster.ObjectMeta) {
		// Let this through, it's a delete topic request from a parent cluster being deleted
		log.Info("Cluster is going down for deletion, assuming a delete topic request")
		return nil, nil
	}

	if util.ObjectManagedByClusterRegistry(cluster) {
		// referencing remote Kafka clusters is not allowed
		logMsg = fmt.Sprintf("kafkaCluster '%s' in the namespace '%s' is a remote kafka cluster", topic.Spec.ClusterRef.Name, topic.Spec.ClusterRef.Namespace)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterRef").Child("name"), clusterName, logMsg))
	}

	fieldErr, err := s.checkExistingKafkaTopicCRs(ctx, clusterNamespace, topic)
	if err != nil {
		return nil, err
	}
	if fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}

	fieldErr, err = s.checkKafka(ctx, topic, cluster)
	if err != nil {
		return nil, err
	}
	if fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}

	return allErrs, nil
}

// checkKafka creates a Kafka admin client and connects to the Kafka brokers to check
// whether the referred topic exists, and what are its properties
func (s *KafkaTopicValidator) checkKafka(ctx context.Context, topic *banzaicloudv1alpha1.KafkaTopic,
	cluster *banzaicloudv1beta1.KafkaCluster) (*field.Error, error) {
	// retrieve an admin client for the cluster
	broker, closeClient, err := s.NewKafkaFromCluster(s.Client, cluster)
	if err != nil {
		// Log as info to not cause stack traces when making CC topic
		return nil, errors.WrapIf(err, fmt.Sprintf("%s: %s", cantConnectErrorMsg, topic.Spec.ClusterRef.Name))
	}
	defer closeClient()

	existing, err := broker.GetTopic(topic.Spec.Name)
	if err != nil {
		return nil, errors.WrapIf(err, fmt.Sprintf("failed to list topics for kafka cluster: %s", topic.Spec.ClusterRef.Name))
	}

	// The topic exists
	if existing != nil {
		// Check if this is the correct CR for this topic
		topicCR := &banzaicloudv1alpha1.KafkaTopic{}
		if err := s.Client.Get(ctx, types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, topicCR); err != nil {
			if apiErrors.IsNotFound(err) {
				// User is trying to overwrite an existing topic - bad user
				logMsg := fmt.Sprintf("topic already exists on kafka cluster '%s'", topic.Spec.ClusterRef.Name)
				return field.Invalid(field.NewPath("spec").Child("name"), topic.Spec.Name, logMsg), nil
			}
			return nil, errors.WrapIf(err, cantConnectAPIServerMsg)
		}

		// make sure the user isn't trying to decrease partition count
		if existing.NumPartitions > topic.Spec.Partitions {
			logMsg := fmt.Sprintf("kafka does not support decreasing partition count on an existing topic (from %v to %v)", existing.NumPartitions, topic.Spec.Partitions)
			return field.Invalid(field.NewPath("spec").Child("partitions"), topic.Spec.Partitions, logMsg), nil
		}

		// check if the user is trying to change the replication factor
		if existing.ReplicationFactor != int16(topic.Spec.ReplicationFactor) {
			logMsg := fmt.Sprintf("kafka does not support changing the replication factor on an existing topic (from %v to %v)", existing.ReplicationFactor, topic.Spec.ReplicationFactor)
			return field.Invalid(field.NewPath("spec").Child("replicationFactor"), topic.Spec.ReplicationFactor, logMsg), nil
		}

		// the topic does not exist check if requesting a replication factor larger than the broker size
	} else if int(topic.Spec.ReplicationFactor) > broker.NumBrokers() {
		logMsg := fmt.Sprintf("%s (available brokers: %v)", invalidReplicationFactorErrMsg, broker.NumBrokers())
		return field.Invalid(field.NewPath("spec").Child("replicationFactor"), topic.Spec.ReplicationFactor, logMsg), nil
	}
	return nil, nil
}

// checkExistingKafkaTopicCRs checks whether there's any other duplicate KafkaTopic CR exists
// that refers to the same KafkaCluster's same topic
func (s *KafkaTopicValidator) checkExistingKafkaTopicCRs(ctx context.Context,
	clusterNamespace string, topic *banzaicloudv1alpha1.KafkaTopic) (*field.Error, error) {
	// check KafkaTopic in the referred KafkaCluster's namespace
	kafkaTopicList := banzaicloudv1alpha1.KafkaTopicList{}
	err := s.Client.List(ctx, &kafkaTopicList, client.MatchingFields{"spec.name": topic.Spec.Name})
	if err != nil {
		return nil, errors.Wrap(err, cantConnectAPIServerMsg)
	}

	var foundKafkaTopic *banzaicloudv1alpha1.KafkaTopic
	for i, kafkaTopic := range kafkaTopicList.Items {
		// filter the cr under admission
		if kafkaTopic.GetName() == topic.GetName() && kafkaTopic.GetNamespace() == topic.GetNamespace() {
			continue
		}

		// filter remote KafkaTopic CRs
		if util.ObjectManagedByClusterRegistry(&kafkaTopic) {
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
		logMsg := fmt.Sprintf("kafkaTopic CR '%s' in namesapce '%s' is already referencing to Kafka topic '%s'",
			foundKafkaTopic.Name, foundKafkaTopic.Namespace, foundKafkaTopic.Spec.Name)
		return field.Invalid(field.NewPath("spec").Child("name"), foundKafkaTopic.Spec.Name, logMsg), nil
	}

	return nil, nil
}
