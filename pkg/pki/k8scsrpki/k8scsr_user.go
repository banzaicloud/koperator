// Copyright © 2021 Banzai Cloud
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

package k8scsrpki

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"emperror.dev/errors"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/util"

	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"

	//TODO replace this with v1 instead of v1beta1 in all files
	certsigningreqv1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ReconcileUserCertificate ensures and returns a user certificate - should be idempotent
func (c *k8sCSR) ReconcileUserCertificate(
	ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme, clusterDomain string) (*pkicommon.UserCertificate, error) {
	var clientKey []byte
	var signingReq *certsigningreqv1.CertificateSigningRequest
	secret := &corev1.Secret{}
	err := c.client.Get(ctx, types.NamespacedName{Name: user.Spec.SecretName, Namespace: user.Namespace}, secret)
	// Handle case when secret with private key is not found
	if apierrors.IsNotFound(err) {
		clientKey, err = certutil.GeneratePrivateKeyInPemFormat()
		if err != nil {
			return nil, err
		}
		secret = generateUserSecret(clientKey, user.Spec.SecretName, user.GetNamespace())
		err = controllerutil.SetControllerReference(user, secret, scheme)
		if err != nil {
			return nil, err
		}
		err = c.client.Create(ctx, secret)
		if err != nil {
			return nil, err
		}
		// Generate new SigningRequest resource
		//TODO add proper organization field if needed
		signingReq, err := c.createCSR(clientKey, user)
		if err != nil {
			return nil, err
		}
		c.logger.Info("Creating k8s csr object")
		err = c.client.Create(ctx, signingReq)
		if err != nil {
			return nil, err
		}
		secret.Annotations =
			util.MergeAnnotations(secret.Annotations, map[string]string{"banzaicloud.io/depending-csr": signingReq.GetName()})
		typeMeta := secret.TypeMeta
		err = c.client.Update(ctx, secret)
		if err != nil {
			return nil, err
		}
		secret.TypeMeta = typeMeta
	} else if err != nil {
		return nil, err
	}
	signingRequestGenName, ok := secret.Annotations["banzaicloud.io/depending-csr"]
	if !ok {
		// Generate new SigningRequest resource
		//TODO add proper organization field if needed
		signingReq, err := c.generateCSR(secret.Data[corev1.TLSPrivateKeyKey], user)
		if err != nil {
			return nil, err
		}

		c.logger.Info("Creating k8s csr object")
		err = c.client.Create(ctx, signingReq)
		if err != nil {
			return nil, err
		}

		secret.Annotations =
			util.MergeAnnotations(secret.Annotations, map[string]string{"banzaicloud.io/depending-csr": signingReq.GetName()})
		typeMeta := secret.TypeMeta
		err = c.client.Update(ctx, secret)
		if err != nil {
			return nil, err
		}
		secret.TypeMeta = typeMeta
	}
	if signingReq == nil {
		signingReq, err = c.getUserSigningRequest(ctx, signingRequestGenName, secret.GetNamespace())
		// Handle case when signing request is not found
		// as like kubernetes removed the signing request
		if apierrors.IsNotFound(err) {
			// Generate signing request object and create it
			// TODO check if CA is present or not
			if _, ok := secret.Data[corev1.TLSCertKey]; !ok {
				delete(secret.Annotations, "banzaicloud.io/depending-csr")
				typeMeta := secret.TypeMeta
				err = c.client.Update(ctx, secret)
				if err != nil {
					return nil, err
				}
				secret.TypeMeta = typeMeta
				return nil, errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("blalba"), "kubernetes deleted the csr request")
			}
		} else if err != nil {
			return nil, err
		}
	}
	// Handle case when signing request is present
	var foundApproved bool
	for _, cond := range signingReq.Status.Conditions {
		c.logger.Info(fmt.Sprintf("Signing request condition is: %s", cond.Type))
		if cond.Type == certsigningreqv1.CertificateApproved {
			foundApproved = true
			break
		}
	}
	//TODO check what happens if csr fails
	if !foundApproved {
		return nil, errorfactory.New(errorfactory.FatalReconcileError{}, errors.New("blalba"), "could not find approved csr")
	}
	if len(signingReq.Status.Certificate) == 0 {
		return nil, errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("blalba"), "certificate to csr status field is not generated yet")
	}

	secret.Data[corev1.TLSCertKey] = signingReq.Status.Certificate
	typeMeta := secret.TypeMeta
	err = c.client.Update(ctx, secret)
	if err != nil {
		return nil, err
	}
	secret.TypeMeta = typeMeta

	return &pkicommon.UserCertificate{
		CA:          secret.Data[corev1.TLSCertKey],
		Certificate: secret.Data[corev1.TLSCertKey],
		Key:         secret.Data[corev1.TLSPrivateKeyKey],
	}, nil
}

// FinalizeUserCertificate removes/revokes a user certificate
func (c *k8sCSR) FinalizeUserCertificate(ctx context.Context, user *v1alpha1.KafkaUser) error {
	return nil
}

// getUserSigningRequest fetches the k8s signing request for a user
func (c *k8sCSR) getUserSigningRequest(ctx context.Context, name, namespace string) (*certsigningreqv1.CertificateSigningRequest, error) {
	signingRequest := &certsigningreqv1.CertificateSigningRequest{}
	err := c.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, signingRequest)
	return signingRequest, err
}

func generateUserSecret(key []byte, secretName, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: key,
		},
	}
}

func generateCSRResouce(csr []byte, name, namespace, signerName string) *certsigningreqv1.CertificateSigningRequest {
	return &certsigningreqv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name + "-",
			Namespace:    namespace,
			Annotations:  map[string]string{pkicommon.KafkaUserAnnotationName: namespace + "/" + name},
		},
		Spec: certsigningreqv1.CertificateSigningRequestSpec{
			Request:    csr,
			SignerName: util.StringPointer(signerName),
			Usages:     []certsigningreqv1.KeyUsage{certsigningreqv1.UsageServerAuth, certsigningreqv1.UsageClientAuth},
		},
	}
}

func (c *k8sCSR) generateCSR(clientkey []byte, user *v1alpha1.KafkaUser) (*certsigningreqv1.CertificateSigningRequest, error) {
	c.logger.Info("Creating PKCS1PrivateKey from secret")
	block, _ := pem.Decode(clientkey)
	privKey, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
	if parseErr != nil {
		return nil, parseErr
	}
	c.logger.Info("Generating SigningRequest")
	csr, err := certutil.GenerateSigningRequestInPemFormat(privKey, user.GetName(), []string{""})
	if err != nil {
		return nil, err
	}
	c.logger.Info("Generating k8s csr object")
	return generateCSRResouce(csr, user.GetName(), user.GetNamespace(), user.Spec.PKIBackendSpec.SignerName), nil

}
