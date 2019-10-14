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

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	corev1 "k8s.io/api/core/v1"
)

func newMockControllerSecret(valid bool) *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = "test-controller"
	secret.Namespace = "test-namespace"
	cert, key, _, _ := certutil.GenerateTestCert()
	if valid {
		secret.Data = map[string][]byte{
			corev1.TLSCertKey:       cert,
			corev1.TLSPrivateKeyKey: key,
			v1alpha1.CoreCACertKey:  cert,
		}
	}
	return secret
}

func TestGetControllerTLSConfig(t *testing.T) {
	manager := newMock(newMockCluster())

	// Test good controller secret
	manager.client.Create(context.TODO(), newMockControllerSecret(true))
	if _, err := manager.GetControllerTLSConfig(); err != nil {
		t.Error("Expected no error, got:", err)
	}

	manager = newMock(newMockCluster())

	// Test non-existent controller secret
	if _, err := manager.GetControllerTLSConfig(); err == nil {
		t.Error("Expected error got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
		t.Error("Expected not ready error, got:", reflect.TypeOf(err))
	}

	// Test invalid controller secret
	manager.client.Create(context.TODO(), newMockControllerSecret(false))
	if _, err := manager.GetControllerTLSConfig(); err == nil {
		t.Error("Expected error got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.InternalError{}) {
		t.Error("Expected internal error, got:", reflect.TypeOf(err))
	}
}
