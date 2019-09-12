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

package kafkautil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const kafkaDefaultTimeout = int64(10)

// KafkaConfig are the opti6ons to creating a new ClusterAdmin client
type KafkaConfig struct {
	BrokerURI string
	UseSSL    bool
	TLSConfig *tls.Config

	IssueCA     string
	IssueCAKind string

	OperationTimeout int64
}

// ClusterConfig creates connection options from a KafkaCluster CR
func ClusterConfig(client client.Client, cluster *v1alpha1.KafkaCluster) (*KafkaConfig, error) {
	conf := &KafkaConfig{}
	conf.BrokerURI = generateKafkaAddress(cluster)
	conf.OperationTimeout = kafkaDefaultTimeout

	if cluster.Spec.ListenersConfig.SSLSecrets != nil {
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
		clientCert := tlsKeys.Data[v1alpha1.ClientCertKey]
		clientKey := tlsKeys.Data[v1alpha1.ClientPrivateKeyKey]
		caCert := tlsKeys.Data[v1alpha1.CACertKey]
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
		conf.IssueCA = fmt.Sprintf(pki.BrokerIssuerTemplate, cluster.Name)
		conf.IssueCAKind = "ClusterIssuer"
	}

	return conf, nil
}

func generateKafkaAddress(cluster *v1alpha1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s:%d", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
}
