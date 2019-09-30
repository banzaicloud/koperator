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

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const namespaceCertManager = "cert-manager"

func (c *certManager) FinalizePKI(logger logr.Logger) error {
	logger.Info("Removing cert-manager certificates and secrets")

	// Safety check that we are actually doing something
	if c.cluster.Spec.ListenersConfig.SSLSecrets == nil {
		return nil
	}

	if c.cluster.Spec.ListenersConfig.SSLSecrets.Create {
		// Names of our certificates and secrets
		objNames := []types.NamespacedName{
			{Name: fmt.Sprintf(pkicommon.BrokerCACertTemplate, c.cluster.Name), Namespace: "cert-manager"},
			{Name: fmt.Sprintf(pkicommon.BrokerServerCertTemplate, c.cluster.Name), Namespace: c.cluster.Namespace},
			{Name: fmt.Sprintf(pkicommon.BrokerControllerTemplate, c.cluster.Name), Namespace: c.cluster.Namespace},
		}
		for _, obj := range objNames {
			// Delete the certificates first so we don't accidentally recreate the
			// secret after it gets deleted
			cert := &certv1.Certificate{}
			if err := c.client.Get(context.TODO(), obj, cert); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				} else {
					return err
				}
			}
			if err := c.client.Delete(context.TODO(), cert); err != nil {
				return err
			}

			// Might as well delete the secret and leave the controller reference earlier
			// as a safety belt
			secret := &corev1.Secret{}
			if err := c.client.Get(context.TODO(), obj, secret); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				} else {
					return err
				}
			}
			if err := c.client.Delete(context.TODO(), secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *certManager) ReconcilePKI(logger logr.Logger, scheme *runtime.Scheme) (err error) {
	logger.Info("Reconciling cert-manager PKI")

	resources, err := c.kafkapki(scheme)
	if err != nil {
		return err
	}

	for _, o := range resources {
		if err := reconcile(logger, c.client, o, c.cluster); err != nil {
			return err
		}
	}

	if c.cluster.Spec.ListenersConfig.SSLSecrets.Create {
		// Grab ownership of CA secret for garbage collection
		if err := ensureCASecretOwnership(c.client, c.cluster, scheme); err != nil {
			return err
		}
	}

	return nil
}

func (c *certManager) kafkapki(scheme *runtime.Scheme) ([]runtime.Object, error) {
	if c.cluster.Spec.ListenersConfig.SSLSecrets.Create {
		return fullPKI(c.cluster, scheme), nil
	}
	return userProvidedPKI(c.client, c.cluster, scheme)
}

func fullPKI(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) []runtime.Object {
	return []runtime.Object{
		// A self-signer for the CA Certificate
		selfSignerForCluster(cluster, scheme),
		// The CA Certificate
		caCertForCluster(cluster, scheme),
		// A cluster issuer backed by the CA certificate - so it can provision secrets
		// for producers/consumers in other namespaces
		mainIssuerForCluster(cluster, scheme),
		// Broker "user"
		pkicommon.BrokerUserForCluster(cluster),
		// Operator user
		pkicommon.ControllerUserForCluster(cluster),
	}
}

func userProvidedPKI(client client.Client, cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) ([]runtime.Object, error) {
	// If we aren't creating the secrets we need a cluster issuer made from the provided secret
	caSecret, err := caSecretForProvidedCert(client, cluster, scheme)
	if err != nil {
		return nil, err
	}
	return []runtime.Object{
		caSecret,
		mainIssuerForCluster(cluster, scheme),
		// The client/peer certificates in the secret will still work, however are not actually used.
		// This will also make sure that if the peerCert/clientCert provided are invalid
		// a valid one will still be used with the provided CA.
		//
		// TODO: (tinyzimmer) - Would it be better to allow the KafkaUser to take a user-provided cert/key combination?
		// It would have to be validated first as signed by whatever the CA is - probably via a webhook.
		pkicommon.BrokerUserForCluster(cluster),
		pkicommon.ControllerUserForCluster(cluster),
	}, nil
}

func ensureCASecretOwnership(client client.Client, cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) error {
	secret := &corev1.Secret{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name),
		Namespace: namespaceCertManager,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return errorfactory.New(errorfactory.ResourceNotReady{}, err, "pki secret not ready")
		}
		return errorfactory.New(errorfactory.APIFailure{}, err, "could not fetch pki secret")
	}
	if err := controllerutil.SetControllerReference(cluster, secret, scheme); err != nil {
		if k8sutil.IsAlreadyOwnedError(err) {
			return nil
		}
		return errorfactory.New(errorfactory.InternalError{}, err, "failed to set controller reference on secret")
	}
	if err := client.Update(context.TODO(), secret); err != nil {
		return errorfactory.New(errorfactory.APIFailure{}, err, "failed to set controller reference on secret")
	}
	return nil
}

func caSecretForProvidedCert(client client.Client, cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName}, secret)
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
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name), pkicommon.LabelsForKafkaPKI(cluster.Name), cluster),
		Data: map[string][]byte{
			v1alpha1.CoreCACertKey:  caCert,
			corev1.TLSCertKey:       caCert,
			corev1.TLSPrivateKeyKey: caKey,
		},
	}
	controllerutil.SetControllerReference(cluster, caSecret, scheme)
	return caSecret, nil
}

func selfSignerForCluster(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) *certv1.ClusterIssuer {
	selfsignerMeta := templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerSelfSignerTemplate, cluster.Name), pkicommon.LabelsForKafkaPKI(cluster.Name), cluster)
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

func caCertForCluster(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) *certv1.Certificate {
	rootCertMeta := templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name), pkicommon.LabelsForKafkaPKI(cluster.Name), cluster)
	rootCertMeta.Namespace = namespaceCertManager
	ca := &certv1.Certificate{
		ObjectMeta: rootCertMeta,
		Spec: certv1.CertificateSpec{
			SecretName: fmt.Sprintf(pkicommon.BrokerCACertTemplate, cluster.Name),
			CommonName: fmt.Sprintf(pkicommon.CAFQDNTemplate, cluster.Name, cluster.Namespace),
			IsCA:       true,
			IssuerRef: certv1.ObjectReference{
				Name: fmt.Sprintf(pkicommon.BrokerSelfSignerTemplate, cluster.Name),
				Kind: certv1.ClusterIssuerKind,
			},
		},
	}
	controllerutil.SetControllerReference(cluster, ca, scheme)
	return ca
}

func mainIssuerForCluster(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) *certv1.ClusterIssuer {
	clusterIssuerMeta := templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerIssuerTemplate, cluster.Name), pkicommon.LabelsForKafkaPKI(cluster.Name), cluster)
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
	controllerutil.SetControllerReference(cluster, issuer, scheme)
	return issuer
}
