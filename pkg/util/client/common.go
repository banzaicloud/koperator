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

package client

import (
	"fmt"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/kafka"
)

func UseSSL(cluster *v1beta1.KafkaCluster) bool {
	for _, val := range cluster.Spec.ListenersConfig.InternalListeners {
		if val.UsedForInnerBrokerCommunication && val.Type.IsSSL() {
			return true
		}
	}
	for _, val := range cluster.Spec.ListenersConfig.ExternalListeners {
		if val.UsedForInnerBrokerCommunication && val.Type.IsSSL() {
			return true
		}
	}
	return false
}

func getContainerPortForInnerCom(internalListeners []v1beta1.InternalListenerConfig, extListeners []v1beta1.ExternalListenerConfig) int32 {
	for _, val := range internalListeners {
		if val.UsedForInnerBrokerCommunication {
			return val.ContainerPort
		}
	}
	for _, val := range extListeners {
		if val.UsedForInnerBrokerCommunication {
			return val.ContainerPort
		}
	}
	return 0
}

func GenerateKafkaAddressWithoutPort(cluster *v1beta1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s.svc.%s",
			fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name),
			cluster.Namespace,
			cluster.Spec.GetKubernetesClusterDomain(),
		)
	}
	return fmt.Sprintf("%s.%s.svc.%s",
		fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name),
		cluster.Namespace,
		cluster.Spec.GetKubernetesClusterDomain(),
	)
}

func GenerateKafkaAddress(cluster *v1beta1.KafkaCluster) string {
	return fmt.Sprintf("%s:%d", GenerateKafkaAddressWithoutPort(cluster), getContainerPortForInnerCom(
		cluster.Spec.ListenersConfig.InternalListeners, cluster.Spec.ListenersConfig.ExternalListeners))
}
