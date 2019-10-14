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
	"reflect"
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type mockClient struct {
	client.Client
}

func newMockCluster() *v1beta1.KafkaCluster {
	cluster := &v1beta1.KafkaCluster{}
	cluster.Name = "test"
	cluster.Namespace = "test-namespace"
	cluster.Spec = v1beta1.KafkaClusterSpec{}
	cluster.Spec.ListenersConfig = v1beta1.ListenersConfig{}
	cluster.Spec.ListenersConfig.InternalListeners = []v1beta1.InternalListenerConfig{
		{ContainerPort: 9092},
	}
	cluster.Spec.ListenersConfig.SSLSecrets = &v1beta1.SSLSecrets{
		TLSSecretName:   "test-controller",
		JKSPasswordName: "test-password",
		PKIBackend:      v1beta1.PKIBackendCertManager,
		Create:          true,
	}
	return cluster
}

func newMock(cluster *v1beta1.KafkaCluster) *certManager {
	certv1.AddToScheme(scheme.Scheme)
	v1alpha1.AddToScheme(scheme.Scheme)
	v1beta1.AddToScheme(scheme.Scheme)
	return &certManager{
		cluster: cluster,
		client:  fake.NewFakeClientWithScheme(scheme.Scheme),
	}
}

func TestNew(t *testing.T) {
	pkiManager := New(&mockClient{}, newMockCluster())
	if reflect.TypeOf(pkiManager) != reflect.TypeOf(&certManager{}) {
		t.Error("Expected new certmanager from New, got:", reflect.TypeOf(pkiManager))
	}
}
