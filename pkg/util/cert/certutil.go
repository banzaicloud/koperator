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

package cert

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"

	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	"github.com/pavel-v-chernykh/keystore-go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
)

// passChars are the characters used when generating passwords
var passChars []rune = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789")

// DecodeKey will take a PEM encoded Private Key and convert to raw der bytes
func DecodeKey(raw []byte) (parsedKey []byte, err error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		err = errors.New("failed to decode PEM data")
		return
	}
	var keytype certv1.PrivateKeyEncoding
	var key interface{}
	if key, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
		if key, err = x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
			return
		}
		keytype = certv1.PKCS8
	} else {
		keytype = certv1.PKCS1
	}
	rsaKey := key.(*rsa.PrivateKey)
	if keytype == certv1.PKCS1 {
		parsedKey = x509.MarshalPKCS1PrivateKey(rsaKey)
	} else {
		parsedKey, _ = x509.MarshalPKCS8PrivateKey(rsaKey)
	}
	return
}

// DecodeCertificate returns an x509.Certificate for a PEM encoded certificate
func DecodeCertificate(raw []byte) (cert *x509.Certificate, err error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		err = errors.New("Failed to decode x509 certificate from PEM")
		return
	}
	cert, err = x509.ParseCertificate(block.Bytes)
	return
}

// GeneratePass generates a random password
func GeneratePass(length int) (passw []byte) {
	mathrand.Seed(time.Now().UnixNano())
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(passChars[mathrand.Intn(len(passChars))])
	}
	passw = []byte(b.String())
	return
}

// EnsureSecretPassJKS ensures a JKS password is present in a certificate secret
func EnsureSecretPassJKS(secret *corev1.Secret) (injected *corev1.Secret, err error) {
	// If the JKS Pass is already present - return
	if _, ok := secret.Data[v1alpha1.PasswordKey]; ok {
		return secret, nil
	}

	injected = secret.DeepCopy()
	injected.Data[v1alpha1.PasswordKey] = GeneratePass(16)
	return
}

// GenerateJKS creates a JKS with a random password from a client cert/key combination
func GenerateJKS(clientCert, clientKey, clientCA []byte) (out, passw []byte, err error) {
	cert, err := DecodeCertificate(clientCert)
	if err != nil {
		return
	}

	key, err := DecodeKey(clientKey)
	if err != nil {
		return
	}

	ca, err := DecodeCertificate(clientCA)
	if err != nil {
		return
	}

	certBundle := []keystore.Certificate{{
		Type:    "X.509",
		Content: cert.Raw,
	}}

	jks := keystore.KeyStore{
		cert.Subject.CommonName: &keystore.PrivateKeyEntry{
			Entry: keystore.Entry{
				CreationDate: time.Now(),
			},
			PrivKey:   key,
			CertChain: certBundle,
		},
	}

	jks["trusted_ca"] = &keystore.TrustedCertificateEntry{
		Entry: keystore.Entry{
			CreationDate: time.Now(),
		},
		Certificate: keystore.Certificate{
			Type:    "X.509",
			Content: ca.Raw,
		},
	}

	var outBuf bytes.Buffer
	passw = GeneratePass(16)
	err = keystore.Encode(&outBuf, jks, passw)
	return outBuf.Bytes(), passw, err
}

// GenerateTestCert is used from unit tests for generating certificates
func GenerateTestCert() (cert, key []byte, expectedDn string, err error) {
	priv, serialNumber, err := generatePrivateKey()
	if err != nil {
		return cert, key, expectedDn, err
	}
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		Subject: pkix.Name{
			CommonName:   "test-cn",
			Organization: []string{"test-ou"},
		},
	}
	cert, err = x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return cert, key, expectedDn, err
	}
	buf := new(bytes.Buffer)
	if err = pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
		return
	}
	cert = buf.Bytes()
	key, err = encodePrivateKeyInPemFormat(priv)
	if err != nil {
		return cert, key, expectedDn, err
	}
	expectedDn = "CN=test-cn,O=test-ou"
	return cert, key, expectedDn, err
}

func generatePrivateKey() (*rsa.PrivateKey, *big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, serialNumber, err
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, serialNumber, err
	}
	return priv, serialNumber, err
}

func encodePrivateKeyInPemFormat(priv *rsa.PrivateKey) ([]byte, error) {
	keyBuf := new(bytes.Buffer)
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return nil, err
	}
	key := keyBuf.Bytes()
	return key, nil
}

// GeneratePrivateKeyInPemFormat is used to generate a private key in a pem format
func GeneratePrivateKeyInPemFormat() ([]byte, error) {
	priv, _, err := generatePrivateKey()
	if err != nil {
		return nil, err
	}
	return encodePrivateKeyInPemFormat(priv)
}

// GenerateSigningRequestInPemFormat is used to generate a signing request in a pem format
func GenerateSigningRequestInPemFormat(priv *rsa.PrivateKey, commonName string, organization []string) ([]byte, error) {
	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &template, priv)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err = pem.Encode(buf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr}); err != nil {
		return nil, err
	}
	signingReq := buf.Bytes()
	return signingReq, err
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
