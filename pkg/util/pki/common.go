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
	"context"
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
	// BrokerClusterIssuerTemplate is the template used for broker issuer resources
	BrokerClusterIssuerTemplate = "%s-%s-issuer"
	// BrokerControllerTemplate is the template used for operator certificate resources
	BrokerControllerTemplate = "%s-controller"
	// BrokerControllerFQDNTemplate is combined with the above and cluster namespace
	// to create a 'fake' full-name for the controller user
	BrokerControllerFQDNTemplate = "%s.%s.mgt.%s"
	// CAIntermediateTemplate is the template used for intermediate CA resources
	CAIntermediateTemplate = "%s-intermediate.%s.%s"
	// CAFQDNTemplate is the template used for the FQDN of a CA
	CAFQDNTemplate = "%s-ca.%s.cluster.local"
)

// Manager is the main interface for objects performing PKI operations
type Manager interface {
	// ReconcilePKI ensures a PKI for a kafka cluster - should be idempotent.
	// This method should at least setup any issuer needed for user certificates
	// as well as broker/cruise-control secrets
	ReconcilePKI(ctx context.Context, logger logr.Logger, scheme *runtime.Scheme, externalHostnames []string) error

	// FinalizePKI performs any cleanup steps necessary for a PKI backend
	FinalizePKI(ctx context.Context, logger logr.Logger) error

	// ReconcileUserCertificate ensures and returns a user certificate - should be idempotent
	ReconcileUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme) (*UserCertificate, error)

	// FinalizeUserCertificate removes/revokes a user certificate
	FinalizeUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser) error

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

// GetInternalDNSNames returns all potential DNS names for a kafka cluster - including brokers
func GetInternalDNSNames(cluster *v1beta1.KafkaCluster) (dnsNames []string) {
	dnsNames = make([]string, 0)
	dnsNames = append(dnsNames, clusterDNSNames(cluster)...)
	return
}

// GetCommonName returns the full FQDN for the internal Kafka listener
func GetCommonName(cluster *v1beta1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s.svc.%s", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain())
	}
	return fmt.Sprintf("%s.%s.svc.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain())
}

// clusterDNSNames returns all the possible DNS Names for a Kafka Cluster
func clusterDNSNames(cluster *v1beta1.KafkaCluster) (names []string) {
	names = make([]string, 0)
	if cluster.Spec.HeadlessServiceEnabled {
		// FQDN
		names = append(names, fmt.Sprintf("*.%s", GetCommonName(cluster)))
		names = append(names, GetCommonName(cluster))

		// SVC notation
		names = append(names,
			fmt.Sprintf("*.%s.%s.svc", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace),
			fmt.Sprintf("%s.%s.svc", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace),
		)

		// Namespace notation
		names = append(names,
			fmt.Sprintf("*.%s.%s", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace),
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace),
		)

		// service name only
		names = append(names,
			fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name))
	} else {
		// FQDN
		names = append(names, fmt.Sprintf("*.%s", GetCommonName(cluster)))
		names = append(names, GetCommonName(cluster))

		// SVC notation
		names = append(names,
			fmt.Sprintf("*.%s.%s.svc", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace),
			fmt.Sprintf("%s.%s.svc", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace),
		)

		// Namespace notation
		names = append(names,
			fmt.Sprintf("*.%s.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace),
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace),
		)

		// service name only
		names = append(names,
			fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name))
	}
	return
}

// LabelsForKafkaPKI returns kubernetes labels for a PKI object
func LabelsForKafkaPKI(name, namespace string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_issuer": fmt.Sprintf(BrokerClusterIssuerTemplate, name, namespace)}
}

// BrokerUserForCluster returns a KafkaUser CR for the broker certificates in a KafkaCluster
func BrokerUserForCluster(cluster *v1beta1.KafkaCluster, additionalHostnames []string) *v1alpha1.KafkaUser {
	return &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(GetCommonName(cluster), LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster),
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: fmt.Sprintf(BrokerServerCertTemplate, cluster.Name),
			DNSNames:   append(GetInternalDNSNames(cluster), additionalHostnames...),
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
			fmt.Sprintf(BrokerControllerFQDNTemplate,
				fmt.Sprintf(BrokerControllerTemplate, cluster.Name), cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain()),
			LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster,
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
