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
	"crypto/rand"
	"crypto/x509"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	pkiutil "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// newUserSecret generates a Certificate and signs it using the cluster CA, and returns
// a kubernetes secret with the PEM encoded data.
func newUserSecret(ctx context.Context, client client.Client, cluster *v1beta1.KafkaCluster, user *v1alpha1.KafkaUser, scheme *runtime.Scheme) (*corev1.Secret, error) {
	ca, signingKey, err := getCA(ctx, client, cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errorfactory.New(errorfactory.ResourceNotReady{}, err, "Could not retrieve CA secret")
		}
		return nil, err
	}
	cert := newCert(user.Name, cluster.Name, user.Spec.DNSNames, ca.NotAfter.AddDate(0, 0, -1))
	certPrivKey, err := newKey()
	if err != nil {
		return nil, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, signingKey)
	if err != nil {
		return nil, err
	}

	caPEM, certPEM, keyPEM, err := encodeToPEM(ca, certBytes, certPrivKey)
	if err != nil {
		return nil, err
	}

	userSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            user.Spec.SecretName,
			Namespace:       user.Namespace,
			OwnerReferences: util.OwnerReference(user.TypeMeta, user.ObjectMeta),
		},
		Data: pkiutil.SecretDataForCert(caPEM, certPEM, keyPEM),
	}
	return userSecret, client.Create(ctx, userSecret)
}

// getUserSecret retrieves the kubernetes secret for a given KafkaUser
func getUserSecret(ctx context.Context, client client.Client, user *v1alpha1.KafkaUser) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	o := types.NamespacedName{Name: user.Spec.SecretName, Namespace: user.Namespace}
	return secret, client.Get(ctx, o, secret)
}
