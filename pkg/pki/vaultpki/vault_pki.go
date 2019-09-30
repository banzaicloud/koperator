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
	"errors"
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	"github.com/go-logr/logr"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const pkiOU = "vault-pki"

func (v *vaultPKI) FinalizePKI(logger logr.Logger) error {
	vault, err := v.getClient()
	if err != nil {
		return err
	}

	// Make sure we finished cleaning up users - the controller should have
	// triggered a delete of all children for this cluster
	users, err := v.list(vault, v.getUserStorePath())
	if err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not list user certificates")
	}
	for _, user := range users {
		if user != "broker" && user != "controller" {
			err = fmt.Errorf("Non broker/controller user exists: %s", user)
			return errorfactory.New(errorfactory.ResourceNotReady{}, err, "pki still contains user certificates")
		}
	}

	// List the currently mounted backends
	presentMounts, err := vault.Sys().ListMounts()
	if err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not list system mounts in vault")
	}

	checkMounts := []string{
		fmt.Sprintf("%s/", v.getUserStorePath()),
		fmt.Sprintf("%s/", v.getCAPath()),
	}

	// Iterate our mounts and unmount if present
	for _, mount := range checkMounts {
		if _, ok := presentMounts[mount]; ok {
			logger.Info("Unmounting vault path", "mount", mount)
			if err := vault.Sys().Unmount(mount[:len(mount)-1]); err != nil {
				return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not unmount vault path", "path", mount)
			}
		}
	}

	return nil
}

func (v *vaultPKI) ReconcilePKI(logger logr.Logger, scheme *runtime.Scheme) (err error) {
	log := logger.WithName("vault_pki")

	vault, err := v.getClient()
	if err != nil {
		return
	}

	// List the currently mounted backends
	mounts, err := vault.Sys().ListMounts()
	if err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not list system mounts in vault")
	}

	// Assume that if our CA doesn't exist we need to do an initial setup
	// TODO: (tinyzimmer) There might be a safer way to do this
	if _, ok := mounts[fmt.Sprintf("%s/", v.getCAPath())]; !ok {
		log.Info("Creating new vault PKI", "vault_addr", vault.Address())
		if err = v.initVaultPKI(vault); err != nil {
			return
		}
	}

	brokerCert, err := v.reconcileBrokerCert(vault)
	if err != nil {
		return
	}

	controllerCert, err := v.reconcileControllerCert(vault)
	if err != nil {
		return
	}

	return v.reconcileBootstrapSecrets(scheme, brokerCert, controllerCert)
}

func (v *vaultPKI) reconcileBootstrapSecrets(scheme *runtime.Scheme, brokerCert, controllerCert *pkicommon.UserCertificate) (err error) {
	serverSecret := &corev1.Secret{}
	clientSecret := &corev1.Secret{}

	toCreate := make([]*corev1.Secret, 0)

	if err = v.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      fmt.Sprintf(pkicommon.BrokerServerCertTemplate, v.cluster.Name),
			Namespace: v.cluster.Namespace,
		},
		serverSecret,
	); err != nil {
		if apierrors.IsNotFound(err) {
			serverCert := &corev1.Secret{
				ObjectMeta: templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerServerCertTemplate, v.cluster.Name), pkicommon.LabelsForKafkaPKI(v.cluster.Name), v.cluster),
				Data: map[string][]byte{
					corev1.TLSCertKey:       brokerCert.Certificate,
					corev1.TLSPrivateKeyKey: brokerCert.Key,
					v1alpha1.CoreCACertKey:  brokerCert.CA,
					v1alpha1.TLSJKSKey:      brokerCert.JKS,
					v1alpha1.PasswordKey:    brokerCert.Password,
				},
			}
			controllerutil.SetControllerReference(v.cluster, serverCert, scheme)
			toCreate = append(toCreate, serverCert)
		} else {
			return errorfactory.New(errorfactory.APIFailure{}, err, "failed to check existence of server secret")
		}
	}

	if err = v.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      fmt.Sprintf(pkicommon.BrokerControllerTemplate, v.cluster.Name),
			Namespace: v.cluster.Namespace,
		},
		clientSecret,
	); err != nil {
		if apierrors.IsNotFound(err) {
			clientCert := &corev1.Secret{
				ObjectMeta: templates.ObjectMeta(fmt.Sprintf(pkicommon.BrokerControllerTemplate, v.cluster.Name), pkicommon.LabelsForKafkaPKI(v.cluster.Name), v.cluster),
				Data: map[string][]byte{
					corev1.TLSCertKey:       controllerCert.Certificate,
					corev1.TLSPrivateKeyKey: controllerCert.Key,
					v1alpha1.CoreCACertKey:  controllerCert.CA,
					v1alpha1.TLSJKSKey:      controllerCert.JKS,
					v1alpha1.PasswordKey:    controllerCert.Password,
				},
			}
			controllerutil.SetControllerReference(v.cluster, clientCert, scheme)
			toCreate = append(toCreate, clientCert)
		} else {
			return errorfactory.New(errorfactory.APIFailure{}, err, "failed to check existence of client secret")
		}
	}

	for _, o := range toCreate {
		if err = v.client.Create(context.TODO(), o); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "failed to create bootstrap secret", "secret", o.Name)
		}
	}

	return
}

