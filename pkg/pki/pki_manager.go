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
	"crypto/tls"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/pki/certmanagerpki"
	"github.com/banzaicloud/kafka-operator/pkg/pki/vaultpki"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockBackend is used for mocking during testing
var MockBackend = v1beta1.PKIBackend("mock")

// GetPKIManager returns a PKI/User manager interface for a given cluster
func GetPKIManager(client client.Client, cluster *v1beta1.KafkaCluster) pkicommon.PKIManager {
	switch cluster.Spec.ListenersConfig.SSLSecrets.PKIBackend {

	// Use cert-manager for pki backend
	case v1beta1.PKIBackendCertManager:
		return certmanagerpki.New(client, cluster)

	// Use vault for pki backend
	case v1beta1.PKIBackendVault:
		return vaultpki.New(client, cluster)

	// Return mock backend for testing - cannot be triggered by CR due to enum in api schema
	case MockBackend:
		return newMockPKIManager(client, cluster)

	// Default use cert-manager - state explicitly for clarity and to make compiler happy
	default:
		return certmanagerpki.New(client, cluster)

	}
}

// Mock types

type mockPKIManager struct {
	pkicommon.PKIManager
	client  client.Client
	cluster *v1beta1.KafkaCluster
}

func newMockPKIManager(client client.Client, cluster *v1beta1.KafkaCluster) pkicommon.PKIManager {
	return &mockPKIManager{client: client, cluster: cluster}
}

func (m *mockPKIManager) ReconcilePKI(logger logr.Logger, scheme *runtime.Scheme) error {
	return nil
}

func (m *mockPKIManager) FinalizePKI(logger logr.Logger) error {
	return nil
}

func (m *mockPKIManager) ReconcileUserCertificate(user *v1alpha1.KafkaUser, scheme *runtime.Scheme) (*pkicommon.UserCertificate, error) {
	return &pkicommon.UserCertificate{}, nil
}

func (m *mockPKIManager) FinalizeUserCertificate(user *v1alpha1.KafkaUser) error {
	return nil
}

func (m *mockPKIManager) GetControllerTLSConfig() (*tls.Config, error) {
	return &tls.Config{}, nil
}
