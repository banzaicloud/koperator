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

package k8sutil

import (
	"context"
	"fmt"

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func LookupKafkaCluster(client runtimeClient.Client, ref v1alpha1.ClusterReference) (cluster *v1alpha1.KafkaCluster, err error) {
	cluster = &v1alpha1.KafkaCluster{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, cluster)
	return
}

func LookupControllerSecret(client runtimeClient.Client, ref v1alpha1.ClusterReference, controllerTempl string) (secret *corev1.Secret, err error) {
	secret = &corev1.Secret{}
	secretName := fmt.Sprintf(controllerTempl, ref.Name)
	err = client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: ref.Namespace}, secret)
	return
}