func (v *vaultPKI) reconcileBrokerCert(vault *vaultapi.Client) (*pkicommon.UserCertificate, error) {
	return v.reconcileStartupUser(vault, pkicommon.BrokerUserForCluster(v.cluster))
}

func (v *vaultPKI) reconcileControllerCert(vault *vaultapi.Client) (*pkicommon.UserCertificate, error) {
	return v.reconcileStartupUser(vault, pkicommon.ControllerUserForCluster(v.cluster))
}

func (v *vaultPKI) reconcileStartupUser(vault *vaultapi.Client, user *v1alpha1.KafkaUser) (cert *pkicommon.UserCertificate, err error) {
	existing := &v1alpha1.KafkaUser{}
	if err := v.client.Get(context.TODO(), types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			if err = v.client.Create(context.TODO(), user); err != nil {
				return nil, err
			}
		}
	}
	userSecret, v2, err := getSecret(vault, fmt.Sprintf("secret/%s", user.Spec.SecretName))
	if err != nil {
		return nil, err
	} else if userSecret == nil || userSecret.Data == nil {
		return nil, errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("not ready"), "certificate is not ready")
	}
	cert, err = userCertForData(v2, userSecret.Data)
	return
}

func (v *vaultPKI) initVaultPKI(vault *vaultapi.Client) error {
	caPath := v.getCAPath()
	// intermediatePath := v.getIntermediatePath()
	userPath := v.getUserStorePath()
	var err error

	// Setup a PKI mount for the ca
	if err = vault.Sys().Mount(
		caPath,
		&vaultapi.MountInput{
			Type: vaultBackendPKI,
			Config: vaultapi.MountConfigInput{
				MaxLeaseTTL: "219000h",
			},
		},
	); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to setup mount for root pki")
	}

	// Setup a KV mount for user certificates
	if err = vault.Sys().Mount(
		userPath,
		&vaultapi.MountInput{
			Type: vaultBackendKV,
		},
	); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to setup kv mount for user certificate store")
	}

	// Create a root CA
	if _, err = vault.Logical().Write(
		fmt.Sprintf("%s/root/generate/internal", caPath),
		map[string]interface{}{
			vaultCommonNameArg:        fmt.Sprintf(pkicommon.CAFQDNTemplate, v.cluster.Name, v.cluster.Namespace),
			vaultOUArg:                pkiOU,
			vaultExcludeCNFromSANSArg: true,
			vaultTTLArg:               "215000h",
		},
	); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to generate root certificate")
	}

	// set information about distribution points and so on
	_, err = vault.Logical().Write(
		fmt.Sprintf("%s/config/urls", caPath),
		map[string]interface{}{
			vaultIssuingCertificates:   fmt.Sprintf("%s/v1/%s/ca", vault.Address(), caPath),
			vaultCRLDistributionPoints: fmt.Sprintf("%s/v1/%s/crl", vault.Address(), caPath),
		},
	)
	if err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to write config for root pki")
	}

	// Create a role that can issue certificates
	if _, err = vault.Logical().Write(
		fmt.Sprintf("%s/roles/operator", caPath),
		map[string]interface{}{
			vaultAllowLocalhost:   true,
			vaultAllowedDomains:   "*",
			vaultAllowSubdomains:  true,
			vaultMaxTTL:           "60000h",
			vaultAllowAnyName:     true,
			vaultAllowIPSANS:      true,
			vaultAllowGlobDomains: true,
			vaultOrganization:     pkiOU,
			vaultUseCSRCommonName: false,
			vaultUseCSRSANS:       false,
		},
	); err != nil {
		return errorfactory.New(errorfactory.VaultAPIFailure{}, err, "failed to create issuer role for intermediate pki")
	}

	return nil
}
