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
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/banzaicloud/bank-vaults/pkg/sdk/vault"
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

// getCACertPath returns the path to read the raw CA certificate from
func (v *vaultPKI) getCACertPath() string {
	return fmt.Sprintf("%s/cert/ca", v.getCAPath())
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

// getClient retrieves a vault client using the role specified in the cluster configuration
func getKubernetesClient(clusterUID types.UID, role string) (client *vaultapi.Client, err error) {
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
		path = path + "/"
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
	mountPath, v2, err := isKVv2(storePath, vault)
	if err != nil {
		return nil, false, err
	} else if v2 {
		storePath = addPrefixToVKVPath(storePath, mountPath, "data")
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

// Below functions are pulled from github.com/hashicorp/vault/command/kv_helpers.go
// Unfortunately they are not exported - but are useful for determining how to read/store
// a user's secret.
// We use them to simulate `vault kv put`

func kvPreflightVersionRequest(client *vaultapi.Client, path string) (string, int, error) {
	r := client.NewRequest("GET", "/v1/sys/internal/ui/mounts/"+path)
	resp, err := client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		// If we get a 404 we are using an older version of vault, default to
		// version 1
		if resp != nil && resp.StatusCode == 404 {
			return "", 1, nil
		}

		return "", 0, err
	}

	secret, err := vaultapi.ParseSecret(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if secret == nil {
		return "", 0, errors.New("nil response from pre-flight request")
	}
	var mountPath string
	if mountPathRaw, ok := secret.Data["path"]; ok {
		mountPath = mountPathRaw.(string)
	}
	options := secret.Data["options"]
	if options == nil {
		return mountPath, 1, nil
	}
	versionRaw := options.(map[string]interface{})["version"]
	if versionRaw == nil {
		return mountPath, 1, nil
	}
	version := versionRaw.(string)
	switch version {
	case "", "1":
		return mountPath, 1, nil
	case "2":
		return mountPath, 2, nil
	}

	return mountPath, 1, nil
}

func isKVv2(path string, client *vaultapi.Client) (string, bool, error) {
	mountPath, version, err := kvPreflightVersionRequest(client, path)
	if err != nil {
		return "", false, err
	}

	return mountPath, version == 2, nil
}

func addPrefixToVKVPath(p, mountPath, apiPrefix string) string {
	switch {
	case p == mountPath, p == strings.TrimSuffix(mountPath, "/"):
		return path.Join(mountPath, apiPrefix)
	default:
		p = strings.TrimPrefix(p, mountPath)
		return path.Join(mountPath, apiPrefix, p)
	}
}
