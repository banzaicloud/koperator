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
	"fmt"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/certutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// A full PKI for Kafka

func (r *Reconciler) kafkapki() ([]runtime.Object, error) {
	rootCertMeta := templates.ObjectMeta(fmt.Sprintf(brokerCACertTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster)
	rootCertMeta.Namespace = "cert-manager"

	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.Create {
		// A self-signer for the CA Certificate
		selfsigner := &certv1.ClusterIssuer{
			ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerSelfSignerTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
			Spec: certv1.IssuerSpec{
				IssuerConfig: certv1.IssuerConfig{
					SelfSigned: &certv1.SelfSignedIssuer{},
				},
			},
		}
		controllerutil.SetControllerReference(r.KafkaCluster, selfsigner, r.Scheme)

		// The CA Certificate
		ca := &certv1.Certificate{
			ObjectMeta: rootCertMeta,
			Spec: certv1.CertificateSpec{
				SecretName: fmt.Sprintf(brokerCACertTemplate, r.KafkaCluster.Name),
				CommonName: fmt.Sprintf("kafkaca.%s.cluster.local", r.KafkaCluster.Namespace),
				IsCA:       true,
				IssuerRef: certv1.ObjectReference{
					Name: fmt.Sprintf(brokerSelfSignerTemplate, r.KafkaCluster.Name),
					Kind: "ClusterIssuer",
				},
			},
		}
		controllerutil.SetControllerReference(r.KafkaCluster, ca, r.Scheme)
		// A cluster issuer backed by the CA certificate - so it can provision secrets
		// for producers/consumers in other namespaces
		clusterissuer := &certv1.ClusterIssuer{
			ObjectMeta: templates.ObjectMeta(fmt.Sprintf(BrokerIssuerTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
			Spec: certv1.IssuerSpec{
				IssuerConfig: certv1.IssuerConfig{
					CA: &certv1.CAIssuer{
						SecretName: fmt.Sprintf(brokerCACertTemplate, r.KafkaCluster.Name),
					},
				},
			},
		}
		controllerutil.SetControllerReference(r.KafkaCluster, clusterissuer, r.Scheme)

		// The broker certificates
		brokerCert := &certv1.Certificate{
			ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerServerCertTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
			Spec: certv1.CertificateSpec{
				SecretName:  fmt.Sprintf(brokerServerCertTemplate, r.KafkaCluster.Name),
				KeyEncoding: certv1.PKCS8,
				CommonName:  getCommonName(r.KafkaCluster),
				DNSNames:    getDNSNames(r.KafkaCluster),
				IssuerRef: certv1.ObjectReference{
					Name: fmt.Sprintf(BrokerIssuerTemplate, r.KafkaCluster.Name),
					Kind: "ClusterIssuer",
				},
			},
		}
		controllerutil.SetControllerReference(r.KafkaCluster, brokerCert, r.Scheme)

		// And finally one for us so we can manage topics/users
		controllerCert := &certv1.Certificate{
			ObjectMeta: templates.ObjectMeta(fmt.Sprintf(BrokerControllerTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
			Spec: certv1.CertificateSpec{
				SecretName:  fmt.Sprintf(BrokerControllerTemplate, r.KafkaCluster.Name),
				KeyEncoding: certv1.PKCS8,
				CommonName:  fmt.Sprintf("%s-controller", r.KafkaCluster.Name),
				IssuerRef: certv1.ObjectReference{
					Name: fmt.Sprintf(BrokerIssuerTemplate, r.KafkaCluster.Name),
					Kind: "ClusterIssuer",
				},
			},
		}
		controllerutil.SetControllerReference(r.KafkaCluster, controllerCert, r.Scheme)

		return []runtime.Object{selfsigner, ca, clusterissuer, brokerCert, controllerCert}, nil

	}

	// If we aren't creating the secrets we need a cluster issuer made from the provided secret
	secret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: r.KafkaCluster.Namespace, Name: r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName}, secret)
	if err != nil {
		return []runtime.Object{}, err
	}
	caKey := secret.Data[banzaicloudv1alpha1.CAPrivateKeyKey]
	caCert := secret.Data[banzaicloudv1alpha1.CACertKey]

	caSecret := &corev1.Secret{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerCACertTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
		Data: map[string][]byte{
			banzaicloudv1alpha1.CoreCACertKey: caCert,
			corev1.TLSCertKey:                 caCert,
			corev1.TLSPrivateKeyKey:           caKey,
		},
	}
	controllerutil.SetControllerReference(r.KafkaCluster, caSecret, r.Scheme)

	clusterissuer := &certv1.ClusterIssuer{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(BrokerIssuerTemplate, r.KafkaCluster.Name), labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				CA: &certv1.CAIssuer{
					SecretName: fmt.Sprintf(brokerCACertTemplate, r.KafkaCluster.Name),
				},
			},
		},
	}
	controllerutil.SetControllerReference(r.KafkaCluster, clusterissuer, r.Scheme)

	return []runtime.Object{caSecret, clusterissuer}, nil

}

func (r *Reconciler) getBootstrapSSLSecret() (certs, passw *corev1.Secret, err error) {
	// get server (peer) certificate
	serverSecret := &corev1.Secret{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf(brokerServerCertTemplate, r.KafkaCluster.Name),
		Namespace: r.KafkaCluster.Namespace,
	}, serverSecret); err != nil {
		return
	}

	clientSecret := &corev1.Secret{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf(BrokerControllerTemplate, r.KafkaCluster.Name),
		Namespace: r.KafkaCluster.Namespace,
	}, clientSecret); err != nil {
		return
	}

	certs = &corev1.Secret{
		ObjectMeta: templates.ObjectMeta(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName, labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
		Data: map[string][]byte{
			banzaicloudv1alpha1.CACertKey:           serverSecret.Data[banzaicloudv1alpha1.CoreCACertKey],
			banzaicloudv1alpha1.PeerCertKey:         serverSecret.Data[corev1.TLSCertKey],
			banzaicloudv1alpha1.PeerPrivateKeyKey:   serverSecret.Data[corev1.TLSPrivateKeyKey],
			banzaicloudv1alpha1.ClientCertKey:       clientSecret.Data[corev1.TLSCertKey],
			banzaicloudv1alpha1.ClientPrivateKeyKey: clientSecret.Data[corev1.TLSPrivateKeyKey],
		},
	}

	passw = &corev1.Secret{
		ObjectMeta: templates.ObjectMeta(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.JKSPasswordName, labelsForKafkaPKI(r.KafkaCluster.Name), r.KafkaCluster),
		Data: map[string][]byte{
			banzaicloudv1alpha1.PasswordKey: certutil.GeneratePass(16),
		},
	}

	return
}

