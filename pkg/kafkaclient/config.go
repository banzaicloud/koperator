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

package kafkaclient

import (
	"crypto/tls"
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/pki"
	"github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const kafkaDefaultTimeout = int64(5)

// KafkaConfig are the options to creating a new ClusterAdmin client
type KafkaConfig struct {
	BrokerURI string
	UseSSL    bool
	TLSConfig *tls.Config

	OperationTimeout int64
}

// ClusterConfig creates connection options from a KafkaCluster CR
func ClusterConfig(client client.Client, cluster *v1beta1.KafkaCluster) (*KafkaConfig, error) {
	conf := &KafkaConfig{}
	conf.BrokerURI = generateKafkaAddress(cluster)
	conf.OperationTimeout = kafkaDefaultTimeout

	if cluster.Spec.ListenersConfig.SSLSecrets != nil {
		tlsConfig, err := pki.GetPKIManager(client, cluster).GetControllerTLSConfig()
		if err != nil {
			return conf, err
		}
		conf.UseSSL = true
		conf.TLSConfig = tlsConfig
	}
	return conf, nil
}

func generateKafkaAddress(cluster *v1beta1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s:%d",
			fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name),
			cluster.Namespace,
			cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort,
		)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d",
		fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name),
		cluster.Namespace,
		cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort,
	)
}
