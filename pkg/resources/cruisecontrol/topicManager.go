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
	"time"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/goph/emperror"
	kafkaGo "github.com/segmentio/kafka-go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func generateCCTopic(cluster *banzaicloudv1alpha1.KafkaCluster, client client.Client) error {

	dialer := &kafkaGo.Dialer{}

	if cluster.Spec.ListenersConfig.SSLSecrets != nil {
		tlsKeys := &corev1.Secret{}
		err := client.Get(context.TODO(), types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName}, tlsKeys)
		if err != nil {
			return emperror.Wrap(err, "could not get TLS secret to create CC topic")
		}
		serverCert := tlsKeys.Data["serverCert"]
		serverKey := tlsKeys.Data["serverKey"]
		caCert := tlsKeys.Data["caCert"]
		x509ServerCert, err := tls.X509KeyPair(serverCert, serverKey)
		if err != nil {
			return err
		}
		rootCAs := x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(caCert)

		dialer = &kafkaGo.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS: &tls.Config{
				Certificates: []tls.Certificate{x509ServerCert},
				RootCAs:      rootCAs,
			},
		}
	} else {
		dialer = &kafkaGo.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		}
	}
	conn, err := dialer.Dial("tcp",
		generateKafkaAddress(cluster))
	if err != nil {
		return emperror.Wrap(err, "could not create topic for CC because kafka is unavailable")
	}
	defer conn.Close()
	tConfig := kafkaGo.TopicConfig{
		Topic:             "__CruiseControlMetrics",
		NumPartitions:     12,
		ReplicationFactor: 3,
	}

	err = conn.CreateTopics(tConfig)
	if err != nil {
		return emperror.Wrap(err, "could not create topic for CC")
	}
	return nil
}

func generateKafkaAddress(cluster *banzaicloudv1alpha1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s:%d", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort)
}
