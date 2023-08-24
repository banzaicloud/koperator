// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"crypto/sha256"
	"crypto/tls"
	"flag"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/k8sutil"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
	"github.com/banzaicloud/koperator/pkg/util/kafka"
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
	// LegacyBrokerClusterIssuerTemplate is the template used earlier for broker issuer resources
	LegacyBrokerClusterIssuerTemplate = "%s-issuer"
	// BrokerControllerTemplate is the template used for operator certificate resources
	BrokerControllerTemplate = "%s-controller"
	// BrokerControllerFQDNTemplate is combined with the above and cluster namespace
	// to create a 'fake' full-name for the controller user
	BrokerControllerFQDNTemplate = "%s.%s.mgt.%s"
	// CAFQDNTemplate is the template used for the FQDN of a CA
	CAFQDNTemplate = "%s-ca.%s.cluster.local"
	// KafkaUserAnnotationName used in case of PKIbackend is k8s-csr to find the appropriate kafkauser in case of
	// signing request event
	KafkaUserAnnotationName = "banzaicloud.io/owner"
	// MaxCNLen specifies the number of chars that the longest common name can have
	MaxCNLen = 64
)

// NamespaceCertManager points to a namespace where cert-manager is located
var NamespaceCertManager string

func init() {
	flag.StringVar(&NamespaceCertManager, "cert-manager-namespace", "cert-manager", "The namespace where cert-manager is running")
}

// Manager is the main interface for objects performing PKI operations
type Manager interface {
	// ReconcilePKI ensures a PKI for a kafka cluster - should be idempotent.
	// This method should at least setup any issuer needed for user certificates
	// as well as broker/cruise-control secrets
	ReconcilePKI(ctx context.Context, externalHostnames map[string]v1beta1.ListenerStatusList) error

	// FinalizePKI performs any cleanup steps necessary for a PKI backend
	FinalizePKI(ctx context.Context) error

	// ReconcileUserCertificate ensures and returns a user certificate - should be idempotent
	ReconcileUserCertificate(
		ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme, clusterDomain string) (*UserCertificate, error)

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
	JKS         []byte
	Password    []byte
}

// GetDistinguishedName returns the Distinguished Name of a TLS certificate
func (u *UserCertificate) GetDistinguishedName() (string, error) {
	// cert has already been validated so we can assume no error
	cert, err := certutil.DecodeCertificate(u.Certificate)
	if err != nil {
		return "", err
	}
	return cert.Subject.String(), nil
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
func clusterDNSNames(cluster *v1beta1.KafkaCluster) []string {
	names := make([]string, 0)
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

		// Per Broker notation
		for _, broker := range cluster.Spec.Brokers {
			names = append(names,
				fmt.Sprintf("%s-%d.%s.svc.%s", cluster.Name, broker.Id, cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain()),
				fmt.Sprintf("%s-%d.%s.svc", cluster.Name, broker.Id, cluster.Namespace),
				fmt.Sprintf("*.%s-%d.%s", cluster.Name, broker.Id, cluster.Namespace),
			)
		}

		// Namespace notation
		names = append(names,
			fmt.Sprintf("*.%s.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace),
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace),
		)

		// service name only
		names = append(names,
			fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name))
	}
	return names
}

// LabelsForKafkaPKI returns kubernetes labels for a PKI object
func LabelsForKafkaPKI(name, namespace string) map[string]string {
	return map[string]string{v1beta1.AppLabelKey: "kafka", "kafka_issuer": fmt.Sprintf(BrokerClusterIssuerTemplate, namespace, name)}
}

// BrokerUserForCluster returns a KafkaUser CR for the broker certificates in a KafkaCluster
func BrokerUserForCluster(cluster *v1beta1.KafkaCluster, extListenerStatuses map[string]v1beta1.ListenerStatusList) *v1alpha1.KafkaUser {
	additionalHosts := make([]string, 0, len(extListenerStatuses))
	for _, listenerStatus := range extListenerStatuses {
		for _, status := range listenerStatus {
			host := strings.Split(status.Address, ":")[0]
			additionalHosts = append(additionalHosts, host)
		}
	}
	additionalHosts = sortAndDedupe(additionalHosts)
	return &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(EnsureValidCommonNameLen(GetCommonName(cluster)), LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster),
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: fmt.Sprintf(BrokerServerCertTemplate, cluster.Name),
			DNSNames:   append(GetInternalDNSNames(cluster), additionalHosts...),
			IncludeJKS: true,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}
}

func sortAndDedupe(hosts []string) []string {
	sort.Strings(hosts)

	uniqueHosts := make([]string, 0)
	lastHost := ""
	for _, host := range hosts {
		if lastHost != host {
			uniqueHosts = append(uniqueHosts, host)
		}
		lastHost = host
	}
	return uniqueHosts
}

// ControllerUserForCluster returns a KafkaUser CR for the controller/cc certificates in a KafkaCluster
func ControllerUserForCluster(cluster *v1beta1.KafkaCluster) *v1alpha1.KafkaUser {
	return &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(
			EnsureValidCommonNameLen(fmt.Sprintf(BrokerControllerFQDNTemplate, fmt.Sprintf(BrokerControllerTemplate, cluster.Name), cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain())),
			LabelsForKafkaPKI(cluster.Name, cluster.Namespace),
			cluster,
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

// EnsureControllerReference ensures that a KafkaUser owns a given Secret
func EnsureControllerReference(ctx context.Context, user *v1alpha1.KafkaUser,
	secret *corev1.Secret, scheme *runtime.Scheme, client client.Client) error {
	err := controllerutil.SetControllerReference(user, secret, scheme)
	if err != nil && !k8sutil.IsAlreadyOwnedError(err) {
		return errorfactory.New(errorfactory.InternalError{}, err, "error checking controller reference on user secret")
	} else if err == nil {
		if err = client.Update(ctx, secret); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "could not update secret with controller reference")
		}
	}
	return nil
}

// EnsureValidCommonNameLen ensures that the passed-in common name doesn't exceed the longest supported length
func EnsureValidCommonNameLen(s string) string {
	if len(s) > MaxCNLen {
		first := sha256.New()
		first.Write([]byte(s))
		// encode the hash to make sure it only consists of lower case alphanumeric characters to enforce RFC 1123
		return fmt.Sprintf("%x", first.Sum(nil))
	}
	return s
}
