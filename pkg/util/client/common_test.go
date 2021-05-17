// Copyright © 2020 Banzai Cloud
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

package client

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func TestGenerateKafkaAddress(t *testing.T) {
	cluster := &v1beta1.KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: v1beta1.KafkaClusterSpec{
			HeadlessServiceEnabled: true,
			ListenersConfig: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							ContainerPort:                   80,
							UsedForInnerBrokerCommunication: true,
						},
					},
				},
			},
		},
	}

	generatedHeadlessWithoutPort := GenerateKafkaAddressWithoutPort(cluster)
	expected := "test-headless.test.svc.cluster.local"
	if generatedHeadlessWithoutPort != expected {
		t.Error("Expected kafka address:", expected, "Got:", generatedHeadlessWithoutPort)
	}

	generatedHeadless := GenerateKafkaAddress(cluster)
	expected = "test-headless.test.svc.cluster.local:80"
	if generatedHeadless != expected {
		t.Error("Expected kafka address:", expected, "Got:", generatedHeadless)
	}

	cluster.Spec.HeadlessServiceEnabled = false
	generatedAllBrokerWithoutPort := GenerateKafkaAddressWithoutPort(cluster)
	expected = "test-all-broker.test.svc.cluster.local"
	if generatedAllBrokerWithoutPort != expected {
		t.Error("Expected kafka address:", expected, "Got:", generatedAllBrokerWithoutPort)
	}

	generatedAllBroker := GenerateKafkaAddress(cluster)
	expected = "test-all-broker.test.svc.cluster.local:80"
	if generatedAllBroker != expected {
		t.Error("Expected kafka address:", expected, "Got:", generatedAllBroker)
	}
}
