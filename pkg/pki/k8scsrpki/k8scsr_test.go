// Copyright © 2021 Banzai Cloud
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

package k8scsrpki

import (
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/koperator/api/v1beta1"
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
		{
			CommonListenerSpec: v1beta1.CommonListenerSpec{
				ContainerPort: 9092,
			},
		},
	}
	cluster.Spec.ListenersConfig.SSLSecrets = &v1beta1.SSLSecrets{
		PKIBackend: v1beta1.PKIBackendK8sCSR,
	}
	return cluster
}

func TestNew(t *testing.T) {
	pkiManager := New(&mockClient{}, newMockCluster(), log.Log)
	if reflect.TypeOf(pkiManager) != reflect.TypeOf(&k8sCSR{}) {
		t.Error("Expected new k8sCSR from New, got:", reflect.TypeOf(pkiManager))
	}
}
