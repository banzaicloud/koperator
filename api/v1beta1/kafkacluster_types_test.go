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

package v1beta1

import (
	"reflect"
	"strconv"
	"testing"

	"gotest.tools/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// we expect the final result will be the two segment's union
func TestGetBrokerConfigAffinityMergeBrokerNodeAffinityWithGroupsAntiAffinity(t *testing.T) {
	expected := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchFields: []corev1.NodeSelectorRequirement{
							{Key: "fruit", Operator: "in", Values: []string{"apple"}},
						},
					},
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: nil,
		},
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_broker"}},
					Namespaces:    nil,
					TopologyKey:   "kubernetes.io/hostname",
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: nil,
		},
	}

	spec := KafkaClusterSpec{
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: nil,
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "fruit",
											Operator: "in",
											Values:   []string{"apple"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &BrokerConfig{
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_broker"},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}

	config, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, expected, config.Affinity)
}

// we expect the final result will be the two segment's union
func TestGetBrokerConfigAffinityMergeEmptyBrokerConfigWithDefaultConfig(t *testing.T) {
	expected := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchFields: []corev1.NodeSelectorRequirement{
							{Key: "fruit", Operator: "in", Values: []string{"apple"}},
						},
					},
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: nil,
		},
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_config_group"}},
					Namespaces:    nil,
					TopologyKey:   "kubernetes.io/hostname",
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: nil,
		},
	}

	spec := KafkaClusterSpec{
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				Affinity: &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_config_group"},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: nil,
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "fruit",
											Operator: "in",
											Values:   []string{"apple"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
	}

	config, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error(err)
	}

	assert.DeepEqual(t, expected, config.Affinity)
}

// the equal values got duplicated
func TestGetBrokerConfigAffinityMergeEqualPodAntiAffinity(t *testing.T) {
	expected := &corev1.Affinity{
		NodeAffinity: nil,
		PodAffinity:  nil,
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_broker"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: nil,
		},
	}

	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &BrokerConfig{
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_broker"},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}

	spec := KafkaClusterSpec{
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				Affinity: &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "kafka", "kafka_cr": "kafka_config_group"},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
		},
	}

	config, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, expected, config.Affinity)
}

func TestGetBrokerConfig(t *testing.T) {
	expected := &BrokerConfig{
		StorageConfigs: []StorageConfig{
			{
				MountPath: "kafka-test/log",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
					},
				},
			},
			{
				MountPath: "kafka-test1/log",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
					},
				},
			},
		},
	}
	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &BrokerConfig{
			StorageConfigs: []StorageConfig{
				{
					MountPath: "kafka-test/log",
					PvcSpec: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
						},
					},
				},
			},
		},
	}
	spec := KafkaClusterSpec{
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				StorageConfigs: []StorageConfig{
					{
						MountPath: "kafka-test1/log",
						PvcSpec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
							},
						},
					},
				},
			},
		},
	}

	result, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error("Error GetBrokerConfig throw an unexpected error")
	}
	if !reflect.DeepEqual(result, expected) {
		t.Error("Expected:", expected, "Got:", result)
	}
}

func TestGetBrokerConfigUniqueStorage(t *testing.T) {
	expected := &BrokerConfig{
		StorageConfigs: []StorageConfig{
			{
				MountPath: "kafka-test/log",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")},
					},
				},
			},
			{
				MountPath: "kafka-test1/log",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
					},
				},
			},
		},
	}
	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &BrokerConfig{
			StorageConfigs: []StorageConfig{
				{
					MountPath: "kafka-test/log",
					PvcSpec: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")},
						},
					},
				},
			},
		},
	}

	spec := KafkaClusterSpec{
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				StorageConfigs: []StorageConfig{
					{
						MountPath: "kafka-test1/log",
						PvcSpec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
							},
						},
					},
					{
						MountPath: "kafka-test/log",
						PvcSpec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
							},
						},
					},
				},
			},
		},
	}

	result, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error("Error GetBrokerConfig throw an unexpected error")
	}
	if !reflect.DeepEqual(result, expected) {
		t.Error("Expected:", expected, "Got:", result)
	}
}

func TestGetBrokerConfigEnvs(t *testing.T) {
	expected := &BrokerConfig{
		Envs: []corev1.EnvVar{
			{Name: "VAR", Value: "cluster"},
			{Name: "VAR", Value: "group"},
			{Name: "VAR", Value: "broker"},
		},
	}

	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &BrokerConfig{
			Envs: []corev1.EnvVar{
				{Name: "VAR", Value: "broker"},
			},
		},
	}

	spec := KafkaClusterSpec{
		Envs: []corev1.EnvVar{
			{Name: "VAR", Value: "cluster"},
		},
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				Envs: []corev1.EnvVar{
					{Name: "VAR", Value: "group"},
				},
			},
		},
	}

	result, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error("Error GetBrokerConfig throw an unexpected error")
	}
	if !reflect.DeepEqual(result, expected) {
		t.Error("Expected:", expected, "Got:", result)
	}
}

func TestGetBrokerLabels(t *testing.T) {
	const (
		expectedDefaultLabelApp = "kafka"
		expectedKafkaCRName     = "kafka"

		expectedBrokerId        = 0
	)

	expected := map[string]string{
		"app":            expectedDefaultLabelApp,
		"brokerId":       strconv.Itoa(0),
		"kafka_cr":       expectedKafkaCRName,
		"test_label_key": "test_label_value",
	}

	brokerConfig := &BrokerConfig{
		BrokerLabels: map[string]string{
			"app":            "test-app",
			"brokerId":       "test-id",
			"kafka_cr":       "test-cr-name",
			"test_label_key": "test_label_value",
		},
	}

	result := brokerConfig.GetBrokerLabels(expectedKafkaCRName, expectedBrokerId)

	if !reflect.DeepEqual(result, expected) {
		t.Error("Expected:", expected, "Got:", result)
	}
}
