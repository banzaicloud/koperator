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

func (v *vaultPKI) FinalizePKI(ctx context.Context, logger logr.Logger) error {
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

	return nil
}

// ReconcilePKI will use the user-provided paths to ensure broker/operator users for a new KafkaCluster
func (v *vaultPKI) ReconcilePKI(ctx context.Context, logger logr.Logger, scheme *runtime.Scheme) (err error) {
	log := logger.WithName("vault_pki")

	log.Info("Retrieving vault client")
	vault, err := v.getClient()
	if err != nil {
		return
	}

	log.Info("Reconciling broker user in vault")
	brokerCert, err := v.reconcileBrokerCert(ctx, vault)
	if err != nil {
		return
	}

	log.Info("Reconciling controller user in vault")
	controllerCert, err := v.reconcileControllerCert(ctx, vault)
	if err != nil {
		return
	}

	return v.reconcileBootstrapSecrets(ctx, scheme, brokerCert, controllerCert)
}

// reconcileBootstrapSecrets creates the secret mounts for cruise control and the kafka brokers
func (v *vaultPKI) reconcileBootstrapSecrets(ctx context.Context, scheme *runtime.Scheme, brokerCert, controllerCert *pkicommon.UserCertificate) (err error) {
	serverSecret := &corev1.Secret{}
	clientSecret := &corev1.Secret{}

	toCreate := make([]*corev1.Secret, 0)

	// The server keystore volume
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

	// the cruise control keystore volume
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
		if err = v.client.Create(ctx, o); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "failed to create bootstrap secret", "secret", o.Name)
		}
	}

	return
}

func (v *vaultPKI) reconcileBrokerCert(ctx context.Context, vault *vaultapi.Client) (*pkicommon.UserCertificate, error) {
	return v.reconcileStartupUser(ctx, vault, pkicommon.BrokerUserForCluster(v.cluster))
}

func (v *vaultPKI) reconcileControllerCert(ctx context.Context, vault *vaultapi.Client) (*pkicommon.UserCertificate, error) {
	return v.reconcileStartupUser(ctx, vault, pkicommon.ControllerUserForCluster(v.cluster))
}

func (v *vaultPKI) reconcileStartupUser(ctx context.Context, vault *vaultapi.Client, user *v1alpha1.KafkaUser) (cert *pkicommon.UserCertificate, err error) {
	existing := &v1alpha1.KafkaUser{}
	if err := v.client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			if err = v.client.Create(ctx, user); err != nil {
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
