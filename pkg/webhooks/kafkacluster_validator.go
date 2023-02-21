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

	"emperror.dev/errors"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/go-logr/logr"

	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

type KafkaClusterValidator struct {
	Log logr.Logger
}

func (s KafkaClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var allErrs field.ErrorList
	kafkaClusterOld := oldObj.(*banzaicloudv1beta1.KafkaCluster)
	kafkaClusterNew := newObj.(*banzaicloudv1beta1.KafkaCluster)
	log := s.Log.WithValues("name", kafkaClusterNew.GetName(), "namespace", kafkaClusterNew.GetNamespace())

	// check storage removal
	fieldErr, err := checkBrokerStorageRemoval(&kafkaClusterOld.Spec, &kafkaClusterNew.Spec)
	if err != nil {
		log.Error(err, errorDuringValidationMsg)
		return apierrors.NewInternalError(errors.WithMessage(err, errorDuringValidationMsg))
	}
	if fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}

	// check external listeners
	extListenerErrs := checkExternalListeners(&kafkaClusterNew.Spec)
	if extListenerErrs != nil {
		allErrs = append(allErrs, extListenerErrs...)
	}

	if len(allErrs) == 0 {
		return nil
	}

	log.Info("rejected", "invalid field(s)", allErrs.ToAggregate().Error())
	return apierrors.NewInvalid(
		kafkaClusterNew.GroupVersionKind().GroupKind(),
		kafkaClusterNew.Name, allErrs)
}

func (s KafkaClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	var allErrs field.ErrorList
	kafkaCluster := obj.(*banzaicloudv1beta1.KafkaCluster)
	log := s.Log.WithValues("name", kafkaCluster.GetName(), "namespace", kafkaCluster.GetNamespace())

	// check external listeners
	extListenerErrs := checkExternalListeners(&kafkaCluster.Spec)
	if extListenerErrs != nil {
		allErrs = append(allErrs, extListenerErrs...)
	}

	if len(allErrs) == 0 {
		return nil
	}

	log.Info("rejected", "invalid field(s)", allErrs.ToAggregate().Error())
	return apierrors.NewInvalid(
		kafkaCluster.GroupVersionKind().GroupKind(),
		kafkaCluster.Name, allErrs)
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
						return field.Invalid(field.NewPath("spec").Child("brokers").Index(int(brokerNew.Id)).Child("brokerConfigGroup"), brokerNew.BrokerConfigGroup, unsupportedRemovingStorageMsg+", provided brokerConfigGroup not found"), nil
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
							return field.Invalid(field.NewPath("spec").Child("brokers").Index(k).Child("brokerConfigGroup"), brokerNew.BrokerConfigGroup, fmt.Sprintf("%s, missing storageConfig mounthPath: %s", unsupportedRemovingStorageMsg, storageConfigOld.MountPath)), nil
						}
						return field.NotFound(field.NewPath("spec").Child("brokers").Index(k).Child("storageConfig").Index(e), storageConfigOld.MountPath+", "+unsupportedRemovingStorageMsg), nil
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

// checkExternalListeners validates the spec.listenersConfig.externalListeners object
func checkExternalListeners(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	// if there are no externalListeners, there is no need to perform the rest of the check in this function
	if kafkaClusterSpec.ListenersConfig.ExternalListeners == nil {
		return nil
	}

	var allErrs field.ErrorList

	// check values of externalStartingPort
	//const maxPort int32 = 65535
	//for i, extLstn := range kafkaClusterSpec.ListenersConfig.ExternalListeners {
	//	var errbrokerids []int32
	//	for _, broker := range kafkaClusterSpec.Brokers {
	//		if extLstn.ExternalStartingPort+broker.Id < 1 || extLstn.ExternalStartingPort+broker.Id > maxPort {
	//			errbrokerids = append(errbrokerids, broker.Id)
	//		}
	//	}
	//	if len(errbrokerids) > 0 {
	//		errmsg := invalidExternalListenerPortErrMsg + ": " + fmt.Sprintf("ExternalListener '%s' would generate invalid port numbers (not between 1 and 65535) for brokers %v", extLstn.Name, errbrokerids)
	//		fldErr := field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("externalStartingPort"), extLstn.ExternalStartingPort, errmsg)
	//		allErrs = append(allErrs, fldErr)
	//	}
	//}

	// check values of containerPort
	//var containerPorts = make(map[int32]int)
	//for _, extLstn := range kafkaClusterSpec.ListenersConfig.ExternalListeners {
	//	containerPorts[extLstn.ContainerPort] += 1
	//}
	//for i, extLstn := range kafkaClusterSpec.ListenersConfig.ExternalListeners {
	//	if containerPorts[extLstn.ContainerPort] > 1 {
	//		fldErr := field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("containerPort"), extLstn.ContainerPort)
	//		allErrs = append(allErrs, fldErr)
	//	}
	//}

	// check values of externalStartingPort
	allErrs = append(allErrs, checkExternalListenerStartingPort(kafkaClusterSpec)...)

	// check values of containerPort
	allErrs = append(allErrs, checkExternalListenerContainerPort(kafkaClusterSpec.ListenersConfig.ExternalListeners)...)

	return allErrs
}

// checkExternalListenerStartingPort checks the generic sanity of the resulting external port (valid port number <65535)
func checkExternalListenerStartingPort(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	var allErrs field.ErrorList
	const maxPort int32 = 65535
	for i, extLstn := range kafkaClusterSpec.ListenersConfig.ExternalListeners {
		var errbrokerids []int32
		for _, broker := range kafkaClusterSpec.Brokers {
			if extLstn.ExternalStartingPort+broker.Id < 1 || extLstn.ExternalStartingPort+broker.Id > maxPort {
				errbrokerids = append(errbrokerids, broker.Id)
			}
		}
		if len(errbrokerids) > 0 {
			errmsg := invalidExternalListenerPortErrMsg + ": " + fmt.Sprintf("ExternalListener '%s' would generate invalid port numbers (not between 1 and 65535) for brokers %v", extLstn.Name, errbrokerids)
			fldErr := field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("externalStartingPort"), extLstn.ExternalStartingPort, errmsg)
			allErrs = append(allErrs, fldErr)
		}
	}
	return allErrs
}

// checkExternalListenerContainerPort checks for duplicate containerPort numbers which would subsequently generate an error when creating a Service
func checkExternalListenerContainerPort(externalListeners []banzaicloudv1beta1.ExternalListenerConfig) field.ErrorList {
	var allErrs field.ErrorList
	var containerPorts = make(map[int32]int)
	for _, extLstn := range externalListeners {
		containerPorts[extLstn.ContainerPort] += 1
	}
	for i, extLstn := range externalListeners {
		if containerPorts[extLstn.ContainerPort] > 1 {
			fldErr := field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("containerPort"), extLstn.ContainerPort)
			allErrs = append(allErrs, fldErr)
		}
	}
	return allErrs
}
