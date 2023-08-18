// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"

	"github.com/banzaicloud/k8s-objectmatcher/patch"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/util"

	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certsigningreqv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	csrclient "k8s.io/client-go/kubernetes/typed/certificates/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	notApprovedErrMsg                             = "instance is not approved"
	notFoundApprovedCsrErrMsg                     = "could not find approved csr and the operator is not capable of approving the csr"
	notFoundCAInClusterIssuerErrMsg               = "could not extract CA from ClusterIssuer"
	notFoundCertManagerSecretField                = "could not find certificate field in cert-manager Secret"
	approveReason                                 = "ApprovedByPolicy"
	defaultCertManagerIssuerSecretCertificateFile = "tls.crt"
)

// ReconcileUserCertificate ensures and returns a user certificate - should be idempotent
func (c *k8sCSR) ReconcileUserCertificate(
	ctx context.Context, user *v1alpha1.KafkaUser, scheme *runtime.Scheme, _ string) (*pkicommon.UserCertificate, error) {
	log := logr.FromContextOrDiscard(ctx)
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
		if err = patch.DefaultAnnotator.SetLastAppliedAnnotation(secret); err != nil {
			return nil, errors.WrapIf(err, "could not apply last state to annotation")
		}
		err = c.client.Create(ctx, secret)
		if err != nil {
			return nil, err
		}
		// Generate new SigningRequest resource
		signingReq, err = c.generateAndCreateCSR(ctx, clientKey, user)
		if err != nil {
			return nil, err
		}

		if err = c.secretUpdateAnnotation(ctx, secret, signingReq.GetName()); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, errors.WrapIfWithDetails(err,
			"failed to get user's secret from K8s", "secretName", user.Spec.SecretName,
			"namespace", user.GetNamespace())
	}
	// Check if the secret has the proper ownerref
	ownerRef := secret.GetOwnerReferences()
	isUserOwnedSecret := false
	for _, ref := range ownerRef {
		if ref.Kind == user.Kind && ref.Name == user.Name {
			isUserOwnedSecret = true
			break
		}
	}
	if !isUserOwnedSecret {
		return nil, errors.New(fmt.Sprintf("secret: %s does not belong to this KafkaUser", secret.Name))
	}

	// skip handling CSR if the secret already includes all the required fields
	kafkaUserSecretReady := isKafkaUserCertificateReady(secret, user.Spec.IncludeJKS)
	if kafkaUserSecretReady {
		return &pkicommon.UserCertificate{
			CA:          secret.Data[v1alpha1.CaChainPem],
			Certificate: secret.Data[corev1.TLSCertKey],
			Key:         secret.Data[corev1.TLSPrivateKeyKey],
			JKS:         secret.Data[v1alpha1.TLSJKSKeyStore],
			Password:    secret.Data[v1alpha1.PasswordKey],
		}, nil
	}

	signingRequestGenName, ok := secret.Annotations[DependingCsrAnnotation]
	if !ok {
		// Generate new SigningRequest resource
		signingReq, err = c.generateAndCreateCSR(ctx, secret.Data[corev1.TLSPrivateKeyKey], user)
		if err != nil {
			return nil, err
		}

		if err = c.secretUpdateAnnotation(ctx, secret, signingReq.GetName()); err != nil {
			return nil, err
		}
	}
	if signingReq == nil {
		signingReq, err = c.getUserSigningRequest(ctx, signingRequestGenName, secret.GetNamespace())
		// Handle case when signing request is not found
		// as like kubernetes removed the signing request
		if apierrors.IsNotFound(err) {
			// Generate signing request object and create it
			if _, ok := secret.Data[corev1.TLSCertKey]; !ok {
				delete(secret.Annotations, DependingCsrAnnotation)
				typeMeta := secret.TypeMeta
				err = c.client.Update(ctx, secret)
				if err != nil {
					return nil, err
				}
				secret.TypeMeta = typeMeta
				return nil, errorfactory.New(errorfactory.ResourceNotReady{},
					errors.New("instance not found"), "kubernetes deleted the csr request",
					"csrName", signingRequestGenName)
			}
		} else if err != nil {
			return nil, errors.WrapIfWithDetails(err,
				"failed to get signing request from K8s", "signingRequestName", signingRequestGenName,
				"namespace", secret.GetNamespace())
		}
	}
	// Handle case when signing request is present
	var foundApproved bool
	for _, cond := range signingReq.Status.Conditions {
		log.Info(fmt.Sprintf("Signing request condition is: %s", cond.Type))
		if cond.Type == certsigningreqv1.CertificateApproved {
			foundApproved = true
			break
		}
	}

	if !foundApproved {
		if strings.Split(signingReq.Spec.SignerName, "/")[0] == v1alpha1.CertManagerSignerNamePrefix {
			err = c.Approve(ctx, signingReq)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errorfactory.New(errorfactory.FatalReconcileError{}, errors.New(notApprovedErrMsg),
				notFoundApprovedCsrErrMsg, "csrName", signingReq.GetName())
		}
	}
	if len(signingReq.Status.Certificate) == 0 {
		return nil, errorfactory.New(errorfactory.ResourceNotReady{},
			errors.New("instance is not ready yet"), "certificate to csr status field is not generated yet",
			"csrName", signingReq.GetName())
	}

	certs, err := certutil.ParseCertificates(signingReq.Status.Certificate)
	if err != nil {
		return nil, err
	}

	//Leaf cert
	secret.Data[corev1.TLSCertKey] = certs[0].ToPEM()
	//CA chain certs
	caChain, err := c.getCAChain(ctx, signingReq, certs)
	if err != nil {
		return nil, err
	}

	secret.Data[v1alpha1.CaChainPem] = caChain
	certBundleX509 := certutil.GetCertBundle(certs)

	// Ensure a JKS if requested
	if user.Spec.IncludeJKS {
		// we don't have an existing one - make a new one
		if value, ok := secret.Data[v1alpha1.TLSJKSKeyStore]; !ok || len(value) == 0 {
			jks, jksPasswd, err := certutil.GenerateJKS(certBundleX509, secret.Data[corev1.TLSPrivateKeyKey])
			if err != nil {
				return nil, err
			}
			secret.Data[v1alpha1.TLSJKSKeyStore] = jks
			// Adding Truststore to the secret to align with the Cert Manager generated secret
			secret.Data[v1alpha1.TLSJKSTrustStore] = jks
			secret.Data[v1alpha1.PasswordKey] = jksPasswd
		}
	}

	typeMeta := secret.TypeMeta
	err = c.client.Update(ctx, secret)
	if err != nil {
		return nil, err
	}
	secret.TypeMeta = typeMeta

	return &pkicommon.UserCertificate{
		CA:          secret.Data[v1alpha1.CaChainPem],
		Certificate: secret.Data[corev1.TLSCertKey],
		Key:         secret.Data[corev1.TLSPrivateKeyKey],
		JKS:         secret.Data[v1alpha1.TLSJKSKeyStore],
		Password:    secret.Data[v1alpha1.PasswordKey],
	}, nil
}

