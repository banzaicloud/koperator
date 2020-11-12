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

package vaultpki

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/builtin/logical/pki"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

var log = logf.Log.WithName("testing")

func newMockCluster() *v1beta1.KafkaCluster {
	cluster := &v1beta1.KafkaCluster{}
	cluster.Name = "test"
	cluster.Namespace = "test-namespace"
	cluster.UID = "test-uid"
	cluster.Spec = v1beta1.KafkaClusterSpec{}
	cluster.Spec.ListenersConfig = v1beta1.ListenersConfig{}
	cluster.Spec.ListenersConfig.InternalListeners = []v1beta1.InternalListenerConfig{
		{CommonListenerSpec: v1beta1.CommonListenerSpec{
			ContainerPort: 9092,
		}},
	}
	cluster.Spec.ListenersConfig.SSLSecrets = &v1beta1.SSLSecrets{
		TLSSecretName:   "test-controller",
		JKSPasswordName: "test-password",
		PKIBackend:      v1beta1.PKIBackendVault,
		Create:          true,
	}
	cluster.Spec.VaultConfig = v1beta1.VaultConfig{
		AuthRole:  "", // will be mocked
		PKIPath:   "pki_kafka/",
		IssuePath: "pki_kafka/issue/operator",
		UserStore: "kafka_users/",
	}
	return cluster
}

func newVaultMock(t *testing.T) (*vaultPKI, net.Listener, *api.Client) {
	t.Helper()

	v1beta1.AddToScheme(scheme.Scheme)

	ln, client := createTestVault(t)

	return &vaultPKI{
		cluster:   newMockCluster(),
		client:    fake.NewFakeClientWithScheme(scheme.Scheme),
		getClient: func() (*api.Client, error) { return client, nil },
	}, ln, client
}

func createTestVault(t *testing.T) (net.Listener, *api.Client) {
	t.Helper()

	// Create an in-memory core
	config := &vault.CoreConfig{
		LogicalBackends: map[string]logical.Factory{
			"pki": pki.Factory,
		},
	}
	core, keyShares, rootToken := vault.TestCoreUnsealedWithConfig(t, config)
	_ = keyShares

	// Start an HTTP server for the core.
	ln, addr := http.TestServer(t, core)

	// Create a client that talks to the server
	conf := api.DefaultConfig()
	conf.Address = addr

	client, err := api.NewClient(conf)
	if err != nil {
		t.Fatal(err)
	}
	client.SetToken(rootToken)

	client.Sys().Mount(
		"pki_kafka/",
		&api.MountInput{
			Type: "pki",
		},
	)

	client.Sys().Mount(
		"kafka_users/",
		&api.MountInput{
			Type: "kv",
		},
	)

	client.Logical().Write(
		"pki_kafka/root/generate/internal",
		map[string]interface{}{
			vaultCommonNameArg: "kafkaca.kafka.svc.cluster.local",
			vaultTTLArg:        "215000h",
		},
	)

	client.Logical().Write(
		"pki_kafka/roles/operator",
		map[string]interface{}{
			"allow_localhost": true,
			"allowed_domains": "*",
			"allow_any_name":  true,
		},
	)
	return ln, client
}

type mockClient struct {
	client.Client
}

func TestNew(t *testing.T) {
	pkiManager := New(&mockClient{}, newMockCluster())
	if reflect.TypeOf(pkiManager) != reflect.TypeOf(&vaultPKI{}) {
		t.Error("Expected new certmanager from New, got:", reflect.TypeOf(pkiManager))
	}
}

func TestAll(t *testing.T) {
	clusterDomain := "cluster.local"
	ctx := context.Background()
	mock, ln, client := newVaultMock(t)
	defer ln.Close()

	cert, key, _, err := certutil.GenerateTestCert()
	if err != nil {
		t.Fatal("Failed to create test cert")
	}
	jks, passw, err := certutil.GenerateJKS(cert, key, cert)
	if err != nil {
		t.Fatal("Failed to convert test cert to JKS")
	}

	if err := mock.ReconcilePKI(ctx, log, scheme.Scheme, map[string]string{}); err == nil {
		t.Error("Expected resource not ready, got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
		t.Error("Expected resource not ready, got:", err)
	}

	brokerPath := fmt.Sprintf("secret/%s", fmt.Sprintf(pkicommon.BrokerServerCertTemplate, mock.cluster.Name))
	controllerPath := fmt.Sprintf("secret/%s", fmt.Sprintf(pkicommon.BrokerControllerTemplate, mock.cluster.Name))
	client.Logical().Write(brokerPath, dataForUserCert(&pkicommon.UserCertificate{
		Certificate: cert,
		Key:         key,
		CA:          cert,
		JKS:         jks,
		Password:    passw,
	}))
	client.Logical().Write(controllerPath, dataForUserCert(&pkicommon.UserCertificate{
		Certificate: cert,
		Key:         key,
		CA:          cert,
		JKS:         jks,
		Password:    passw,
	}))

	// Should be safe to do multiple times
	if err := mock.ReconcilePKI(ctx, log, scheme.Scheme, map[string]string{}); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Tear down PKI
	if err := mock.FinalizePKI(ctx, log); err != nil {
		t.Error("Expected no error, got:", err)
	}

	if err := mock.ReconcilePKI(ctx, log, scheme.Scheme, map[string]string{}); err != nil {
		t.Error("Expected no error, got:", err)
	}

	if _, err := mock.GetControllerTLSConfig(); err != nil {
		t.Error("Expected no error, got:", err)
	}

	if _, err := mock.ReconcileUserCertificate(ctx, newMockUser(), scheme.Scheme, clusterDomain); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Safe to do multiple times
	if _, err := mock.ReconcileUserCertificate(ctx, newMockUser(), scheme.Scheme, clusterDomain); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Finalize PKI should block with an existing user
	if err := mock.FinalizePKI(ctx, log); err == nil {
		t.Error("Expected error trying to tear down non-empty PKI, got nil")
	} else if reflect.TypeOf(err) != reflect.TypeOf(errorfactory.ResourceNotReady{}) {
		t.Error("Expected resource not read err, got:", reflect.TypeOf(err))
	}

	if err := mock.FinalizeUserCertificate(ctx, newMockUser()); err != nil {
		t.Error("Expected no error, got:", err)
	}

	// Tear down PKI should go through now
	if err := mock.FinalizePKI(ctx, log); err != nil {
		t.Error("Expected no error, got:", err)
	}
}
