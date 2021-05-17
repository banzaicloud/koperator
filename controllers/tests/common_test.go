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

package tests

import (
	"context"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
)

const defaultBrokerConfigGroup = "default"

func createMinimalKafkaClusterCR(name, namespace string) *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.KafkaClusterSpec{
			ListenersConfig: v1beta1.ListenersConfig{
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Name:          "test",
							ContainerPort: 9094,
							Type:          "plaintext",
						},
						ExternalStartingPort: 19090,
						IngressServiceSettings: v1beta1.IngressServiceSettings{
							HostnameOverride: "test-host",
						},
						AccessMethod: corev1.ServiceTypeLoadBalancer,
					},
				},
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:                            "plaintext",
							Name:                            "internal",
							ContainerPort:                   29092,
							UsedForInnerBrokerCommunication: true,
						},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:                            "plaintext",
							Name:                            "controller",
							ContainerPort:                   29093,
							UsedForInnerBrokerCommunication: false,
						},
						UsedForControllerCommunication: true,
					},
				},
			},
			BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
				defaultBrokerConfigGroup: {
					StorageConfigs: []v1beta1.StorageConfig{
						{
							MountPath: "/kafka-logs",
							PvcSpec: &corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceStorage: resource.MustParse("10Gi"),
									},
								},
							},
						},
					},
				},
			},
			Brokers: []v1beta1.Broker{
				{
					Id:                0,
					BrokerConfigGroup: defaultBrokerConfigGroup,
				},
				{
					Id:                1,
					BrokerConfigGroup: defaultBrokerConfigGroup,
				},
				{
					Id:                2,
					BrokerConfigGroup: defaultBrokerConfigGroup,
				},
			},
			ClusterImage: "ghcr.io/banzaicloud/kafka:2.13-2.8.1",
			ZKAddresses:  []string{},
			MonitoringConfig: v1beta1.MonitoringConfig{
				CCJMXExporterConfig: "custom_property: custom_value",
			},
			ReadOnlyConfig: "cruise.control.metrics.topic.auto.create=true",
		},
	}
}

func waitForClusterRunningState(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan struct{}, 1)

	treshold := 10
	consecutiveRunningState := 0

	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			select {
			case <-ctx.Done():
				return
			default:
				createdKafkaCluster := &v1beta1.KafkaCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: kafkaCluster.Name, Namespace: namespace}, createdKafkaCluster)
				if err != nil || createdKafkaCluster.Status.State != v1beta1.KafkaClusterRunning {
					consecutiveRunningState = 0
					continue
				}
				consecutiveRunningState++
				if consecutiveRunningState > treshold {
					ch <- struct{}{}
					return
				}
			}
		}
	}()

	Eventually(ch, 120*time.Second, 50*time.Millisecond).Should(Receive())
}

func getMockedKafkaClientForCluster(kafkaCluster *v1beta1.KafkaCluster) (kafkaclient.KafkaClient, func()) {
	name := types.NamespacedName{
		Name:      kafkaCluster.Name,
		Namespace: kafkaCluster.Namespace,
	}
	if val, ok := mockKafkaClients[name]; ok {
		return val, func() { val.Close() }
	}
	mockKafkaClient, _, _ := kafkaclient.NewMockFromCluster(k8sClient, kafkaCluster)
	mockKafkaClients[name] = mockKafkaClient
	return mockKafkaClient, func() { mockKafkaClient.Close() }
}

func resetMockKafkaClient(kafkaCluster *v1beta1.KafkaCluster) {
	// delete all topics
	mockKafkaClient, _ := getMockedKafkaClientForCluster(kafkaCluster)
	topics, _ := mockKafkaClient.ListTopics()
	for topicName := range topics {
		_ = mockKafkaClient.DeleteTopic(topicName, false)
	}

	// delete all acls
	_ = mockKafkaClient.DeleteUserACLs("")
}
