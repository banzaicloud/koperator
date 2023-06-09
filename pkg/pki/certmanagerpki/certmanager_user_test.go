// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/util"
	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
)

func newMockUser() *v1alpha1.KafkaUser {
	user := &v1alpha1.KafkaUser{}
	user.Name = "test-user"
	user.Namespace = testNamespace
	user.Spec = v1alpha1.KafkaUserSpec{SecretName: "test-secret", IncludeJKS: true}
	return user
}

func newMockUserSecret() *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = "test-secret"
	secret.Namespace = testNamespace
	cert, key, _, _ := certutil.GenerateTestCert()
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:         cert,
		corev1.TLSPrivateKeyKey:   key,
		v1alpha1.TLSJKSKeyStore:   []byte("testkeystore"),
		v1alpha1.PasswordKey:      []byte("testpassword"),
		v1alpha1.TLSJKSTrustStore: []byte("testtruststore"),
		v1alpha1.CoreCACertKey:    cert,
	}
	return secret
}

func TestFinalizeUserCertificate(t *testing.T) {
	manager, err := newMock(newMockCluster())
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}
	if err := manager.FinalizeUserCertificate(context.Background(), &v1alpha1.KafkaUser{}); err != nil {
		t.Error("Expected no error, got:", err)
	}
}

func TestReconcileUserCertificate(t *testing.T) {
	clusterDomain := "cluster.local"
	manager, err := newMock(newMockCluster())
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}
	ctx := context.Background()

	if err := manager.client.Create(context.TODO(), newMockUser()); err != nil {
		t.Error("Expected no error, got:", err)
	}
	if _, err := manager.ReconcileUserCertificate(ctx, newMockUser(), scheme.Scheme, clusterDomain); err == nil {
		t.Error("Expected resource not ready error, got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
		t.Error("Expected resource not ready error, got:", reflect.TypeOf(err))
	}
	if err := manager.client.Delete(context.TODO(), newMockUserSecret()); err != nil {
		t.Error("could not delete test secret")
	}
	if err := manager.client.Create(context.TODO(), newMockUserSecret()); err != nil {
		t.Error("could not update test secret")
	}
	if _, err := manager.ReconcileUserCertificate(ctx, newMockUser(), scheme.Scheme, clusterDomain); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Test IssuerRef case
	user := newMockUser()
	user.Spec.PKIBackendSpec = &v1alpha1.PKIBackendSpec{
		IssuerRef: &cmmeta.ObjectReference{
			Name:  "test",
			Kind:  "testKind",
			Group: "testGroup",
		},
	}

	if _, err := manager.ReconcileUserCertificate(ctx, user, scheme.Scheme, clusterDomain); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Test cert duration
	user = newMockUser()
	user.Spec.ExpirationSeconds = util.Int32Pointer(7200)

	if _, err := manager.ReconcileUserCertificate(ctx, user, scheme.Scheme, clusterDomain); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Test error conditions
	manager, err = newMock(newMockCluster())
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}
	if err := manager.client.Create(context.TODO(), newMockUser()); err != nil {
		t.Error("Expected no error, got:", err)
	}
	if err := manager.client.Create(context.TODO(),
		manager.clusterCertificateForUser(newMockUser(), clusterDomain)); err != nil {
		t.Error("Expected no error, got:", err)
	}
	if _, err := manager.ReconcileUserCertificate(ctx, newMockUser(), scheme.Scheme, clusterDomain); err == nil {
		t.Error("Expected  error, got nil")
	}
}
