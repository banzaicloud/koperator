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

package pki

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
)

var log logr.Logger

type mockClient struct {
	client.Client
}

func newMockCluster() *v1beta1.KafkaCluster {
	cluster := &v1beta1.KafkaCluster{}
	cluster.Name = "test"
	cluster.Namespace = "test"
	cluster.Spec = v1beta1.KafkaClusterSpec{}
	cluster.Spec.ListenersConfig = v1beta1.ListenersConfig{}
	cluster.Spec.ListenersConfig.InternalListeners = []v1beta1.InternalListenerConfig{
		{CommonListenerSpec: v1beta1.CommonListenerSpec{
			ContainerPort: 80,
		}},
	}
	cluster.Spec.ListenersConfig.SSLSecrets = &v1beta1.SSLSecrets{
		PKIBackend: MockBackend,
	}
	return cluster
}

func TestGetPKIManager(t *testing.T) {
	cluster := newMockCluster()
	mock := GetPKIManager(&mockClient{}, cluster, v1beta1.PKIBackendProvided)
	if reflect.TypeOf(mock) != reflect.TypeOf(&mockPKIManager{}) {
		t.Error("Expected mock client got:", reflect.TypeOf(mock))
	}
	ctx := context.Background()

	// Test mock functions
	var err error
	if err = mock.ReconcilePKI(ctx, make(map[string]v1beta1.ListenerStatusList)); err != nil {
		t.Error("Expected nil error got:", err)
	}

	if err = mock.FinalizePKI(ctx); err != nil {
		t.Error("Expected nil error got:", err)
	}

	if _, err = mock.ReconcileUserCertificate(ctx, &v1alpha1.KafkaUser{}, scheme.Scheme, "cluster.local"); err != nil {
		t.Error("Expected nil error got:", err)
	}

	if err = mock.FinalizeUserCertificate(ctx, &v1alpha1.KafkaUser{}); err != nil {
		t.Error("Expected nil error got:", err)
	}

	if _, err = mock.GetControllerTLSConfig(); err != nil {
		t.Error("Expected nil error got:", err)
	}

	// Test other getters
	cluster.Spec.ListenersConfig.SSLSecrets.PKIBackend = v1beta1.PKIBackendCertManager
	certmanager := GetPKIManager(&mockClient{}, cluster, v1beta1.PKIBackendProvided)
	pkiType := reflect.TypeOf(certmanager).String()
	expected := "*certmanagerpki.certManager"
	if pkiType != expected {
		t.Error("Expected:", expected, "got:", pkiType)
	}

	// Default should be cert-manager also
	cluster.Spec.ListenersConfig.SSLSecrets.PKIBackend = ""
	certmanager = GetPKIManager(&mockClient{}, cluster, v1beta1.PKIBackendProvided)
	pkiType = reflect.TypeOf(certmanager).String()
	expected = "*certmanagerpki.certManager"
	if pkiType != expected {
		t.Error("Expected:", expected, "got:", pkiType)
	}
}
