// Copyright © 2022 Cisco Systems, Inc. and/or its affiliates
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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"emperror.dev/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/util"
)

const (
	TopicManagedByAnnotationKey            = "managedBy"
	TopicManagedByKoperatorAnnotationValue = "koperator"
)

type KafkaTopicValidator struct {
	Client              client.Client
	NewKafkaFromCluster func(client.Client, *banzaicloudv1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error)
	Log                 logr.Logger
}

func (s KafkaTopicValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return s.validate(ctx, obj)
}

func (s KafkaTopicValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	return s.validate(ctx, newObj)
}

func (s KafkaTopicValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (s *KafkaTopicValidator) validate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	kafkaTopic := obj.(*banzaicloudv1alpha1.KafkaTopic)
	log := s.Log.WithValues("name", kafkaTopic.GetName(), "namespace", kafkaTopic.GetNamespace())

	fieldErrs, err := s.validateKafkaTopic(ctx, log, kafkaTopic)
	if err != nil {
		log.Error(err, errorDuringValidationMsg)
		return nil, apierrors.NewInternalError(errors.WithMessage(err, errorDuringValidationMsg))
	}
	if len(fieldErrs) == 0 {
		return nil, nil
	}
	log.Info("rejected", "invalid field(s)", fieldErrs.ToAggregate().Error())
	return nil, apierrors.NewInvalid(
		kafkaTopic.GetObjectKind().GroupVersionKind().GroupKind(),
		kafkaTopic.Name, fieldErrs)
}

func (s *KafkaTopicValidator) validateKafkaTopic(ctx context.Context, log logr.Logger, topic *banzaicloudv1alpha1.KafkaTopic) (field.ErrorList, error) {
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
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, cantConnectAPIServerMsg)
		}
		if k8sutil.IsMarkedForDeletion(topic.ObjectMeta) {
			log.Info("Deleted as a result of a cluster deletion")
			return nil, nil
		}
		logMsg = fmt.Sprintf("kafkaCluster '%s' in the namespace '%s' does not exist", topic.Spec.ClusterRef.Name, topic.Spec.ClusterRef.Namespace)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterRef").Child("name"), clusterName, logMsg))
		// retrun is needed here because later this cluster is used for further checks but it is nil
		return allErrs, nil
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

	fieldErrList, err := s.checkKafka(ctx, topic, cluster)
	if err != nil {
		return nil, err
	}

	allErrs = append(allErrs, fieldErrList...)

	return allErrs, nil
}

// checkKafka creates a Kafka admin client and connects to the Kafka brokers to check
// whether the referred topic exists, and what are its properties
func (s *KafkaTopicValidator) checkKafka(ctx context.Context, topic *banzaicloudv1alpha1.KafkaTopic,
	cluster *banzaicloudv1beta1.KafkaCluster) (field.ErrorList, error) {
	// retrieve an admin client for the cluster
	broker, closeClient, err := s.NewKafkaFromCluster(s.Client, cluster)
	if err != nil {
		// Log as info to not cause stack traces when making CC topic
		return nil, errors.WrapIff(err, fmt.Sprintf("%s: %s", cantConnectErrorMsg, topic.Spec.ClusterRef.Name))
	}
	defer closeClient()

	existing, err := broker.GetTopic(topic.Spec.Name)
	if err != nil {
		return nil, errors.WrapIff(err, fmt.Sprintf("failed to list topics for kafka cluster: %s", topic.Spec.ClusterRef.Name))
	}

	var allErrs field.ErrorList
	// The topic exists
	if existing != nil {
		// Check if this is the correct CR for this topic
		topicCR := &banzaicloudv1alpha1.KafkaTopic{}
		if err := s.Client.Get(ctx, types.NamespacedName{Name: topic.Name, Namespace: topic.Namespace}, topicCR); err != nil {
			// Checking that the validation request is update
			if apierrors.IsNotFound(err) {
				if manager, ok := topic.GetAnnotations()[TopicManagedByAnnotationKey]; !ok || strings.ToLower(manager) != TopicManagedByKoperatorAnnotationValue {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("name"), topic.Spec.Name,
						fmt.Sprintf(`topic "%s" already exists on kafka cluster and it is not managed by Koperator,
					if you want it to be managed by Koperator so you can modify its configurations through a KafkaTopic CR,
					add this "%s: %s" annotation to this KafkaTopic CR`, topic.Spec.Name, TopicManagedByAnnotationKey, TopicManagedByKoperatorAnnotationValue)))
				}
				// Comparing KafkaTopic configuration with the existing
				if existing.NumPartitions != topic.Spec.Partitions {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("partitions"), topic.Spec.Partitions,
						fmt.Sprintf(`When creating KafkaTopic CR for existing topic, initially its partition number must be the same as what the existing kafka topic has (given: %v present: %v)`, topic.Spec.Partitions, existing.NumPartitions)))
				}
				if existing.ReplicationFactor != int16(topic.Spec.ReplicationFactor) {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replicationfactor"), topic.Spec.ReplicationFactor,
						fmt.Sprintf(`When creating KafkaTopic CR for existing topic, initially its replication factor must be the same as what the existing kafka topic has (given: %v present: %v)`, topic.Spec.ReplicationFactor, existing.ReplicationFactor)))
				}

				if diff := cmp.Diff(existing.ConfigEntries, util.MapStringStringPointer(topic.Spec.Config), cmpopts.EquateEmpty()); diff != "" {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("config"), topic.Spec.Partitions,
						fmt.Sprintf(`When creating KafkaTopic CR for existing topic, initially its configuration must be the same as the existing kafka topic configuration.
						Difference: %s`, diff)))
				}

				if len(allErrs) > 0 {
					return allErrs, nil
				}
			} else {
				return nil, errors.WrapIff(err, cantConnectAPIServerMsg)
			}
		}

		// make sure the user isn't trying to decrease partition count
		if existing.NumPartitions > topic.Spec.Partitions {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("partitions"), topic.Spec.Partitions,
				fmt.Sprintf("kafka does not support decreasing partition count on an existing topic (from %v to %v)", existing.NumPartitions, topic.Spec.Partitions)))
		}

		// check if the user is trying to change the replication factor
		if existing.ReplicationFactor != int16(topic.Spec.ReplicationFactor) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replicationFactor"), topic.Spec.ReplicationFactor,
				fmt.Sprintf("kafka does not support changing the replication factor on an existing topic (from %v to %v)", existing.ReplicationFactor, topic.Spec.ReplicationFactor)))
		}

		// the topic does not exist check if requesting a replication factor larger than the broker size
	} else if int(topic.Spec.ReplicationFactor) > broker.NumBrokers() {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replicationFactor"), topic.Spec.ReplicationFactor,
			fmt.Sprintf("%s (available brokers: %v)", invalidReplicationFactorErrMsg, broker.NumBrokers())))
	}
	return allErrs, nil
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
