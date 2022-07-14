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

package webhooks

import (
	"context"
	"fmt"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
)

var (
	log = logf.Log.WithName("webhooks")
)

type KafkaClusterValidator struct {
}

func (s KafkaClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	log := log.WithValues("KafkaCluster")
	var allErrs field.ErrorList
	kafkaClusterOld, ok := oldObj.(*banzaicloudv1beta1.KafkaCluster)
	if !ok {
		log.Info(unableToRecognizeMsg)
		return apiErrors.NewBadRequest(unableToRecognizeMsg)
	}
	kafkaClusterNew, ok := newObj.(*banzaicloudv1beta1.KafkaCluster)
	if !ok {
		return apiErrors.NewBadRequest(unableToRecognizeMsg)
	}
	log = log.WithValues("name", kafkaClusterNew.GetName(), "namespace", kafkaClusterNew.GetNamespace())
	fieldErr := checkBrokerStorageRemoval(&kafkaClusterOld.Spec, &kafkaClusterNew.Spec)
	if fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}
	if len(allErrs) == 0 {
		return nil
	}
	log.Info(fmt.Sprintf(rejectingFieldsMsg, allErrs.ToAggregate().Error()))
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
func checkBrokerStorageRemoval(kafkaClusterSpecOld, kafkaClusterSpecNew *banzaicloudv1beta1.KafkaClusterSpec) *field.Error {
	for j := range kafkaClusterSpecOld.Brokers {
		brokerOld := &kafkaClusterSpecOld.Brokers[j]
		for k := range kafkaClusterSpecNew.Brokers {
			brokerNew := &kafkaClusterSpecNew.Brokers[k]
			if brokerOld.Id == brokerNew.Id {
				brokerConfigsOld, _ := brokerOld.GetBrokerConfig(*kafkaClusterSpecOld)
				brokerConfigsNew, _ := brokerNew.GetBrokerConfig(*kafkaClusterSpecNew)
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
						logMsg := fmt.Sprintf("Removing storage from a broker is not supported! (mountPath: %s, brokerID: %v)", storageConfigOld.MountPath, brokerOld.Id)
						return field.Invalid(field.NewPath("spec").Child("brokers").Index(k).Child("storageConfig").Index(e), storageConfigOld.MountPath, logMsg)
					}
				}
			}
		}
	}
	return nil
}
