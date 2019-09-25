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

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// LookupKafkaCluster returns the running cluster instance based on its name and namespace
func LookupKafkaCluster(client runtimeClient.Client, clusterName, clusterNamespace string) (cluster *v1beta1.KafkaCluster, err error) {
	cluster = &v1beta1.KafkaCluster{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: clusterName, Namespace: clusterNamespace}, cluster)
	return
}

// This could be used if we get rid of the "intermediate" certificate we create for now during cluster creation
// func LookupControllerSecret(client runtimeClient.Client, clusterName, clusterNamespace, controllerTempl string) (secret *corev1.Secret, err error) {
// 	secret = &corev1.Secret{}
// 	secretName := fmt.Sprintf(controllerTempl, clusterName)
// 	err = client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: clusterNamespace}, secret)
// 	return
// }
