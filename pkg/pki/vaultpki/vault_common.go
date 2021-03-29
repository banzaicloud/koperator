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

package vaultpki

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/bank-vaults/pkg/sdk/vault"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

const (

	// Fields in a vault PKI certificate
	vaultCertificateKey = "certificate"
	vaultIssuerKey      = "issuing_ca"
	vaultPrivateKeyKey  = "private_key"
	vaultSerialNoKey    = "serial_number"

	// Common arguments to vault certificates/issuers/configs
	vaultCommonNameArg        = "common_name"
	vaultAltNamesArg          = "alt_names"
	vaultTTLArg               = "ttl"
	vaultPrivateKeyFormatArg  = "private_key_format"
	vaultExcludeCNFromSANSArg = "exclude_cn_from_sans"
)

// vaultClients are cached in memory for each cluster since they handle token
// renewal internally and creating a new one every reconcile will flood the logs
// Also, without this - we quickly hit the max file descriptors in the container
var vaultClients = make(map[types.UID]*vault.Client)

// getCAPath returns the path to the pki mount for a cluster
func (v *vaultPKI) getCAPath() string {
	return v.cluster.Spec.VaultConfig.PKIPath
}

// getUserStorePath returns the path to where issued keys/certificates are
// held persistently for use within the operator
func (v *vaultPKI) getUserStorePath() string {
	return v.cluster.Spec.VaultConfig.UserStore
}

// getIssuePath returns the path for issuing certificates for the cluster
func (v *vaultPKI) getIssuePath() string {
	return v.cluster.Spec.VaultConfig.IssuePath
}

// getRevokePath returns the revocation path for a cluster
func (v *vaultPKI) getRevokePath() string {
	return fmt.Sprintf("%s/revoke", v.getCAPath())
}

// getStorePathForUser returns the persistent storage path for a user
func (v *vaultPKI) getStorePathForUser(user *v1alpha1.KafkaUser) string {
	return fmt.Sprintf("%s/%s", v.getUserStorePath(), user.GetUID())
}

// checkSecretPath checks if the provided path is a full vault path and
// returns a modified path for the default backend if not
func checkSecretPath(path string) string {
	if len(strings.Split(path, "/")) == 1 {
		return fmt.Sprintf("secret/%s", path)
	}
	return path
}

// getVaultClient retrieves a vault client using the role specified in the cluster configuration
func getVaultClient(clusterUID types.UID, role string) (client *vaultapi.Client, err error) {
	// return a cached one for the cluster if we have it
	// otherwise we'll constantly log new token acquisitions
	if vaultClient, ok := vaultClients[clusterUID]; ok {
		return vaultClient.RawClient(), nil
	}
	vaultClient, err := vault.NewClient(role)
	if err != nil {
		return nil, err
	}
	vaultClients[clusterUID] = vaultClient
	return vaultClient.RawClient(), nil
}

// newVaultSecretData returns raw POST data for a user certificate object
func newVaultSecretData(isV2 bool, cert *pkicommon.UserCertificate) map[string]interface{} {
	if isV2 {
		return map[string]interface{}{
			"data":    dataForUserCert(cert),
			"options": map[string]interface{}{},
		}
	}
	return dataForUserCert(cert)
}

// certificatesMatch checks if two certificate objects are identical
func certificatesMatch(cert1, cert2 *pkicommon.UserCertificate) bool {
	if string(cert1.CA) != string(cert2.CA) || string(cert1.Certificate) != string(cert2.Certificate) || string(cert1.Key) != string(cert2.Key) {
		return false
	}
	return true
}

// list is a convenience wrapper for getting a list of objects at a path in vault
func (v *vaultPKI) list(vault *vaultapi.Client, path string) (res []string, err error) {
	res = make([]string, 0)
	if path[len(path)-1:] != "/" {
		path += "/"
	}
	vaultRes, err := vault.Logical().List(path)
	if err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not list vault path", "path", path)
		return
	} else if vaultRes == nil {
		// path is empty
		return
	}

	var ifaceSlice []interface{}
	var ok bool
	if ifaceSlice, ok = vaultRes.Data["keys"].([]interface{}); !ok {
		return
	}

	for _, x := range ifaceSlice {
		str, _ := x.(string)
		res = append(res, str)
	}
	return
}

// getMaxTTL returns the maximum TTL allowed for the provided issue role
func (v *vaultPKI) getMaxTTL(vault *vaultapi.Client) (string, error) {
	res, err := v.getCA(vault)
	if err != nil {
		return "", err
	}
	cert, err := certutil.DecodeCertificate([]byte(res))
	if err != nil {
		return "", err
	}
	// return one hour before the CA expires
	caExpiry := cert.NotAfter.Sub(time.Now())
	caExpiryHours := (caExpiry / time.Hour) - 1
	return strconv.Itoa(int(caExpiryHours)) + "h", nil
}

