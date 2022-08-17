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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
)

func newMockControllerSecret(valid bool) *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = "test-controller"
	secret.Namespace = testNamespace
	cert, key, _, _ := certutil.GenerateTestCert()
	keystore_jks, password, _ := certutil.GenerateJKSFromByte(cert, key, cert)

	if valid {
		secret.Data = map[string][]byte{
			v1alpha1.PasswordKey:      password,
			v1alpha1.TLSJKSTrustStore: keystore_jks,
			v1alpha1.TLSJKSKeyStore:   keystore_jks,
		}
	}
	return secret
}

func TestGetControllerTLSConfig(t *testing.T) {
	manager, err := newMock(newMockCluster())
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}
	// Test good controller secret
	if err := manager.client.Create(context.TODO(), newMockControllerSecret(true)); err != nil {
		t.Error("Expected no error, got:", err)
	}
	if _, err := manager.GetControllerTLSConfig(); err != nil {
		t.Error("Expected no error, got:", err)
	}

	manager, err = newMock(newMockCluster())
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}
	// Test non-existent controller secret
	if _, err := manager.GetControllerTLSConfig(); err == nil {
		t.Error("Expected error got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
		t.Error("Expected not ready error, got:", reflect.TypeOf(err))
	}

	// Test invalid controller secret
	if err := manager.client.Create(context.TODO(), newMockControllerSecret(false)); err != nil {
		t.Error("Expected no error, got:", err)
	}
	if _, err := manager.GetControllerTLSConfig(); err == nil {
		t.Error("Expected error got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.InternalError{}) {
		t.Error("Expected internal error, got:", reflect.TypeOf(err))
	}
}
