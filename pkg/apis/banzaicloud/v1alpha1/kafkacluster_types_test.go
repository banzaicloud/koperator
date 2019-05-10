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

package v1alpha1

import (
	"testing"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorageKafkaCluster(t *testing.T) {
	key := types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}
	created := &KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: KafkaClusterSpec{
			ZKAddresses: []string{"example.zk:2181"},
			BrokerConfigs: []BrokerConfig{
				{
					Image: "banzaicloud/kafka-operator:latest",
					Id:    int32(0),
					StorageConfigs: []StorageConfig{
						{
							MountPath: "/kafka-logs",
							PVCSpec:   &corev1.PersistentVolumeClaimSpec{},
						},
					},
				},
			},
			ListenersConfig: ListenersConfig{
				ExternalListeners: []ExternalListenerConfig{
					{
						Type:                 "plaintext",
						Name:                 "external",
						ExternalStartingPort: 9090,
						ContainerPort:        9094,
					},
				},
				InternalListeners: []InternalListenerConfig{
					{
						Type:                            "plaintext",
						Name:                            "plaintext",
						ContainerPort:                   29092,
						UsedForInnerBrokerCommunication: true,
					},
				},
			},
			ServiceAccount: "",
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &KafkaCluster{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), key, fetched)).To(gomega.HaveOccurred())
}
