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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// newCA returns an unsigned CA Certificate object with the given organization
func newCA(ou string) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: newSerial(),
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("%s-ca", ou),
			Organization: []string{ou},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
}

// newCert returns an unsigned client/server certificate with the given values
func newCert(cn, ou string, dnsNames []string, expiry time.Time) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: newSerial(),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{ou},
		},
		DNSNames:     dnsNames,
		NotBefore:    time.Now(),
		NotAfter:     expiry,
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
}

// newKey generates a 4096-bit RSA keypair
func newKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 4096)
}

// newSerial returns a serial number for a certificate. These values don't need
// to be random, but need to be unique within a PKI. Generating a random number for this
// adds to the certificate creation time quite a bit since generating a key takes a fair amount
// of entropy already. Taking the nano value since epoch instead gives us a number that
// will always be different within a PKI since the controller does not allow concurrent reconciles.
func newSerial() *big.Int {
	return big.NewInt(time.Now().UnixNano())
}

// encodeToPEM takes a CA, signed certificate, and key - and returns their PEM encoded values
func encodeToPEM(ca *x509.Certificate, certBytes []byte, key *rsa.PrivateKey) (caPEM, certPEM, keyPEM []byte, err error) {
	caPEMBuf := new(bytes.Buffer)
	if err := pem.Encode(caPEMBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}); err != nil {
		return nil, nil, nil, err
	}
	certPEMBuf := new(bytes.Buffer)
	if err := pem.Encode(certPEMBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return nil, nil, nil, err
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	certPrivKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}); err != nil {
		return nil, nil, nil, err
	}
	return caPEMBuf.Bytes(), certPEMBuf.Bytes(), certPrivKeyPEM.Bytes(), nil
}
