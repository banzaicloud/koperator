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
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"reflect"
	"testing"

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type PKCS8Key struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

func TestDecodeKey(t *testing.T) {
	// Test a regular PKCS1 key
	_, key, _, err := GenerateTestCert()
	if err != nil {
		t.Error("Failed to generate test certificate")
	}

	var decoded []byte
	if decoded, err = DecodeKey(key); err != nil {
		t.Error("Expected to decode PKCS1 key, got error:", err)
	}

	// Convert to PKCS8 and test

	var pkey PKCS8Key
	pkey.Version = 0
	pkey.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 1)
	pkey.PrivateKeyAlgorithm[0] = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	rsaKey, err := x509.ParsePKCS1PrivateKey(decoded)
	if err != nil {
		t.Error("Error parsing generated key")
	}
	pkey.PrivateKey = x509.MarshalPKCS1PrivateKey(rsaKey)

	marshalledPKCS8, err := asn1.Marshal(pkey)
	if err != nil {
		t.Error("Failed to convert test key to PKCS8")
	}
	buf := new(bytes.Buffer)
	if err = pem.Encode(buf, &pem.Block{Type: "PRIVATE KEY", Bytes: marshalledPKCS8}); err != nil {
		t.Error("Failed to encode test PKCS8 key")
	}

	if _, err = DecodeKey(buf.Bytes()); err != nil {
		t.Error("Expected to decode PKCS8 key, got error:", err)
	}

	// Test error condition(s)
	stripped := key[:len(key)-10]
	if _, err = DecodeKey(stripped); err == nil {
		t.Error("Expected error trying to decode bad key, got nil")
	}
}

func TestDecodeCertificate(t *testing.T) {
	cert, _, name, err := GenerateTestCert()
	if err != nil {
		t.Error("Failed to generate test certificate")
	}
	if decoded, err := DecodeCertificate(cert); err != nil {
		t.Error("Expected to decode certificate, got error:", err)
	} else {
		if decoded.Subject.String() != name {
			t.Error("Expected certificate subject:", name, "got:", decoded.Subject.String())
		}
	}
}

func TestGeneratePass(t *testing.T) {
	if generated := GeneratePass(16); len(string(generated)) != 16 {
		t.Error("Expected to generate a 16 char pass, got:", len(string(generated)))
	}
}

func TestGenerateJKS(t *testing.T) {
	cert, key, _, err := GenerateTestCert()
	if err != nil {
		t.Error("Failed to generate test certificate")
	}
	caCert := cert

	if _, _, err = GenerateJKS(cert, key, caCert); err != nil {
		t.Error("Expected to generate JKS, got error:", err)
	}

	badCACert := cert[:len(cert)-10]
	if _, _, err = GenerateJKS(cert, key, badCACert); err == nil {
		t.Error("Expected to fail decoding CA cert, got nil error")
	}

	badKey := key[:len(key)-10]
	if _, _, err = GenerateJKS(cert, badKey, caCert); err == nil {
		t.Error("Expected to fail decoding key, got nil error")
	}

	badCert := key[:len(cert)-10]
	if _, _, err = GenerateJKS(badCert, key, caCert); err == nil {
		t.Error("Expected to fail decoding cert, got nil error")
	}
}

func TestEnsureJKS(t *testing.T) {
	cert, key, _, err := GenerateTestCert()
	if err != nil {
		t.Error("Failed to generate test certificate")
	}

	secret := &corev1.Secret{}
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       cert,
		corev1.TLSPrivateKeyKey: key,
		v1alpha1.CoreCACertKey:  cert,
	}

	if injectedSecret, err := EnsureSecretJKS(secret); err != nil {
		t.Error("Expected injected corev1 secret, got error:", err)
	} else {
		if _, ok := injectedSecret.Data[v1alpha1.TLSJKSKey]; !ok {
			t.Error("Expected JKS to be present in injected secret")
		}
		if _, ok := injectedSecret.Data[v1alpha1.PasswordKey]; !ok {
			t.Error("Expected generated password in injected secret")
		}

		noModify, _ := EnsureSecretJKS(injectedSecret)
		if !reflect.DeepEqual(noModify.Data, injectedSecret.Data) {
			t.Error("Expected already injected secret to be returned identical")
		}
	}
}
