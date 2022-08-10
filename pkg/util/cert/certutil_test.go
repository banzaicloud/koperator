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
	"encoding/base64"
	"encoding/pem"
	"reflect"
	"testing"

	"github.com/pavlo-v-chernykh/keystore-go/v4"

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

func TestParseTLSCertFromKeyStore(t *testing.T) {
	testCases := []struct {
		testName   string
		trustStore string
		keyStore   string
		password   string
	}{
		{
			testName: "basic",
			//nolint:lll
			keyStore: "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",
			password: "MjNZcGRpSzVoNlNFY2o5OQ==",
		},
	}

	for _, test := range testCases {
		keyStore, _ := base64.StdEncoding.DecodeString(test.keyStore)
		password, _ := base64.StdEncoding.DecodeString(test.password)

		_, err := ParseTLSCertFromKeyStore(keyStore, password)
		if err != nil {
			t.Errorf("error should be nil, got: %s", err)
		}
	}
}
