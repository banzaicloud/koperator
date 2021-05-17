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

package kafka

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

const (
	kubernetesClusterDomain = "foo.bar"
)

func TestShouldRefreshOnlyPerBrokerConfigs(t *testing.T) {
	testCases := []struct {
		Description    string
		CurrentConfigs string
		DesiredConfigs string
		Result         bool
	}{
		{
			Description: "configs did not change",
			CurrentConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=unmodified_value_2
`,
			DesiredConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=unmodified_value_2
`,
			Result: true,
		},
		{
			Description: "only non per-broker config changed",
			CurrentConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=modified_value_2
`,
			DesiredConfigs: `unmodified_config_1=unmodified_value_1
unmodified_config_2=modified_value_3
`,
			Result: false,
		},
		{
			Description: "only per-broker config changed",
			CurrentConfigs: `unmodified_config_1=unmodified_value_1
ssl.client.auth=modified_value_2
`,
			DesiredConfigs: `unmodified_config_1=unmodified_value_1
ssl.client.auth=modified_value_3
`,
			Result: true,
		},
		{
			Description: "per-broker and non per-broker configs have changed",
			CurrentConfigs: `modified_config_1=modified_value_1
ssl.client.auth=modified_value_3
`,
			DesiredConfigs: `modified_config_1=modified_value_2
ssl.client.auth=modified_value_4
`,
			Result: false,
		},
		{
			Description:    "security protocol map can be changed as a per-broker config",
			CurrentConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol2",
			DesiredConfigs: "listener.security.protocol.map=listener1:protocol1,listener3:protocol3",
			Result:         true,
		},
		{
			Description:    "security protocol map can't be changed as a per-broker config",
			CurrentConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol2",
			DesiredConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol3",
			Result:         false,
		},
		{
			Description:    "security protocol map added as config",
			CurrentConfigs: "",
			DesiredConfigs: "listener.security.protocol.map=listener1:protocol1,listener2:protocol2",
			Result:         true,
		},
	}
	logger := util.CreateLogger(false, false)
	for i, testCase := range testCases {
		current, err := properties.NewFromString(testCase.CurrentConfigs)
		if err != nil {
			t.Fatalf("failed to parse Properties from string: %s", testCase.CurrentConfigs)
		}
		desired, err := properties.NewFromString(testCase.DesiredConfigs)
		if err != nil {
			t.Fatalf("failed to parse Properties from string: %s", testCase.DesiredConfigs)
		}
		if ShouldRefreshOnlyPerBrokerConfigs(current, desired, logger) != testCase.Result {
			t.Errorf("test case %d failed: %s", i, testCase.Description)
		}
	}
}

const defaultBrokerConfigGroup = "default"

var MinimalKafkaCluster = &v1beta1.KafkaCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "kafka-cluster",
		Namespace: "kafka-ns",
	},
	Spec: v1beta1.KafkaClusterSpec{
		ListenersConfig: v1beta1.ListenersConfig{
			ExternalListeners: []v1beta1.ExternalListenerConfig{
				{
					CommonListenerSpec: v1beta1.CommonListenerSpec{
						Name:          "test",
						ContainerPort: 9094,
					},
					ExternalStartingPort: 19090,
					AccessMethod:         corev1.ServiceTypeLoadBalancer,
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
	},
}

func TestGetClusterServiceDomainName(t *testing.T) {
	t.Run("Without cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Namespace = "kafka-ns"
		cluster.Spec.KubernetesClusterDomain = ""
		svcName := GetClusterServiceDomainName(cluster)
		expected := "kafka-ns.svc.cluster.local"

		if svcName != expected {
			t.Errorf("Mismatch in service domain name. Expected: %v, got %v", expected, svcName)
		}
	})

	t.Run("With cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Namespace = "kafka-ns2"
		cluster.Spec.KubernetesClusterDomain = kubernetesClusterDomain
		svcName := GetClusterServiceDomainName(cluster)
		expected := "kafka-ns2.svc.foo.bar"

		if svcName != expected {
			t.Errorf("Mismatch in service domain name. Expected: %v, got %v", expected, svcName)
		}
	})
}

func TestGetBrokerServiceFqdn(t *testing.T) {
	broker := &v1beta1.Broker{
		Id:                1,
		BrokerConfigGroup: defaultBrokerConfigGroup,
	}

	t.Run("Without cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Name = "kafka-cluster"
		cluster.Namespace = "kafka-ns"
		cluster.Spec.KubernetesClusterDomain = ""
		fqdn := GetBrokerServiceFqdn(cluster, broker)
		expectedFqdn := "kafka-cluster-1.kafka-ns.svc.cluster.local"

		if fqdn != expectedFqdn {
			t.Errorf("Mismatch in broker service fqdn. Expected: %v, got %v", expectedFqdn, fqdn)
		}
	})

	t.Run("With cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Name = "kafka-cluster1"
		cluster.Namespace = "kafka-ns2"
		cluster.Spec.KubernetesClusterDomain = kubernetesClusterDomain
		fqdn := GetBrokerServiceFqdn(cluster, broker)
		expectedFqdn := "kafka-cluster1-1.kafka-ns2.svc.foo.bar"

		if fqdn != expectedFqdn {
			t.Errorf("Mismatch in broker service fqdn. Expected: %v, got %v", expectedFqdn, fqdn)
		}
	})
}

func TestGetBootstrapServersService(t *testing.T) {
	t.Run("Without headless service and cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Spec.HeadlessServiceEnabled = false
		cluster.Spec.KubernetesClusterDomain = ""
		expected := "kafka-cluster-all-broker.kafka-ns.svc.cluster.local:29092"

		bootstrapServers, err := GetBootstrapServersService(cluster)

		if err != nil {
			t.Errorf("Should not return error. Got %v", err)
		}

		if bootstrapServers != expected {
			t.Errorf("Mismatch in cluster service bootstrap servers string. Expected: %v, got %v", expected, bootstrapServers)
		}
	})

	t.Run("With headless service and cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Spec.HeadlessServiceEnabled = true
		cluster.Spec.KubernetesClusterDomain = kubernetesClusterDomain
		expected := "kafka-cluster-headless.kafka-ns.svc.foo.bar:29092"

		bootstrapServers, err := GetBootstrapServersService(cluster)

		if err != nil {
			t.Errorf("Should not return error. Got %v", err)
		}

		if bootstrapServers != expected {
			t.Errorf("Mismatch in cluster service bootstrap servers string. Expected: %v, got %v", expected, bootstrapServers)
		}
	})

	t.Run("With headless service but without cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Spec.HeadlessServiceEnabled = true
		cluster.Spec.KubernetesClusterDomain = ""
		expected := "kafka-cluster-headless.kafka-ns.svc.cluster.local:29092"

		bootstrapServers, err := GetBootstrapServersService(cluster)

		if err != nil {
			t.Errorf("Should not return error. Got %v", err)
		}

		if bootstrapServers != expected {
			t.Errorf("Mismatch in cluster service bootstrap servers string. Expected: %v, got %v", expected, bootstrapServers)
		}
	})

	t.Run("With cluster domain override but without headless service", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Spec.HeadlessServiceEnabled = false
		cluster.Spec.KubernetesClusterDomain = kubernetesClusterDomain
		expected := "kafka-cluster-all-broker.kafka-ns.svc.foo.bar:29092"

		bootstrapServers, err := GetBootstrapServersService(cluster)

		if err != nil {
			t.Errorf("Should not return error. Got %v", err)
		}

		if bootstrapServers != expected {
			t.Errorf("Mismatch in cluster service bootstrap servers string. Expected: %v, got %v", expected, bootstrapServers)
		}
	})
}

func TestGetBootstrapServers(t *testing.T) {
	t.Run("Without cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Spec.KubernetesClusterDomain = ""
		expected := "kafka-cluster-0.kafka-ns.svc.cluster.local:29092,kafka-cluster-1.kafka-ns.svc.cluster.local:29092,kafka-cluster-2.kafka-ns.svc.cluster.local:29092"

		bootstrapServers, err := GetBootstrapServers(cluster)

		if err != nil {
			t.Errorf("Should not return error. Got %v", err)
		}

		if bootstrapServers != expected {
			t.Errorf("Mismatch in broker service bootstrap servers string. Expected: %v, got %v", expected, bootstrapServers)
		}
	})

	t.Run("With cluster domain override", func(t *testing.T) {
		cluster := MinimalKafkaCluster.DeepCopy()
		cluster.Spec.KubernetesClusterDomain = kubernetesClusterDomain
		expected := "kafka-cluster-0.kafka-ns.svc.foo.bar:29092,kafka-cluster-1.kafka-ns.svc.foo.bar:29092,kafka-cluster-2.kafka-ns.svc.foo.bar:29092"

		bootstrapServers, err := GetBootstrapServers(cluster)

		if err != nil {
			t.Errorf("Should not return error. Got %v", err)
		}

		if bootstrapServers != expected {
			t.Errorf("Mismatch in cluster service bootstrap servers string. Expected: %v, got %v", expected, bootstrapServers)
		}
	})
}
