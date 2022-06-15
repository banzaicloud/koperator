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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/banzaicloud/koperator/api/v1alpha1"

	"github.com/pavel-v-chernykh/keystore-go/v4"
	corev1 "k8s.io/api/core/v1"
)

const (
	RSAPrivateKeyType = "RSA PRIVATE KEY"
	PrivateKeyType    = "PRIVATE KEY"
	ECPrivateKeyType  = "EC PRIVATE KEY"
	CertRequestType   = "CERTIFICATE REQUEST"
)

type CertificateContainer struct {
	// Certificate
	Certificate *x509.Certificate
	// PEM holds the certificate in PEM format
	PEM *pem.Block
}

func (c CertificateContainer) ToPEM() []byte {
	return pem.EncodeToMemory(c.PEM)
}

func GetCertBundle(certContainers []*CertificateContainer) []*x509.Certificate {
	certs := make([]*x509.Certificate, 0, len(certContainers))
	for _, certContainer := range certContainers {
		certs = append(certs, certContainer.Certificate)
	}
	return certs
}

func ParseCertificates(data []byte) ([]*CertificateContainer, error) {
	ok := false
	certs := make([]*CertificateContainer, 0)

	for len(data) > 0 {
		var certBlock *pem.Block

		certBlock, data = pem.Decode(data)
		if certBlock == nil {
			return certs, fmt.Errorf("malformed PEM data found")
		}
		if certBlock.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, err
		}

		certs = append(certs, &CertificateContainer{cert, certBlock})
		ok = true
	}

	if !ok {
		return certs, fmt.Errorf("no certificates found")
	}

	return certs, nil
}

// passChars are the characters used when generating passwords
var passChars []rune = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789")

// DecodePrivateKeyBytes will decode a PEM encoded private key into a crypto.Signer.
// It supports ECDSA, PKCS1, PKCS8  private key format only. All other types will return err.
func DecodePrivateKeyBytes(keyBytes []byte) (key crypto.Signer, err error) {
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM data")
	}

	switch block.Type {
	case RSAPrivateKeyType:
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case PrivateKeyType:
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		var ok bool
		if key, ok = parsedKey.(crypto.Signer); !ok {
			return nil, errors.New("error parsing pkcs8 private key")
		}
	case ECPrivateKeyType:
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unknown private key type: %s", block.Type)
	}

	return
}

// DecodeCertificate returns an x509.Certificate for a PEM encoded certificate
func DecodeCertificate(raw []byte) (cert *x509.Certificate, err error) {
	certs, err := ParseCertificates(raw)
	if err != nil {
		return nil, err
	}
	if len(certs) != 1 {
		return nil, errors.New("only one certificate should be present, more found")
	}
	return certs[0].Certificate, nil
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

func GenerateJKSFromByte(certByte []byte, privateKey []byte, caCert []byte) (out, passw []byte, err error) {
	c, err := DecodeCertificate(certByte)
	if err != nil {
		return
	}
	ca, err := DecodeCertificate(caCert)
	if err != nil {
		return
	}

	return GenerateJKS([]*x509.Certificate{ca, c}, privateKey)
}

// GenerateJKS creates a JKS with a random password from a client cert/key combination
func GenerateJKS(certs []*x509.Certificate, privateKey []byte) (out, passw []byte, err error) {
	pKeyRaw, err := DecodePrivateKeyBytes(privateKey)
	if err != nil {
		return nil, nil, err
	}

	pKeyPKCS8, err := x509.MarshalPKCS8PrivateKey(pKeyRaw)
	if err != nil {
		return nil, nil, err
	}

	certCABundle := make([]keystore.Certificate, 0, len(certs))
	for _, cert := range certs {
		kcert := keystore.Certificate{
			Type:    "X.509",
			Content: cert.Raw,
		}
		certCABundle = append(certCABundle, kcert)
	}

	jksKeyStore := keystore.New()

	pkeIn := keystore.PrivateKeyEntry{
		CreationTime:     time.Now(),
		PrivateKey:       pKeyPKCS8,
		CertificateChain: certCABundle,
	}

	//Add into trusted from our cert chain
	for i, cert := range certs {
		if cert.IsCA {
			caIn := keystore.TrustedCertificateEntry{
				CreationTime: time.Now(),
				Certificate: keystore.Certificate{
					Type:    "X.509",
					Content: cert.Raw,
				},
			}
			alias := fmt.Sprintf("trusted_ca_%d", i)
			if err = jksKeyStore.SetTrustedCertificateEntry(alias, caIn); err != nil {
				return nil, nil, err
			}
		}
	}

	password := GeneratePass(16)
	if err = jksKeyStore.SetPrivateKeyEntry("certs", pkeIn, password); err != nil {
		return nil, nil, err
	}

	var outBuf bytes.Buffer
	if err = jksKeyStore.Store(&outBuf, password); err != nil {
		return nil, nil, err
	}
	return outBuf.Bytes(), password, err
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
func GenerateSigningRequestInPemFormat(priv *rsa.PrivateKey, commonName string, dnsNames []string) ([]byte, error) {
	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames: dnsNames,
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &template, priv)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err = pem.Encode(buf, &pem.Block{Type: CertRequestType, Bytes: csr}); err != nil {
		return nil, err
	}
	signingReq := buf.Bytes()
	return signingReq, err
}
