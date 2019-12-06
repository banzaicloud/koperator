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

package internalpki

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkiutil "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

// createNewCA creates a new CA for a given cluster, and stores it in a kubernetes secret
func createNewCA(ctx context.Context, client client.Client, cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) error {
	caSecret, err := newClusterCASecret(cluster, scheme)
	if err != nil {
		return err
	}
	return client.Create(ctx, caSecret)
}

// newClusterCASecret generates a new CA certificate and returns a Kubernetes secret object
// with PEM encoded fields.
func newClusterCASecret(cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme) (*corev1.Secret, error) {
	// generate a new CA certificate using cluster name as Organization
	ca := newCA(cluster.Name)
	// generate a new RSA key
	caPrivKey, err := newKey()
	if err != nil {
		return nil, err
	}
	// Sign the certificate with the key
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}
	// PEM encode the certificate and key and return a secret
	_, caPEM, caKeyPEM, err := encodeToPEM(ca, caBytes, caPrivKey)
	if err != nil {
		return nil, err
	}
	caSecret := &corev1.Secret{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(pkiutil.BrokerCACertTemplate, cluster.Name),
			pkiutil.LabelsForKafkaPKI(cluster.Name),
			cluster,
		),
		Data: pkiutil.SecretDataForCert(caPEM, caPEM, caKeyPEM),
	}
	return caSecret, nil
}

// getCA retrieves the CA certificate and signing key for a given KafkaCluster
func getCA(ctx context.Context, client client.Client, cluster *v1beta1.KafkaCluster) (*x509.Certificate, *rsa.PrivateKey, error) {
	o := types.NamespacedName{
		Name:      fmt.Sprintf(pkiutil.BrokerCACertTemplate, cluster.Name),
		Namespace: cluster.Namespace,
	}
	// fetch the secret
	caSecret := &corev1.Secret{}
	if err := client.Get(ctx, o, caSecret); err != nil {
		return nil, nil, err
	}
	// make sure we still have the required fields
	if err := verifyCASecretFields(caSecret); err != nil {
		return nil, nil, err
	}
	// decode the PEM objects
	cert, err := certutil.DecodeCertificate(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return nil, nil, err
	}
	keyData, _ := pem.Decode(caSecret.Data[corev1.TLSPrivateKeyKey])
	privKey, err := x509.ParsePKCS8PrivateKey(keyData.Bytes)
	if err != nil {
		return nil, nil, err
	}
	// return the cert object and signing key
	return cert, privKey.(*rsa.PrivateKey), nil
}

// verifyCASecretFields checks to make sure the certificate and key are present
// in a secret and returns an error if either are missing.
func verifyCASecretFields(secret *corev1.Secret) error {
	for _, key := range []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey} {
		if _, ok := secret.Data[key]; !ok {
			return fmt.Errorf("No key %s in secret", key)
		}
	}
	return nil
}
