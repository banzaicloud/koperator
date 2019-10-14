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

package certmanagerpki

import (
	"context"
	"fmt"
	"reflect"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/go-logr/logr"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconcile ensures the given kubernetes object
func reconcile(ctx context.Context, log logr.Logger, client client.Client, object runtime.Object, cluster *v1beta1.KafkaCluster) (err error) {
	switch object.(type) {
	case *certv1.ClusterIssuer:
		issuer, _ := object.(*certv1.ClusterIssuer)
		return reconcileClusterIssuer(ctx, log, client, issuer, cluster)
	case *certv1.Certificate:
		cert, _ := object.(*certv1.Certificate)
		return reconcileCertificate(ctx, log, client, cert, cluster)
	case *corev1.Secret:
		secret, _ := object.(*corev1.Secret)
		return reconcileSecret(ctx, log, client, secret, cluster)
	case *v1alpha1.KafkaUser:
		user, _ := object.(*v1alpha1.KafkaUser)
		return reconcileUser(ctx, log, client, user, cluster)
	default:
		panic(fmt.Sprintf("Invalid object type: %v", reflect.TypeOf(object)))
	}
}

// reconcileClusterIssuer ensures a cert-manager ClusterIssuer
func reconcileClusterIssuer(ctx context.Context, log logr.Logger, client client.Client, issuer *certv1.ClusterIssuer, cluster *v1beta1.KafkaCluster) error {
	obj := &certv1.ClusterIssuer{}
	var err error
	if err = client.Get(ctx, types.NamespacedName{Name: issuer.Name, Namespace: issuer.Namespace}, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return client.Create(ctx, issuer)
	}
	return nil
}

// reconcileCertificate ensures a cert-manager certificate
func reconcileCertificate(ctx context.Context, log logr.Logger, client client.Client, cert *certv1.Certificate, cluster *v1beta1.KafkaCluster) error {
	obj := &certv1.Certificate{}
	var err error
	if err = client.Get(ctx, types.NamespacedName{Name: cert.Name, Namespace: cert.Namespace}, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return client.Create(ctx, cert)
	}
	return nil
}

// reconcileSecret ensures a Kubernetes secret
func reconcileSecret(ctx context.Context, log logr.Logger, client client.Client, secret *corev1.Secret, cluster *v1beta1.KafkaCluster) error {
	obj := &corev1.Secret{}
	var err error
	if err = client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return client.Create(ctx, secret)
	}
	return nil
}

// reconcileUser ensures a v1alpha1.KafkaUser
func reconcileUser(ctx context.Context, log logr.Logger, client client.Client, user *v1alpha1.KafkaUser, cluster *v1beta1.KafkaCluster) error {
	obj := &v1alpha1.KafkaUser{}
	var err error
	if err = client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return client.Create(ctx, user)
	}
	return nil
}
