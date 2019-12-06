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

package internalpki

import (
	"context"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// ReconcileUserCertificate ensures and returns a user TLS certificate
func (i *internalPKI) ReconcileUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme) (*pkicommon.UserCertificate, error) {
	secret, err := getUserSecret(ctx, i.client, user)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		if secret, err = newUserSecret(ctx, i.client, i.cluster, user, scheme); err != nil {
			return nil, err
		}
	}
	if user.Spec.IncludeJKS {
		if secret, err = i.injectJKS(ctx, user, secret); err != nil {
			return nil, err
		}
	}
	return &pkicommon.UserCertificate{
		CA:          secret.Data[v1alpha1.CoreCACertKey],
		Certificate: secret.Data[corev1.TLSCertKey],
		Key:         secret.Data[corev1.TLSPrivateKeyKey],
	}, nil
}

// FinalizeUserCertificate returns nil as owner reference on secret handles cleanup
func (i *internalPKI) FinalizeUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser) error {
	return nil
}

// injectJKS ensures that a secret contains JKS format when requested
func (i *internalPKI) injectJKS(ctx context.Context, user *v1alpha1.KafkaUser, secret *corev1.Secret) (*corev1.Secret, error) {
	var err error
	if secret, err = certutil.EnsureSecretJKS(secret); err != nil {
		return nil, errorfactory.New(errorfactory.InternalError{}, err, "could not inject secret with jks")
	}
	if err = i.client.Update(ctx, secret); err != nil {
		return nil, errorfactory.New(errorfactory.APIFailure{}, err, "could not update secret with jks")
	}
	return secret, nil
}
