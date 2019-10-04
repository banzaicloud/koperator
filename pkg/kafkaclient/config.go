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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	pkiutils "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const kafkaDefaultTimeout = int64(10)

// KafkaConfig are the options to creating a new ClusterAdmin client
type KafkaConfig struct {
	BrokerURI string
	UseSSL    bool
	TLSConfig *tls.Config

	IssueCA     string
	IssueCAKind string

	OperationTimeout int64
}

// ClusterConfig creates connection options from a KafkaCluster CR
func ClusterConfig(client client.Client, cluster *banzaicloudv1beta1.KafkaCluster) (*KafkaConfig, error) {
	conf := &KafkaConfig{}
	conf.BrokerURI = generateKafkaAddress(cluster)
	conf.OperationTimeout = kafkaDefaultTimeout

	if cluster.Spec.ListenersConfig.SSLSecrets != nil && cluster.Spec.ListenersConfig.InternalListeners[determineInternalListenerForInnerCom(cluster.Spec.ListenersConfig.InternalListeners)].Type != "plaintext" {
		var err error
		tlsKeys := &corev1.Secret{}
		err = client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: cluster.Namespace,
				Name:      cluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName},
			tlsKeys,
		)
		if err != nil {
			if apierrors.IsNotFound(err) {
				err = errorfactory.New(errorfactory.ResourceNotReady{}, err, "controller secret not found")
			}
			return conf, err
		}
		clientCert := tlsKeys.Data[banzaicloudv1alpha1.ClientCertKey]
		clientKey := tlsKeys.Data[banzaicloudv1alpha1.ClientPrivateKeyKey]
		caCert := tlsKeys.Data[banzaicloudv1alpha1.CACertKey]
		x509ClientCert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			err = errorfactory.New(errorfactory.InternalError{}, err, "could not decode controller certificate")
			return conf, err
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(caCert)
		t := &tls.Config{
			Certificates: []tls.Certificate{x509ClientCert},
			RootCAs:      rootCAs,
		}

		conf.UseSSL = true
		conf.TLSConfig = t
		conf.IssueCA = fmt.Sprintf(pkiutils.BrokerIssuerTemplate, cluster.Name)
		conf.IssueCAKind = "ClusterIssuer"
	}

	return conf, nil
}

func determineInternalListenerForInnerCom(internalListeners []banzaicloudv1beta1.InternalListenerConfig) int {
	for id, val := range internalListeners {
		if val.UsedForInnerBrokerCommunication {
			return id
		}
	}
	return 0
}

func generateKafkaAddress(cluster *banzaicloudv1beta1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s:%d", fmt.Sprintf(kafkautils.HeadlessServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
}
