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

package pki

import (
	"crypto/tls"
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	"github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// BrokerSelfSignerTemplate is the template used for self-signer resources
	BrokerSelfSignerTemplate = "%s-self-signer"
	// BrokerCACertTemplate is the template used for CA certificate resources
	BrokerCACertTemplate = "%s-ca-certificate"
	// BrokerServerCertTemplate is the template used for broker certificate resources
	BrokerServerCertTemplate = "%s-server-certificate"
	// BrokerIssuerTemplate is the template used for broker issuer resources
	BrokerIssuerTemplate = "%s-issuer"
	// BrokerControllerTemplate is the template used for operator certificate resources
	BrokerControllerTemplate = "%s-controller"
	// CAIntermediateTemplate is the template used for intermediate CA resources
	CAIntermediateTemplate = "%s-intermediate.%s.cluster.local"
	// CAFQDNTemplate is the template used for the FQDN of a CA
	CAFQDNTemplate = "%s-ca.%s.cluster.local"
)

// PKIManager is the main interface for performing PKI operations
type PKIManager interface {
	// ReconcilePKI ensures a PKI for a kafka cluster - should be idempotent.
	// This method should at least setup any issuer needed for user certificates
	ReconcilePKI(logger logr.Logger, scheme *runtime.Scheme) error

	// FinalizePKI performs any cleanup steps necessary for a PKI backend
	FinalizePKI(logger logr.Logger) error

	// ReconcileUserCertificate ensures and returns a user certificate - should be idempotent
	ReconcileUserCertificate(user *v1alpha1.KafkaUser, scheme *runtime.Scheme) (*UserCertificate, error)

	// FinalizeUserCertificate removes/revokes a user certificate
	FinalizeUserCertificate(user *v1alpha1.KafkaUser) error

	// GetControllerTLSConfig retrieves a TLS configuration for a controller kafka client
	GetControllerTLSConfig() (*tls.Config, error)
}

// UserCertificate is a struct representing the key components of a user TLS certificate
// for use across operations from other packages and internally.
type UserCertificate struct {
	CA          []byte
	Certificate []byte
	Key         []byte

	// Serial is used by vault backend for certificate revocations
	Serial string
	// jks and password are used by vault backend for passing jks info between itself
	// the cert-manager backend passes it through the k8s secret
	JKS      []byte
	Password []byte
}

// DN returns the Distinguished Name of a TLS certificate
func (u *UserCertificate) DN() string {
	// cert has already been validated so we can assume no error
	cert, _ := certutil.DecodeCertificate(u.Certificate)
	return cert.Subject.String()
}

// GetDNSNames returns all potential DNS names for a kafka cluster - including brokers
func GetDNSNames(cluster *v1beta1.KafkaCluster) (dnsNames []string) {
	dnsNames = make([]string, 0)
	for _, broker := range cluster.Spec.Brokers {
		dnsNames = append(dnsNames, brokerDNSNames(cluster, broker)...)
	}
	dnsNames = append(dnsNames, clusterDNSNames(cluster)...)
	return
}

// GetCommonName returns the full FQDN for the internal Kafka listener
func GetCommonName(cluster *v1beta1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s.svc.cluster.local", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace)
}

// brokerDNSNames returns all the possible DNS Names for a Kafka Broker
func brokerDNSNames(cluster *v1beta1.KafkaCluster, broker v1beta1.Broker) (names []string) {
	names = make([]string, 0)
	if cluster.Spec.HeadlessServiceEnabled {
		names = append(names,
			fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local", cluster.Name, broker.Id, fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		names = append(names,
			fmt.Sprintf("%s-%d.%s.%s.svc", cluster.Name, broker.Id, fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		names = append(names,
			fmt.Sprintf("%s-%d.%s.%s", cluster.Name, broker.Id, fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
	} else {
		names = append(names,
			fmt.Sprintf("%s-%d.%s.svc.cluster.local", cluster.Name, broker.Id, cluster.Namespace))
		names = append(names,
			fmt.Sprintf("%s-%d.%s.svc", cluster.Name, broker.Id, cluster.Namespace))
		names = append(names,
			fmt.Sprintf("%s-%d.%s", cluster.Name, broker.Id, cluster.Namespace))
	}
	return
}

// clusterDNSNames returns all the possible DNS Names for a Kafka Cluster
func clusterDNSNames(cluster *v1beta1.KafkaCluster) (names []string) {
	names = make([]string, 0)
	if cluster.Spec.HeadlessServiceEnabled {
		names = append(names, GetCommonName(cluster))
		names = append(names,
			fmt.Sprintf("%s.%s.svc", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		names = append(names,
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		names = append(names,
			fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name))
	} else {
		names = append(names, GetCommonName(cluster))
		names = append(names,
			fmt.Sprintf("%s.%s.svc", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace))
		names = append(names,
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace))
		names = append(names,
			fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name))
	}
	return
}

// LabelsForKafkaPKI returns kubernetes labels for a PKI object
func LabelsForKafkaPKI(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_issuer": fmt.Sprintf(BrokerIssuerTemplate, name)}
}

// BrokerUserForCluster returns a KafkaUser CR for the broker certificates in a KafkaCluster
func BrokerUserForCluster(cluster *v1beta1.KafkaCluster) *v1alpha1.KafkaUser {
	return &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(GetCommonName(cluster), LabelsForKafkaPKI(cluster.Name), cluster),
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: fmt.Sprintf(BrokerServerCertTemplate, cluster.Name),
			DNSNames:   GetDNSNames(cluster),
			IncludeJKS: true,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}
}

// ControllerUserForCluster returns a KafkaUser CR for the controller/cc certificates in a KafkaCluster
func ControllerUserForCluster(cluster *v1beta1.KafkaCluster) *v1alpha1.KafkaUser {
	return &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf("%s.%s.mgt.cluster.local", fmt.Sprintf(BrokerControllerTemplate, cluster.Name), cluster.Namespace),
			LabelsForKafkaPKI(cluster.Name), cluster,
		),
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: fmt.Sprintf(BrokerControllerTemplate, cluster.Name),
			IncludeJKS: true,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}
}
