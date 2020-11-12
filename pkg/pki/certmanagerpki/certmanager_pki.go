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

package certmanagerpki

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	certmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

func (c *certManager) FinalizePKI(ctx context.Context, logger logr.Logger) error {
	logger.Info("Removing cert-manager certificates and secrets")

	// Safety check that we are actually doing something
	if c.cluster.Spec.ListenersConfig.SSLSecrets == nil {
		return nil
	}

	if c.cluster.Spec.ListenersConfig.SSLSecrets.Create {
		// Names of our certificates and secrets
		objNames := []types.NamespacedName{
			{Name: fmt.Sprintf(pkicommon.BrokerServerCertTemplate, c.cluster.Name), Namespace: c.cluster.Namespace},
			{Name: fmt.Sprintf(pkicommon.BrokerControllerTemplate, c.cluster.Name), Namespace: c.cluster.Namespace},
		}
		if c.cluster.Spec.ListenersConfig.SSLSecrets.IssuerRef == nil {
			objNames = append(
				objNames,
				types.NamespacedName{Name: fmt.Sprintf(pkicommon.BrokerCACertTemplate, c.cluster.Name), Namespace: namespaceCertManager})

		}
		for _, obj := range objNames {
			// Delete the certificates first so we don't accidentally recreate the
			// secret after it gets deleted
			cert := &certv1.Certificate{}
			if err := c.client.Get(ctx, obj, cert); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				} else {
					return err
				}
			}
			if err := c.client.Delete(ctx, cert); err != nil {
				return err
			}

			// Might as well delete the secret and leave the controller reference earlier
			// as a safety belt
			secret := &corev1.Secret{}
			if err := c.client.Get(ctx, obj, secret); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				} else {
					return err
				}
			}
			if err := c.client.Delete(ctx, secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *certManager) ReconcilePKI(ctx context.Context, logger logr.Logger, scheme *runtime.Scheme, externalHostnames map[string]string) (err error) {
	logger.Info("Reconciling cert-manager PKI")

	resources, err := c.kafkapki(ctx, scheme, externalHostnames)
	if err != nil {
		return err
	}

	for _, o := range resources {
		if err := reconcile(ctx, logger, c.client, o, c.cluster); err != nil {
			return err
		}
	}

	return nil
}

func (c *certManager) kafkapki(ctx context.Context, scheme *runtime.Scheme, externalHostnames map[string]string) ([]runtime.Object, error) {
	sslConfig := c.cluster.Spec.ListenersConfig.SSLSecrets
	if sslConfig.Create {
		if sslConfig.IssuerRef == nil {
			return fullPKI(c.cluster, scheme, externalHostnames), nil
		}
		return userProvidedIssuerPKI(c.cluster, externalHostnames), nil
	}
	return userProvidedPKI(ctx, c.client, c.cluster, scheme, externalHostnames)
}

func userProvidedIssuerPKI(cluster *v1beta1.KafkaCluster, externalHostnames map[string]string) []runtime.Object {
	// No need to generate self-signed certs and issuers because the issuer is provided by user
	return []runtime.Object{
		// Broker "user"
		pkicommon.BrokerUserForCluster(cluster, externalHostnames),
		// Operator user
		pkicommon.ControllerUserForCluster(cluster),
	}
}

func fullPKI(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme, externalHostnames map[string]string) []runtime.Object {
	return []runtime.Object{
		// A self-signer for the CA Certificate
		selfSignerForCluster(cluster, scheme),
		// The CA Certificate
		caCertForCluster(cluster),
		// A cluster issuer backed by the CA certificate - so it can provision secrets
		// for producers/consumers in other namespaces
		mainIssuerForCluster(cluster),
		// Broker "user"
		pkicommon.BrokerUserForCluster(cluster, externalHostnames),
		// Operator user
		pkicommon.ControllerUserForCluster(cluster),
	}
}

func userProvidedPKI(
	ctx context.Context, client client.Client,
	cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme, externalHostnames map[string]string) ([]runtime.Object, error) {
	// If we aren't creating the secrets we need a cluster issuer made from the provided secret
	caSecret, err := caSecretForProvidedCert(ctx, client, cluster, scheme)
	if err != nil {
		return nil, err
	}
	return []runtime.Object{
		caSecret,
		mainIssuerForCluster(cluster),
		// The client/peer certificates in the secret will still work, however are not actually used.
		// This will also make sure that if the peerCert/clientCert provided are invalid
		// a valid one will still be used with the provided CA.
		//
		// TODO: (tinyzimmer) - Would it be better to allow the KafkaUser to take a user-provided cert/key combination?
		// It would have to be validated first as signed by whatever the CA is - probably via a webhook.
		pkicommon.BrokerUserForCluster(cluster, externalHostnames),
		pkicommon.ControllerUserForCluster(cluster),
	}, nil
}

func caSecretForProvidedCert(ctx context.Context, client client.Client, cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = errorfactory.New(errorfactory.ResourceNotReady{}, err, "could not find provided tls secret")
		} else {
			err = errorfactory.New(errorfactory.APIFailure{}, err, "could not lookup provided tls secret")
		}
		return nil, err
	}

	caKey := secret.Data[v1alpha1.CAPrivateKeyKey]
	caCert := secret.Data[v1alpha1.CACertKey]

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name),
			Namespace: namespaceCertManager,
			Labels:    pkicommon.LabelsForKafkaPKI(cluster.Name, cluster.Namespace),
		},
		Data: map[string][]byte{
			v1alpha1.CoreCACertKey:  caCert,
			corev1.TLSCertKey:       caCert,
			corev1.TLSPrivateKeyKey: caKey,
		},
	}
	return caSecret, nil
}

func selfSignerForCluster(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) *certv1.ClusterIssuer {
	selfsignerMeta := templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerSelfSignerTemplate, cluster.Name),
		pkicommon.LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster)
	selfsignerMeta.Namespace = metav1.NamespaceAll
	selfsigner := &certv1.ClusterIssuer{
		ObjectMeta: selfsignerMeta,
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				SelfSigned: &certv1.SelfSignedIssuer{},
			},
		},
	}
	controllerutil.SetControllerReference(cluster, selfsigner, scheme)
	return selfsigner
}

func caCertForCluster(cluster *v1beta1.KafkaCluster) *certv1.Certificate {
	return &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name),
			Namespace: namespaceCertManager,
			Labels:    pkicommon.LabelsForKafkaPKI(cluster.Name, cluster.Namespace),
		},
		Spec: certv1.CertificateSpec{
			SecretName: fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name),
			CommonName: fmt.Sprintf(pkicommon.CAFQDNTemplate, cluster.Name, cluster.Namespace),
			IsCA:       true,
			IssuerRef: certmeta.ObjectReference{
				Name: fmt.Sprintf(pkicommon.BrokerSelfSignerTemplate, cluster.Name),
				Kind: certv1.ClusterIssuerKind,
			},
		},
	}
}

func mainIssuerForCluster(cluster *v1beta1.KafkaCluster) *certv1.ClusterIssuer {
	clusterIssuerMeta := templates.ObjectMeta(
		fmt.Sprintf(pkicommon.BrokerClusterIssuerTemplate, cluster.Namespace, cluster.Name),
		pkicommon.LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster)
	clusterIssuerMeta.Namespace = metav1.NamespaceAll
	issuer := &certv1.ClusterIssuer{
		ObjectMeta: clusterIssuerMeta,
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				CA: &certv1.CAIssuer{
					SecretName: fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name),
				},
			},
		},
	}
	return issuer
}
