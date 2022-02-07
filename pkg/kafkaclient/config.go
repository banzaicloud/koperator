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

	"emperror.dev/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/pki"
	"github.com/banzaicloud/koperator/pkg/util"
	clientutil "github.com/banzaicloud/koperator/pkg/util/client"
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
	conf.BrokerURI = clientutil.GenerateKafkaAddress(cluster)
	conf.OperationTimeout = kafkaDefaultTimeout
	if clientutil.UseSSL(cluster) {
		var tlsConfig *tls.Config
		var err error
		if cluster.Spec.GetClientSSLCertSecretName() != "" {
			tlsConfig, err = util.GetClientTLSConfig(client, types.NamespacedName{Name: cluster.Spec.GetClientSSLCertSecretName(), Namespace: cluster.Namespace})
		} else if cluster.Spec.ListenersConfig.SSLSecrets != nil {
			tlsConfig, err = pki.GetPKIManager(client, cluster, v1beta1.PKIBackendProvided).GetControllerTLSConfig()
		} else {
			err = errors.New("either 'clientSSLCertSecret' or 'sslSecrets' must be specified as internal listener used for inner communication uses SSL")
		}
		if err != nil {
			return conf, err
		}
		conf.UseSSL = true
		conf.TLSConfig = tlsConfig
	}
	return conf, nil
}
