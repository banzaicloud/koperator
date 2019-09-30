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
	"strings"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
)

const (
	vaultBackendPKI = "pki"
	vaultBackendKV  = "kv"

	// Fields in a vault PKI certificate
	vaultCertificateKey = "certificate"
	vaultIssuerKey      = "issuing_ca"
	vaultPrivateKeyKey  = "private_key"
	vaultSerialNoKey    = "serial_number"

	// Common arguments to vault certificates/issuers/configs
	vaultCommonNameArg        = "common_name"
	vaultAltNamesArg          = "alt_names"
	vaultTTLArg               = "ttl"
	vaultOUArg                = "ou"
	vaultPrivateKeyFormatArg  = "private_key_format"
	vaultExcludeCNFromSANSArg = "exclude_cn_from_sans"

	// Arguments to issuer config
	vaultIssuingCertificates   = "issuing_certificates"
	vaultCRLDistributionPoints = "crl_distribution_points"

	// Arguments to create issue role
	vaultAllowLocalhost   = "allow_localhost"
	vaultAllowedDomains   = "allowed_domains"
	vaultAllowSubdomains  = "allow_subdomains"
	vaultMaxTTL           = "max_ttl"
	vaultAllowAnyName     = "allow_any_name"
	vaultAllowIPSANS      = "allow_ip_sans"
	vaultAllowGlobDomains = "allow_glob_domains"
	vaultOrganization     = "organization"
	vaultUseCSRCommonName = "use_csr_common_name"
	vaultUseCSRSANS       = "use_csr_sans"
)

func getClient() (client *vaultapi.Client, err error) {
	config := vaultapi.DefaultConfig()
	if err = config.ReadEnvironment(); err != nil {
		err = errorfactory.New(errorfactory.InternalError{}, err, "could not read vault environment")
		return
	}
	client, err = vaultapi.NewClient(config)
	if err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not create a vault client")
	}
	return
}

func (v *vaultPKI) list(vault *vaultapi.Client, path string) (res []string, err error) {
	res = make([]string, 0)
	if path[len(path)-1:] != "/" {
		path = path + "/"
	}

	vaultRes, err := vault.Logical().List(path)
	if err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not list vault path", "path", path)
	} else if vaultRes == nil {
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

func (v *vaultPKI) getCA(vault *vaultapi.Client) (string, error) {
	ca, err := vault.Logical().Read(
		fmt.Sprintf("%s/cert/ca", v.getCAPath()),
	)
	if err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not retrieve vault CA certificate")
		return "", err
	}
	if ca == nil {
		err = errorfactory.New(errorfactory.InternalError{}, err, "our CA has dissappeared!")
		return "", err
	}
	caCert, ok := ca.Data[vaultCertificateKey].(string)
	if !ok {
		err = errorfactory.New(errorfactory.InternalError{}, err, "could not find certificate data in ca secret")
		return "", err
	}
	return caCert, nil
}

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

func dataForUserCert(cert *pkicommon.UserCertificate) map[string]interface{} {
	data := map[string]interface{}{
		corev1.TLSCertKey:       string(cert.Certificate),
		corev1.TLSPrivateKeyKey: string(cert.Key),
		v1alpha1.CoreCACertKey:  string(cert.CA),
	}
	if cert.JKS != nil && cert.Password != nil {
		data[v1alpha1.TLSJKSKey] = base64.StdEncoding.EncodeToString(cert.JKS)
		data[v1alpha1.PasswordKey] = string(cert.Password)
	}
	return data
}

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

	if _, ok := data[v1alpha1.TLSJKSKey]; ok {
		var err error
		jksB64, _ := data[v1alpha1.TLSJKSKey].(string)
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

func contains(vals []string, val string) bool {
	for _, x := range vals {
		if x == val {
			return true
		}
	}
	return false
}

func (v *vaultPKI) getCAPath() string {
	return fmt.Sprintf("pki_%s", v.cluster.GetUID())
}

func (v *vaultPKI) getUserStorePath() string {
	return fmt.Sprintf("pki_users_%s", v.cluster.GetUID())
}

func (v *vaultPKI) getIssuePath() string {
	return fmt.Sprintf("%s/issue/operator", v.getCAPath())
}

// Below functions are pulled from github.com/hashicorp/vault/command/kv_helpers.go
// Unfortunately they are not exported - but are useful for determining how to store
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