// getCA returns the PEM encoded CA certificate for the cluster
func (v *vaultPKI) getCA(vault *vaultapi.Client) (string, error) {
	ca, err := vault.Logical().Read(
		fmt.Sprintf("%s/cert/ca", v.getCAPath()),
	)
	if err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not retrieve vault CA certificate")
		return "", err
	}

	// The below two error conditions would be a show-stopper, but also signify
	// that we may not have the correct permissions.
	if ca == nil {
		err = errorfactory.New(errorfactory.FatalReconcileError{}, errors.New("got nil back for CA"), "our CA has dissappeared!")
		return "", err
	}
	caCert, ok := ca.Data[vaultCertificateKey].(string)
	if !ok {
		err = errorfactory.New(errorfactory.FatalReconcileError{}, errors.New("no certificate key in ca"), "could not find certificate data in ca secret")
		return "", err
	}
	return caCert, nil
}

// getSecret is a convenience wrapper for reading a vault path
// It takes care of checking kv version pre-flight
func getSecret(vault *vaultapi.Client, storePath string) (secret *vaultapi.Secret, v2 bool, err error) {
	v2, err = isKVv2(vault)
	if err != nil {
		return nil, false, err
	} else if v2 {
		storePath = addKVDataTypeToPath(storePath, "data")
	}

	secret, err = vault.Logical().Read(storePath)
	return
}

// rawToCertificate converts the response from an issue request to a UserCertificate object
func rawToCertificate(data map[string]interface{}) *pkicommon.UserCertificate {
	caCert, _ := data[vaultIssuerKey].(string)
	clientCert, _ := data[vaultCertificateKey].(string)
	clientKey, _ := data[vaultPrivateKeyKey].(string)
	serial, _ := data[vaultSerialNoKey].(string)

	return &pkicommon.UserCertificate{
		CA:          []byte(caCert),
		Certificate: []byte(clientCert),
		Key:         []byte(clientKey),
		Serial:      serial,
	}
}

// dataForUserCert converts a UserCertificate to data for writing to the persistent back-end
func dataForUserCert(cert *pkicommon.UserCertificate) map[string]interface{} {
	data := map[string]interface{}{
		corev1.TLSCertKey:       string(cert.Certificate),
		corev1.TLSPrivateKeyKey: string(cert.Key),
		v1alpha1.CoreCACertKey:  string(cert.CA),
	}
	if cert.JKS != nil && cert.Password != nil {
		data[v1alpha1.TLSJKSKeyStore] = base64.StdEncoding.EncodeToString(cert.JKS)
		data[v1alpha1.TLSJKSTrustStore] = base64.StdEncoding.EncodeToString(cert.JKS)
		data[v1alpha1.PasswordKey] = string(cert.Password)
	}
	return data
}

// userCertForData returns a UserCertificate object for data read from the persistent back-end
func userCertForData(isV2 bool, data map[string]interface{}) (*pkicommon.UserCertificate, error) {
	if isV2 {
		var ok bool
		if data, ok = data["data"].(map[string]interface{}); !ok {
			return nil, errors.New("got v2 secret but could not parse data field")
		}
	}
	cert := &pkicommon.UserCertificate{}
	for _, key := range []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey, v1alpha1.CoreCACertKey} {
		if _, ok := data[key]; !ok {
			return nil, fmt.Errorf("user secret does not contain required field: %s", key)
		}
	}
	certStr, _ := data[corev1.TLSCertKey].(string)
	keyStr, _ := data[corev1.TLSPrivateKeyKey].(string)
	caStr, _ := data[v1alpha1.CoreCACertKey].(string)

	cert.Certificate = []byte(certStr)
	cert.Key = []byte(keyStr)
	cert.CA = []byte(caStr)

	if _, ok := data[v1alpha1.TLSJKSKeyStore]; ok {
		var err error
		jksB64, _ := data[v1alpha1.TLSJKSKeyStore].(string)
		cert.JKS, err = base64.StdEncoding.DecodeString(jksB64)
		if err != nil {
			return nil, err
		}
	}

	if _, ok := data[v1alpha1.PasswordKey]; ok {
		passw, _ := data[v1alpha1.PasswordKey].(string)
		cert.Password = []byte(passw)
	}

	return cert, nil
}

func isKVv2(client *vaultapi.Client) (bool, error) {
	kv2Config, err := client.Logical().Read("secret/config")
	if err != nil {
		return false, err
	}

	if kv2Config == nil {
		return false, nil
	} else if _, ok := kv2Config.Data["cas_required"]; ok {
		return true, nil
	}

	return false, nil
}

func addKVDataTypeToPath(path, dataType string) string {
	if strings.HasPrefix(path, "secret/"+dataType+"/") {
		return path
	}
	suffix := strings.TrimPrefix(path, "secret/")
	return "secret/" + dataType + "/" + suffix
}
