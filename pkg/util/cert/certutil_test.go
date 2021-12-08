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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"reflect"
	"testing"

	"github.com/pavel-v-chernykh/keystore-go/v4"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
)

func TestDecodePrivateKeyBytes(t *testing.T) {
	ecdsaPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Error("Failed to generate test priv key", err)
	}
	pemEcdsaPriv := new(bytes.Buffer)
	marshaledEcdsaPriv, _ := x509.MarshalECPrivateKey(ecdsaPriv)
	if err := pem.Encode(pemEcdsaPriv, &pem.Block{Type: ECPrivateKeyType, Bytes: marshaledEcdsaPriv}); err != nil {
		t.Error("Failed to encode test priv key", err)
	}
	_, pkcs8Priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Error("Failed to generate test priv key", err)
	}
	pemPkcs8Priv := new(bytes.Buffer)
	marshaledPkcs8Priv, _ := x509.MarshalPKCS8PrivateKey(pkcs8Priv)
	if err := pem.Encode(pemPkcs8Priv, &pem.Block{Type: PrivateKeyType, Bytes: marshaledPkcs8Priv}); err != nil {
		t.Error("Failed to encode test priv key", err)
	}

	pkcs1Priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Error("Failed to generate test priv key", err)
	}
	pemPkcs1Priv := new(bytes.Buffer)
	marshaledPkcs1Priv := x509.MarshalPKCS1PrivateKey(pkcs1Priv)
	if err := pem.Encode(pemPkcs1Priv, &pem.Block{Type: RSAPrivateKeyType, Bytes: marshaledPkcs1Priv}); err != nil {
		t.Error("Failed to encode test priv key", err)
	}
	testCases := []struct {
		testName string
		privKey  []byte
	}{
		{
			testName: RSAPrivateKeyType,
			privKey:  pemPkcs1Priv.Bytes(),
		},
		{
			testName: PrivateKeyType,
			privKey:  pemPkcs8Priv.Bytes(),
		},
		{
			testName: ECPrivateKeyType,
			privKey:  pemEcdsaPriv.Bytes(),
		},
	}

	t.Parallel()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			key, err := DecodePrivateKeyBytes(test.privKey)
			if err != nil {
				t.Error("Expected error nil but get at decode priv key", err)
			}
			_, err = x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				t.Error("Expected error nil but get error at convert priv key to PKCS8", err)
			}
		})

	}
}

func TestDecodeCertificate(t *testing.T) {
	cert, _, name, err := GenerateTestCert()
	if err != nil {
		t.Error("Failed to generate test certificate")
	}
	if decoded, err := DecodeCertificate(cert); err != nil {
		t.Error("Expected to decode certificate, got error:", err)
	} else if decoded.Subject.String() != name {
		t.Error("Expected certificate subject:", name, "got:", decoded.Subject.String())
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
	keyStoreBytes, password, err := GenerateJKSFromByte(cert, key, caCert)
	if err != nil {
		t.Error("Expected to generate JKS, got error:", err)
	}
	jksKeyStore := keystore.New()
	keyStoreBytesReader := bytes.NewReader(keyStoreBytes)
	if err = jksKeyStore.Load(keyStoreBytesReader, password); err != nil {
		t.Error("Failed to generate keyStore from keyStore representation in bytes, probably due wrong pssword. Error: ", err)
	}

	privKeyEntry, err := jksKeyStore.GetPrivateKeyEntry("certs", password)
	if err != nil {
		t.Error("Can't get private key entry", err)
	}

	_, err = x509.ParsePKCS8PrivateKey(privKeyEntry.PrivateKey)
	if err != nil {
		t.Error("Private key must be PKCS8 encoded format in JKS", err)
	}

	badCACert := cert[:len(cert)-10]
	if _, _, err = GenerateJKSFromByte(cert, key, badCACert); err == nil {
		t.Error("Expected to fail decoding CA cert, got nil error")
	}

	badKey := key[:len(key)-10]
	if _, _, err = GenerateJKSFromByte(cert, badKey, caCert); err == nil {
		t.Error("Expected to fail decoding key, got nil error")
	}

	badCert := key[:len(cert)-10]
	if _, _, err = GenerateJKSFromByte(badCert, key, caCert); err == nil {
		t.Error("Expected to fail decoding cert, got nil error")
	}
}

func TestEnsureJKSPassoword(t *testing.T) {
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

	if injectedSecret, err := EnsureSecretPassJKS(secret); err != nil {
		t.Error("Expected injected corev1 secret, got error:", err)
	} else {
		if _, ok := injectedSecret.Data[v1alpha1.PasswordKey]; !ok {
			t.Error("Expected generated password in injected secret")
		}

		noModify, _ := EnsureSecretPassJKS(injectedSecret)
		if !reflect.DeepEqual(noModify.Data, injectedSecret.Data) {
			t.Error("Expected already injected secret to be returned identical")
		}
	}
}
