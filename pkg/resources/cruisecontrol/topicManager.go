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

package cruisecontrol

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"emperror.dev/errors"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"

	"github.com/Shopify/sarama"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func generateCCTopic(cluster *banzaicloudv1alpha1.KafkaCluster, client client.Client, log logr.Logger) error {

	conf := sarama.NewConfig()
	conf.Version = sarama.V2_1_0_0

	if cluster.Spec.ListenersConfig.SSLSecrets != nil {
		tlsKeys := &corev1.Secret{}
		err := client.Get(context.TODO(), types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName}, tlsKeys)
		if err != nil {
			return emperror.Wrap(err, "could not get TLS secret to create CC topic")
		}
		clientCert := tlsKeys.Data["clientCert"]
		clientKey := tlsKeys.Data["clientKey"]
		caCert := tlsKeys.Data["caCert"]

		x509ClientCert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			return err
		}

		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(caCert)

		t := &tls.Config{
			Certificates: []tls.Certificate{x509ClientCert},
			RootCAs:      rootCAs,
		}
		conf.Net.TLS.Enable = true
		conf.Net.TLS.Config = t
	}

	adminClient, err := sarama.NewClusterAdmin([]string{generateKafkaAddress(cluster)}, conf)
	if err != nil {
		return emperror.Wrap(err, "Error while creating admin client to create CC topic")
	}
	defer func() { _ = adminClient.Close() }()

	err = adminClient.CreateTopic("__CruiseControlMetrics", &sarama.TopicDetail{
		NumPartitions:     12,
		ReplicationFactor: 3,
	}, false)

	var tError *sarama.TopicError

	if err != nil && !(errors.As(err, &tError) && tError.Err == sarama.ErrTopicAlreadyExists) {
		return emperror.Wrap(err, "Error while creating CC topic")
	}

	return nil
}

func generateKafkaAddress(cluster *banzaicloudv1alpha1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s:%d", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
}
