package certutil

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/rand"
	"strings"
	"text/template"
	"time"

	logr "github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	keystore "github.com/pavel-v-chernykh/keystore-go"
	corev1 "k8s.io/api/core/v1"
)

const (
	TLSCAKey            = "ca.crt"
	TLSJKSKey           = "tls.jks"
	TLSPasswordKey      = "pass.txt"
	ClientPropertiesKey = "client-ssl.properties"
)

// This gives the user of the secret a convenience consumer/producer config.
// It may not be necessary.
var clientPropertiesTemplate = `security.protocol=SSL
ssl.truststore.location=/etc/certs/tls.jks
ssl.truststore.password={{ .Password }}
ssl.keystore.location=/etc/certs/tls.jks
ssl.keystore.password={{ .Password }}
ssl.key.password={{ .Password }}
`

func DecodeKey(raw []byte) (parsedKey []byte, err error) {
	block, _ := pem.Decode(raw)
	var keytype certv1.KeyEncoding
	var key interface{}
	if key, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
		if key, err = x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
			return
		} else {
			keytype = certv1.PKCS8
		}
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

func DecodeCertificate(raw []byte) (cert *x509.Certificate, err error) {
	block, _ := pem.Decode(raw)
	cert, err = x509.ParseCertificate(block.Bytes)
	return
}

func GeneratePass(length int) (passw []byte) {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	passw = []byte(b.String())
	return
}

func InjectJKS(reqLogger logr.Logger, secret *corev1.Secret) (injected *corev1.Secret, err error) {
	injected = secret.DeepCopy()
	cert, err := DecodeCertificate(secret.Data[corev1.TLSCertKey])
	if err != nil {
		reqLogger.Error(err, "Could not parse tls.crt from secret")
		return
	}

	key, err := DecodeKey(secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		reqLogger.Error(err, "Could not parse tls.key from secret")
		return
	}

	ca, err := DecodeCertificate(secret.Data[TLSCAKey])
	if err != nil {
		reqLogger.Error(err, "Could not parse ca.crt from secret")
		return
	}

	certBundle := []keystore.Certificate{keystore.Certificate{
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

	var passw []byte
	var ok bool
	if passw, ok = secret.Data[TLSPasswordKey]; !ok {
		passw = GeneratePass(16)
	}

	var jksOut bytes.Buffer
	err = keystore.Encode(&jksOut, jks, passw)
	if err != nil {
		reqLogger.Error(err, "Could not encode chain to JKS")
		return
	}

	var propsOut bytes.Buffer
	t := template.Must(template.New("client-ssl.properties").Parse(clientPropertiesTemplate))
	err = t.Execute(&propsOut, map[string]string{
		"Password": string(passw),
	})
	if err != nil {
		reqLogger.Error(err, "Could not create properties template")
		return
	}

	injected.Data[TLSJKSKey] = jksOut.Bytes()
	injected.Data[TLSPasswordKey] = passw
	injected.Data[ClientPropertiesKey] = propsOut.Bytes()
	return
}
