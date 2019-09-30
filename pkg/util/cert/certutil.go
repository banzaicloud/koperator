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

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	keystore "github.com/pavel-v-chernykh/keystore-go"
	corev1 "k8s.io/api/core/v1"
)

// DecodeKey will take a PEM encoded Private Key and convert to raw der bytes
func DecodeKey(raw []byte) (parsedKey []byte, err error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		err = errors.New("failed to decode PEM data")
		return
	}
	var keytype certv1.KeyEncoding
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
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[mathrand.Intn(len(chars))])
	}
	passw = []byte(b.String())
	return
}

// EnsureSecretJKS ensures a JKS is present in a certificate secret
func EnsureSecretJKS(secret *corev1.Secret) (injected *corev1.Secret, err error) {

	// If the JKS is already present - return
	if _, ok := secret.Data[v1alpha1.TLSJKSKey]; ok {
		return secret, nil
	}

	injected = secret.DeepCopy()

	jks, passw, err := GenerateJKS(
		secret.Data[corev1.TLSCertKey],
		secret.Data[corev1.TLSPrivateKeyKey],
		secret.Data[v1alpha1.CoreCACertKey],
	)
	if err != nil {
		return
	}

	injected.Data[v1alpha1.TLSJKSKey] = jks
	injected.Data[v1alpha1.PasswordKey] = passw
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
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
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
		return
	}
	buf := new(bytes.Buffer)
	keyBuf := new(bytes.Buffer)
	if err = pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
		return
	}
	if err = pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return
	}
	cert = buf.Bytes()
	key = keyBuf.Bytes()
	expectedDn = "CN=test-cn,O=test-ou"
	return
}
