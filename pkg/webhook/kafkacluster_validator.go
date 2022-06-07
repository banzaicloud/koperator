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

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
)

func (s *webhookServer) validateKafkaCluster(kafkaClusterNew *banzaicloudv1beta1.KafkaCluster) *admissionv1.AdmissionResponse {
	ctx := context.Background()
	// get the Old kafkaCluster CR
	kafkaClusterSpecOld := banzaicloudv1beta1.KafkaCluster{}
	err := s.client.Get(ctx, client.ObjectKey{Name: kafkaClusterNew.GetName(), Namespace: kafkaClusterNew.GetNamespace()}, &kafkaClusterSpecOld)
	if err != nil {
		// New kafkaCluster has been added thus no need to check storage removal
		if apierrors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "couldn't get KafkaCluster custom resource")
		return notAllowed("API failure while retrieving KafkaCluster CR, please try again", metav1.StatusReasonInternalError)
	}

	res := checkBrokerStorageRemoval(&kafkaClusterSpecOld.Spec, &kafkaClusterNew.Spec)
	if res != nil {
		return res
	}

	// everything looks a-okay
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

// checkBrokerStorageRemoval checks if there is any broker storage which has been removed. If yes, admission will be rejected
func checkBrokerStorageRemoval(kafkaClusterSpecOld, kafkaClusterSpecNew *banzaicloudv1beta1.KafkaClusterSpec) *admissionv1.AdmissionResponse {
	for _, brokerOld := range kafkaClusterSpecOld.Brokers {
		for _, brokerNew := range kafkaClusterSpecNew.Brokers {
			if brokerOld.Id == brokerNew.Id {
				brokerConfigsOld, _ := brokerOld.GetBrokerConfig(*kafkaClusterSpecOld)
				brokerConfigsNew, _ := brokerNew.GetBrokerConfig(*kafkaClusterSpecNew)
				for _, storageConfigOld := range brokerConfigsOld.StorageConfigs {
					isStorageFound := false

					for _, storageConfigNew := range brokerConfigsNew.StorageConfigs {
						if storageConfigOld.MountPath == storageConfigNew.MountPath {
							isStorageFound = true
							break
						}
					}
					if !isStorageFound {
						log.Info(fmt.Sprintf("Not allowed to remove broker storage with mountPath: %s from brokerID: %v", storageConfigOld.MountPath, brokerOld.Id))
						return notAllowed(fmt.Sprintf("Removing storage from a runnng broker is not supported! (mountPath: %s, brokerID: %v)", storageConfigOld.MountPath, brokerOld.Id), metav1.StatusReasonInvalid)
					}
				}
			}
		}
	}
	return nil
}
