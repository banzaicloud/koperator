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
	"context"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

func (v *vaultPKI) ReconcileUserCertificate(
	ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme, clusterDomain string) (*pkicommon.UserCertificate, error) {
	client, err := v.getClient()
	if err != nil {
		return nil, err
	}
	// get a list of all currently issued certificates for the cluster
	certs, err := v.list(client, v.getUserStorePath())
	if err != nil {
		return nil, err
	}

	var userSecret *vaultapi.Secret
	var userCert *pkicommon.UserCertificate

	if util.StringSliceContains(certs, string(user.GetUID())) {
		// we have a certificate already for this user
		userSecret, err = client.Logical().Read(
			v.getStorePathForUser(user),
		)
		if err != nil {
			return nil, errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to retrieve user certificate")
		}
		userCert = rawToCertificate(userSecret.Data)
	} else {
		// get max ttl allowed for role
		ttl, err := v.getMaxTTL(client)
		if err != nil {
			return nil, err
		}

		// args for a new certificate for the user
		args := map[string]interface{}{
			vaultCommonNameArg:        user.Name,
			vaultTTLArg:               ttl,
			vaultPrivateKeyFormatArg:  "pkcs8",
			vaultExcludeCNFromSANSArg: true,
		}
		// add dns names if provided
		if user.Spec.DNSNames != nil && len(user.Spec.DNSNames) > 0 {
			args[vaultAltNamesArg] = strings.Join(user.Spec.DNSNames, ",")
		}
		// issue the certificate
		userSecret, err = client.Logical().Write(v.getIssuePath(), args)
		if err != nil {
			return nil, errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to create user certificate, does the issue role exist?")
		}
		userCert = rawToCertificate(userSecret.Data)
		// write the issue response back to our storage for this user
		_, err = client.Logical().Write(
			v.getStorePathForUser(user),
			userSecret.Data,
		)
		if err != nil {
			return nil, errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to store user certificate, does the user path exist?")
		}
	}

	// ensure the user provided secret location with the credentials
	if err = ensureVaultSecret(client, userCert, user); err != nil {
		return nil, err
	}

	return userCert, nil
}

func ensureVaultSecret(client *vaultapi.Client, userCert *pkicommon.UserCertificate, user *v1alpha1.KafkaUser) error {
	storePath := checkSecretPath(user.Spec.SecretName)

	// Do pre-flight check to determine kv backend version
	v2, err := isKVv2(client)
	if err != nil {
		return err
	} else if v2 {
		storePath = addKVDataTypeToPath(storePath, "data")
	}

	var present *vaultapi.Secret

	// Check if we have an existing user secret
	if present, err = client.Logical().Read(storePath); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not check for existing user secret")
	}

	if present != nil {
		// "de-serialize" the existing vault secret
		presentCert, err := userCertForData(v2, present.Data)
		if err != nil {
			return errorfactory.New(errorfactory.InternalError{}, err, "could not parse stored user secret")
		}
		// make sure the stored certificate matches the one we have
		if certificatesMatch(presentCert, userCert) {
			// We have an existing secret stored that matches the provided one
			// we'll use that going forward to re-use a JKS if present
			userCert = presentCert
		}
	}

	// Ensure a JKS if requested
	if user.Spec.IncludeJKS {
		// we don't have an existing one - make a new one
		if userCert.JKS == nil || len(userCert.JKS) == 0 {
			userCert.JKS, userCert.Password, err = certutil.GenerateJKSFromByte(userCert.Certificate, userCert.Key, userCert.CA)
			if err != nil {
				return errorfactory.New(errorfactory.InternalError{}, err, "failed to generate JKS from user certificate")
			}
		}
	}

	// Write any changes back to the vault backend
	if _, err = client.Logical().Write(storePath, newVaultSecretData(v2, userCert)); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to store secret to user provided location")
	}
	return nil
}

func (v *vaultPKI) FinalizeUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser) (err error) {
	client, err := v.getClient()
	if err != nil {
		return
	}

	certs, err := v.list(client, v.getUserStorePath())
	if err != nil {
		return
	}

	if !util.StringSliceContains(certs, string(user.GetUID())) {
		// we'll just assume we already cleaned up
		return nil
	}

	userSecret, err := client.Logical().Read(
		v.getStorePathForUser(user),
	)
	if err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to retrieve user certificate")
	}

	userCert := rawToCertificate(userSecret.Data)

	// Revoke the certificate
	if _, err = client.Logical().Write(
		v.getRevokePath(),
		map[string]interface{}{
			vaultSerialNoKey: userCert.Serial,
		},
	); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to revoke user certificate")
	}

	v2, err := isKVv2(client)
	if err != nil {
		return
	}

	storePath := v.getStorePathForUser(user)

	if v2 {
		storePath = addKVDataTypeToPath(storePath, "metadata")
	}

	// Delete entry from user store
	if _, err = client.Logical().Delete(storePath); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to delete certificate from user store")
	}

	// Delete the user secret
	storePath = checkSecretPath(user.Spec.SecretName)

	if v2 {
		storePath = addKVDataTypeToPath(storePath, "metadata")
	}

	if _, err = client.Logical().Delete(storePath); err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to delete secret from user provided location")
		return
	}

	return nil
}