// FinalizeUserCertificate removes/revokes a user certificate
func (c *k8sCSR) FinalizeUserCertificate(_ context.Context, _ *v1alpha1.KafkaUser) error {
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

func generateCSRResource(csr []byte, name, namespace, signerName string,
	expirationSeconds int32, annotation map[string]string) *certsigningreqv1.CertificateSigningRequest {
	owner := types.NamespacedName{Namespace: namespace, Name: name}
	return &certsigningreqv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name + "-",
			Annotations: util.MergeAnnotations(annotation,
				map[string]string{pkicommon.KafkaUserAnnotationName: owner.String(), IncludeFullChainAnnotation: "true"}),
		},
		Spec: certsigningreqv1.CertificateSigningRequestSpec{
			Request:           csr,
			SignerName:        signerName,
			Usages:            []certsigningreqv1.KeyUsage{certsigningreqv1.UsageServerAuth, certsigningreqv1.UsageClientAuth},
			ExpirationSeconds: &expirationSeconds,
		},
	}
}

func (c *k8sCSR) generateAndCreateCSR(ctx context.Context, clientkey []byte, user *v1alpha1.KafkaUser) (*certsigningreqv1.CertificateSigningRequest, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.Info("Creating PKCS1PrivateKey from secret")
	block, _ := pem.Decode(clientkey)
	privKey, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
	if parseErr != nil {
		return nil, parseErr
	}
	log.Info("Generating SigningRequest")
	csr, err := certutil.GenerateSigningRequestInPemFormat(privKey, user.GetName(), user.Spec.DNSNames)
	if err != nil {
		return nil, err
	}
	log.Info("Generating k8s csr object")
	signingReq := generateCSRResource(csr, user.GetName(), user.GetNamespace(),
		user.Spec.PKIBackendSpec.SignerName, user.Spec.GetExpirationSeconds(), user.Spec.GetAnnotations())
	log.Info("Creating k8s csr object")
	if err = patch.DefaultAnnotator.SetLastAppliedAnnotation(signingReq); err != nil {
		return nil, errors.WrapIf(err, "could not apply last state to annotation")
	}
	err = c.client.Create(ctx, signingReq)
	if err != nil {
		return nil, err
	}
	return signingReq, nil
}
func (c *k8sCSR) secretUpdateAnnotation(ctx context.Context, secret *corev1.Secret, srName string) error {
	secret.Annotations =
		util.MergeAnnotations(secret.Annotations, map[string]string{DependingCsrAnnotation: srName})
	typeMeta := secret.TypeMeta
	err := c.client.Update(ctx, secret)
	if err != nil {
		return err
	}
	secret.TypeMeta = typeMeta
	return nil
}

