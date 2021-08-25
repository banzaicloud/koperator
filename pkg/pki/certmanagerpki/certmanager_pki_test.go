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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"
)

var log = ctrl.Log.WithName("testing")

const (
	testNamespace = "test-namespace"
)

func newServerSecret() *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = fmt.Sprintf(pkicommon.BrokerServerCertTemplate, "test")
	secret.Namespace = testNamespace
	cert, key, _, _ := certutil.GenerateTestCert()
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       cert,
		corev1.TLSPrivateKeyKey: key,
		v1alpha1.CoreCACertKey:  cert,
	}
	return secret
}

func newControllerSecret() *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = fmt.Sprintf(pkicommon.BrokerControllerTemplate, "test")
	secret.Namespace = testNamespace
	cert, key, _, _ := certutil.GenerateTestCert()
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       cert,
		corev1.TLSPrivateKeyKey: key,
		v1alpha1.CoreCACertKey:  cert,
	}
	return secret
}

func newCASecret() *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = fmt.Sprintf(pkicommon.BrokerCACertTemplate, "test")
	secret.Namespace = "cert-manager"
	cert, key, _, _ := certutil.GenerateTestCert()
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       cert,
		corev1.TLSPrivateKeyKey: key,
		v1alpha1.CoreCACertKey:  cert,
	}
	return secret
}

func newPreCreatedSecret() *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = "test-controller"
	secret.Namespace = testNamespace
	cert, key, _, _ := certutil.GenerateTestCert()
	secret.Data = map[string][]byte{
		v1alpha1.CAPrivateKeyKey: key,
		v1alpha1.CACertKey:       cert,
	}
	return secret
}

func TestFinalizePKI(t *testing.T) {
	manager, err := newMock(newMockCluster())
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}

	if err := manager.FinalizePKI(context.Background(), log); err != nil {
		t.Error("Expected no error on finalize, got:", err)
	}
}

func TestReconcilePKI(t *testing.T) {
	cluster := newMockCluster()
	manager, err := newMock(cluster)
	if err != nil {
		t.Error("Expected no error during initialization, got:", err)
	}
	ctx := context.Background()

	if err := manager.client.Create(ctx, newServerSecret()); err != nil {
		t.Error("error during server secret creation", reflect.TypeOf(err))
	}
	if err := manager.ReconcilePKI(ctx, log, scheme.Scheme, make(map[string]v1beta1.ListenerStatusList)); err != nil {
		if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
			t.Error("Expected not ready error, got:", reflect.TypeOf(err))
		}
	}

	if err := manager.client.Create(ctx, newControllerSecret()); err != nil {
		t.Error("error during controller secret creation", reflect.TypeOf(err))
	}
	if err := manager.ReconcilePKI(ctx, log, scheme.Scheme, make(map[string]v1beta1.ListenerStatusList)); err != nil {
		if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
			t.Error("Expected not ready error, got:", reflect.TypeOf(err))
		}
	}

	if err := manager.client.Create(ctx, newCASecret()); err != nil {
		t.Error("error during CA secret creation", reflect.TypeOf(err))
	}
	if err := manager.ReconcilePKI(ctx, log, scheme.Scheme, make(map[string]v1beta1.ListenerStatusList)); err != nil {
		t.Error("Expected successful reconcile, got:", err)
	}

	cluster.Spec.ListenersConfig.SSLSecrets.Create = false
	manager, err = newMock(cluster)
	if err != nil {
		t.Error("Expected no error during mocking the cluster, got:", err)
	}
	if err := manager.ReconcilePKI(ctx, log, scheme.Scheme, make(map[string]v1beta1.ListenerStatusList)); err == nil {
		t.Error("Expected error got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
		t.Error("Expected not ready error, got:", reflect.TypeOf(err))
	}
	if err := manager.client.Create(ctx, newPreCreatedSecret()); err != nil {
		t.Error("error during pre created secret creation", reflect.TypeOf(err))
	}
	if err := manager.ReconcilePKI(ctx, log, scheme.Scheme, make(map[string]v1beta1.ListenerStatusList)); err != nil {
		t.Error("Expected successful reconcile, got:", err)
	}
}
