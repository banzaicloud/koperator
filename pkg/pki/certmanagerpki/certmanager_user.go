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

	"emperror.dev/errors"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/resources/templates"

	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"

	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	certmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// FinalizeUserCertificate for cert-manager backend auto returns because controller references handle cleanup
func (c *certManager) FinalizeUserCertificate(_ context.Context, _ *v1alpha1.KafkaUser) (err error) {
	return
}

// ReconcileUserCertificate ensures a certificate/secret combination using cert-manager
func (c *certManager) ReconcileUserCertificate(
	ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme, clusterDomain string) (*pkicommon.UserCertificate, error) {
	var err error
	var secret *corev1.Secret
	// See if we have an existing certificate for this user already
	_, err = c.getUserCertificate(ctx, user)

	if err != nil && apierrors.IsNotFound(err) {
		// the certificate does not exist, let's make one
		// check if jks is required and create password for it
		if user.Spec.IncludeJKS {
			if err := c.injectJKSPassword(ctx, user); err != nil {
				return nil, err
			}
		}
		cert := c.clusterCertificateForUser(user, clusterDomain)
		if err = c.client.Create(ctx, cert); err != nil {
			return nil, errorfactory.New(errorfactory.APIFailure{}, err, "could not create user certificate")
		}
	} else if err != nil {
		// API failure, requeue
		return nil, errorfactory.New(errorfactory.APIFailure{}, err, "failed looking up user certificate")
	}

	// Get the secret created from the certificate
	secret, err = c.getUserSecret(ctx, user)
	if err != nil {
		return nil, err
	}

	// Ensure controller reference on user secret
	if err = pkicommon.EnsureControllerReference(ctx, user, secret, scheme, c.client); err != nil {
		return nil, err
	}

	return &pkicommon.UserCertificate{
		CA:          secret.Data[v1alpha1.CoreCACertKey],
		Certificate: secret.Data[corev1.TLSCertKey],
		Key:         secret.Data[corev1.TLSPrivateKeyKey],
	}, nil
}

// injectJKSPassword ensures that a secret contains JKS password when requested
func (c *certManager) injectJKSPassword(ctx context.Context, user *v1alpha1.KafkaUser) error {
	var err error
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Spec.SecretName,
			Namespace: user.Namespace,
		},
		Data: map[string][]byte{},
	}
	secret, err = certutil.EnsureSecretPassJKS(secret)
	if err != nil {
		return errorfactory.New(errorfactory.InternalError{}, err, "could not inject secret with jks password")
	}
	if err = c.client.Create(ctx, secret); err != nil {
		return errorfactory.New(errorfactory.APIFailure{}, err, "could not create secret with jks password")
	}

	return nil
}

// getUserCertificate fetches the cert-manager Certificate for a user
func (c *certManager) getUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser) (*certv1.Certificate, error) {
	cert := &certv1.Certificate{}
	err := c.client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, cert)
	return cert, err
}

// getUserSecret fetches the secret created from a cert-manager Certificate for a user
func (c *certManager) getUserSecret(ctx context.Context, user *v1alpha1.KafkaUser) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := c.client.Get(ctx, types.NamespacedName{Name: user.Spec.SecretName, Namespace: user.Namespace}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return secret, errorfactory.New(errorfactory.ResourceNotReady{}, err, "user secret not ready")
		}
		return secret, errorfactory.New(errorfactory.APIFailure{}, err, "failed to get user secret")
	}
	if user.Spec.IncludeJKS {
		if len(secret.Data) != 6 {
			return secret, errorfactory.New(errorfactory.ResourceNotReady{}, err, "user secret not populated yet")
		}
	} else {
		if len(secret.Data) != 3 {
			return secret, errorfactory.New(errorfactory.ResourceNotReady{}, err, "user secret not populated yet")
		}
	}

	for _, v := range secret.Data {
		if len(v) == 0 {
			return secret, errorfactory.New(errorfactory.ResourceNotReady{},
				errors.New("not all secret value populated"), "secret is not ready")
		}
	}

	return secret, nil
}

// clusterCertificateForUser generates a Certificate object for a KafkaUser
func (c *certManager) clusterCertificateForUser(
	user *v1alpha1.KafkaUser, clusterDomain string) *certv1.Certificate {
	caName, caKind := c.getCA(user)
	cert := &certv1.Certificate{
		ObjectMeta: templates.ObjectMetaWithKafkaUserOwnerAndWithoutLabels(user.GetName(), user),
		Spec: certv1.CertificateSpec{
			SecretName: user.Spec.SecretName,
			PrivateKey: &certv1.CertificatePrivateKey{
				Encoding: certv1.PKCS8,
			},
			CommonName: user.GetName(),
			URIs:       []string{fmt.Sprintf(spiffeIdTemplate, clusterDomain, user.GetNamespace(), user.GetName())},
			Usages:     []certv1.KeyUsage{certv1.UsageClientAuth, certv1.UsageServerAuth},
			IssuerRef: certmeta.ObjectReference{
				Name: caName,
				Kind: caKind,
			},
		},
	}
	if user.Spec.IncludeJKS {
		cert.Spec.Keystores = &certv1.CertificateKeystores{
			JKS: &certv1.JKSKeystore{
				Create: true,
				PasswordSecretRef: certmeta.SecretKeySelector{
					LocalObjectReference: certmeta.LocalObjectReference{
						Name: user.Spec.SecretName,
					},
					Key: v1alpha1.PasswordKey,
				},
			},
		}
	}
	if user.Spec.DNSNames != nil && len(user.Spec.DNSNames) > 0 {
		cert.Spec.DNSNames = user.Spec.DNSNames
	}
	return cert
}

// getCA returns the CA name/kind for the KafkaCluster
func (c *certManager) getCA(user *v1alpha1.KafkaUser) (caName, caKind string) {
	var issuerRef *certmeta.ObjectReference
	if user.Spec.PKIBackendSpec != nil {
		issuerRef = user.Spec.PKIBackendSpec.IssuerRef
	} else {
		issuerRef = c.cluster.Spec.ListenersConfig.SSLSecrets.IssuerRef
	}
	if issuerRef != nil {
		caName = issuerRef.Name
		caKind = issuerRef.Kind
	} else {
		caKind = certv1.ClusterIssuerKind
		caName = fmt.Sprintf(pkicommon.BrokerClusterIssuerTemplate,
			c.cluster.Namespace, c.cluster.Name)
	}
	//Check if the new cluster issuer with namespaced name exists if not fall back to original one
	var issuer *certv1.ClusterIssuer
	err := c.client.Get(context.Background(), types.NamespacedName{Namespace: metav1.NamespaceAll, Name: caName}, issuer)
	if err != nil && apierrors.IsNotFound(err) {
		caName = fmt.Sprintf(pkicommon.LegacyBrokerClusterIssuerTemplate, c.cluster.Name)
	}
	return
}