func isKafkaUserCertificateReady(secret *corev1.Secret, includeJKS bool) bool {
	requiredFields := []string{corev1.TLSCertKey, v1alpha1.CaChainPem}
	if includeJKS {
		requiredFields = append(requiredFields, v1alpha1.TLSJKSKeyStore, v1alpha1.PasswordKey)
	}
	for _, field := range requiredFields {
		if _, ok := secret.Data[field]; !ok {
			return false
		}
	}

	return true
}

// Approve approves certificate signing requests
func (c *k8sCSR) Approve(ctx context.Context, signingReq *certsigningreqv1.CertificateSigningRequest) error {
	cond := certsigningreqv1.CertificateSigningRequestCondition{
		Type:    certsigningreqv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  approveReason,
		Message: "CSR has been approved by Koperator",
	}
	signingReq.Status.Conditions = append(signingReq.Status.Conditions, cond)

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return err
	}
	csrClient := csrclient.NewForConfigOrDie(restConfig).CertificateSigningRequests()

	signingReq, err = csrClient.UpdateApproval(ctx, signingReq.Name, signingReq, metav1.UpdateOptions{}) //nolint:staticcheck
	if err != nil {
		return err
	}

	return nil
}

func (c *k8sCSR) getCAChain(ctx context.Context, signingReq *certsigningreqv1.CertificateSigningRequest, certs []*certutil.CertificateContainer) ([]byte, error) {
	var caChain []byte
	signerName := strings.Split(signingReq.Spec.SignerName, "/")
	if len(signerName) < 2 { // Note: [signerNamePrefix, clusterIssuerName]
		return nil, errors.NewWithDetails("invalid signer name", "signerName", signingReq.Spec.SignerName)
	}

	if signerName[0] == v1alpha1.CertManagerSignerNamePrefix {
		clusterIssuer := &certv1.ClusterIssuer{}
		clusterIssuerName := signerName[1]
		err := c.client.Get(ctx, types.NamespacedName{
			Name: clusterIssuerName,
		}, clusterIssuer)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err,
				"failed to get ClusterIssuer from K8s", "clusterIssuer", clusterIssuerName)
		}

		if clusterIssuer.GetSpec().CA == nil {
			return nil, errorfactory.New(errorfactory.FatalReconcileError{}, errors.New(notFoundCAInClusterIssuerErrMsg),
				"clusterIssuer doesn't contain CA secret reference", "clusterIssuer", clusterIssuerName)
		}

		certManagerSecret := &corev1.Secret{}
		err = c.client.Get(ctx, types.NamespacedName{
			Name:      clusterIssuer.GetSpec().CA.SecretName,
			Namespace: pkicommon.NamespaceCertManager,
		}, certManagerSecret)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err,
				"failed to get secret from K8s", "secretName", clusterIssuer.GetSpec().CA.SecretName,
				"namespace", certManagerSecret.GetNamespace())
		}

		chain, ok := certManagerSecret.Data[defaultCertManagerIssuerSecretCertificateFile]
		if !ok {
			return caChain, errorfactory.New(errorfactory.FatalReconcileError{}, errors.New(notFoundCertManagerSecretField),
				"failed to get field", "secretName", clusterIssuer.GetSpec().CA.SecretName,
				"namespace", certManagerSecret.GetNamespace(), "field", defaultCertManagerIssuerSecretCertificateFile)
		}
		caChain = chain
	} else {
		for _, cr := range certs {
			if cr.Certificate.IsCA {
				caChain = append(caChain, cr.ToPEM()...)
				caChain = append(caChain, byte('\n'))
			}
		}
	}

	return caChain, nil
}
