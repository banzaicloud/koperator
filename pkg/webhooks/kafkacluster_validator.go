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
	"golang.org/x/exp/slices"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/go-logr/logr"

	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

// +kubebuilder:webhook:verbs=update,path=/validate-kafka-banzaicloud-io-v1beta1-kafkacluster,mutating=false,failurePolicy=fail,groups=kafka.banzaicloud.io,resources=kafkaclusters,versions=v1beta1,name=kafkaclusters.kafka.banzaicloud.io,sideEffects=None,admissionReviewVersions=v1

type KafkaClusterValidator struct {
	Log logr.Logger
}

func (s KafkaClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var allErrs field.ErrorList
	kafkaClusterOld := oldObj.(*banzaicloudv1beta1.KafkaCluster)
	kafkaClusterNew := newObj.(*banzaicloudv1beta1.KafkaCluster)
	log := s.Log.WithValues("name", kafkaClusterNew.GetName(), "namespace", kafkaClusterNew.GetNamespace())
	fieldErr, err := checkBrokerStorageRemoval(&kafkaClusterOld.Spec, &kafkaClusterNew.Spec)
	if err != nil {
		log.Error(err, errorDuringValidationMsg)
		return apiErrors.NewInternalError(errors.WithMessage(err, errorDuringValidationMsg))
	}
	if fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}
	if len(allErrs) == 0 {
		return nil
	}
	log.Info("rejected", "invalid field(s)", allErrs.ToAggregate().Error())
	return apiErrors.NewInvalid(
		kafkaClusterNew.GroupVersionKind().GroupKind(),
		kafkaClusterNew.Name, allErrs)
}

func (s KafkaClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (s KafkaClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// checkBrokerStorageRemoval checks if there is any broker storage which has been removed. If yes, admission will be rejected
func checkBrokerStorageRemoval(kafkaClusterSpecOld, kafkaClusterSpecNew *banzaicloudv1beta1.KafkaClusterSpec) (*field.Error, error) {
	for j := range kafkaClusterSpecOld.Brokers {
		brokerOld := &kafkaClusterSpecOld.Brokers[j]
		for k := range kafkaClusterSpecNew.Brokers {
			brokerNew := &kafkaClusterSpecNew.Brokers[k]
			if brokerOld.Id == brokerNew.Id {
				brokerConfigsOld, err := brokerOld.GetBrokerConfig(*kafkaClusterSpecOld)
				if err != nil {
					return nil, err
				}
				// checking broukerConfigGroup existence
				if brokerNew.BrokerConfigGroup != "" {
					if _, exists := kafkaClusterSpecNew.BrokerConfigGroups[brokerNew.BrokerConfigGroup]; !exists {
						return field.Invalid(field.NewPath("spec").Child("brokers").Index(int(brokerNew.Id)).Child("brokerConfigGroup"), brokerNew.BrokerConfigGroup, removingStorageMsg+", provided brokerConfigGroup not found"), nil
					}
				}
				brokerConfigsNew, err := brokerNew.GetBrokerConfig(*kafkaClusterSpecNew)
				if err != nil {
					return nil, err
				}
				for e := range brokerConfigsOld.StorageConfigs {
					storageConfigOld := &brokerConfigsOld.StorageConfigs[e]
					isStorageFound := false

					for f := range brokerConfigsNew.StorageConfigs {
						storageConfigNew := &brokerConfigsNew.StorageConfigs[f]
						if storageConfigOld.MountPath == storageConfigNew.MountPath {
							isStorageFound = true
							break
						}
					}
					if !isStorageFound {
						fromConfigGroup := getMissingMounthPathLocation(storageConfigOld.MountPath, kafkaClusterSpecOld, int32(k))
						if fromConfigGroup != nil && *fromConfigGroup {
							return field.Invalid(field.NewPath("spec").Child("brokers").Index(k).Child("brokerConfigGroup"), brokerNew.BrokerConfigGroup, fmt.Sprintf("%s, missing storageConfig mounthPath: %s", removingStorageMsg, storageConfigOld.MountPath)), nil
						}
						return field.NotFound(field.NewPath("spec").Child("brokers").Index(k).Child("storageConfig").Index(e), storageConfigOld.MountPath+", "+removingStorageMsg), nil
					}
				}
			}
		}
	}
	return nil, nil
}
func getMissingMounthPathLocation(mounthPath string, kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec, brokerId int32) (fromConfigGroup *bool) {
	if brokerId < 0 || int(brokerId) >= len(kafkaClusterSpec.Brokers) {
		return nil
	}

	brokerConfigGroup := kafkaClusterSpec.Brokers[brokerId].BrokerConfigGroup
	brokerConfigs, ok := kafkaClusterSpec.BrokerConfigGroups[brokerConfigGroup]
	if !ok {
		fromConfigGroup = util.BoolPointer(true)
	}
	idx := slices.IndexFunc(brokerConfigs.StorageConfigs, func(c banzaicloudv1beta1.StorageConfig) bool { return c.MountPath == mounthPath })
	if idx != -1 {
		fromConfigGroup = util.BoolPointer(true)
	}

	perBrokerConfigs := kafkaClusterSpec.Brokers[brokerId].BrokerConfig
	if perBrokerConfigs != nil {
		idx := slices.IndexFunc(perBrokerConfigs.StorageConfigs, func(c banzaicloudv1beta1.StorageConfig) bool { return c.MountPath == mounthPath })
		if idx != -1 {
			fromConfigGroup = util.BoolPointer(false)
		}
	}
	return fromConfigGroup
}
