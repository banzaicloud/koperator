// Copyright © 2020 Banzai Cloud
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

package kafka

import (
	"context"
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (r *Reconciler) serviceAccount() (runtime.Object, error) {

	exists, err := r.serviceAccountForKafkaClusterExists()
	if err != nil {
		return nil, err
	}

	if !exists {
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   r.KafkaCluster.Namespace,
				Name:        fmt.Sprintf(v1beta1.ServiceAccountNameFormat, r.KafkaCluster.Name),
				Labels:      LabelsForKafka(r.KafkaCluster.Name),
				Annotations: nil,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         r.KafkaCluster.APIVersion,
						Kind:               r.KafkaCluster.Kind,
						Name:               r.KafkaCluster.Name,
						UID:                r.KafkaCluster.UID,
						Controller:         util.BoolPointer(true),
						BlockOwnerDeletion: util.BoolPointer(true),
					},
				},
			},
		}

		return serviceAccount, nil
	}

	return nil, nil
}

func (r *Reconciler) serviceAccountForKafkaClusterExists() (bool, error) {
	serviceAccount := corev1.ServiceAccount{}
	namespacedName := types.NamespacedName{
		Namespace: r.KafkaCluster.Namespace,
		Name:      fmt.Sprintf(v1beta1.ServiceAccountNameFormat, r.KafkaCluster.Name),
	}

	err := r.Get(context.Background(), namespacedName, &serviceAccount)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