func getCommonName(cluster *banzaicloudv1alpha1.KafkaCluster) string {
	if cluster.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s.%s.svc.cluster.local", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace)
}

func getDNSNames(cluster *banzaicloudv1alpha1.KafkaCluster) (dnsNames []string) {
	dnsNames = make([]string, 0)
	for _, broker := range cluster.Spec.BrokerConfigs {
		if cluster.Spec.HeadlessServiceEnabled {
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local", cluster.Name, broker.Id, fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s-%d.%s.%s.svc", cluster.Name, broker.Id, fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s-%d.%s.%s", cluster.Name, broker.Id, fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		} else {
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s-%d.%s.svc.cluster.local", cluster.Name, broker.Id, cluster.Namespace))
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s-%d.%s.svc", cluster.Name, broker.Id, cluster.Namespace))
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s-%d.%s", cluster.Name, broker.Id, cluster.Namespace))
		}
	}
	if cluster.Spec.HeadlessServiceEnabled {
		dnsNames = append(dnsNames, getCommonName(cluster))
		dnsNames = append(dnsNames,
			fmt.Sprintf("%s.%s.svc", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		dnsNames = append(dnsNames,
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name), cluster.Namespace))
		dnsNames = append(dnsNames,
			fmt.Sprintf(kafka.HeadlessServiceTemplate, cluster.Name))
	} else {
		dnsNames = append(dnsNames, getCommonName(cluster))
		dnsNames = append(dnsNames,
			fmt.Sprintf("%s.%s.svc", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace))
		dnsNames = append(dnsNames,
			fmt.Sprintf("%s.%s", fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name), cluster.Namespace))
		dnsNames = append(dnsNames,
			fmt.Sprintf(kafka.AllBrokerServiceTemplate, cluster.Name))
	}
	return
}
