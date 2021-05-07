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
			/*{Name: "a", Value: "global"},
			{Name: "b", Value: "group"},
			{Name: "c", Value: "broker"},
			{Name: "d", Value: "global"},
			{Name: "e", Value: "group"},
			{Name: "f", Value: "broker"},
			{Name: "g", Value: "group"},
			{Name: "h", Value: "broker"},
			{Name: "i", Value: "groupbroker"},
			{Name: "j", Value: "broker"},
			{Name: "k", Value: "globalgroupbroker"},
			{Name: "l", Value: "groupbroker"},
			{Name: "m", Value: "globalgroupbroker"},*/
		},
	}

	broker := Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &BrokerConfig{
			Envs: []corev1.EnvVar{
				{Name: "VAR", Value: "broker"},
				/*{Name: "c", Value: "broker"},
				{Name: "f", Value: "+broker"},
				{Name: "h", Value: "broker"},
				{Name: "i", Value: "+broker"},
				{Name: "j", Value: "broker"},
				{Name: "k", Value: "+broker"},
				{Name: "l", Value: "+broker"},
				{Name: "m", Value: "+broker"},*/
			},
		},
	}

	spec := KafkaClusterSpec{
		Envs: []corev1.EnvVar{
			{Name: "VAR", Value: "cluster"},
			/*{Name: "a", Value: "global"},
			{Name: "d", Value: "+global"},
			{Name: "g", Value: "global"},
			{Name: "i", Value: "+global"},
			{Name: "j", Value: "global"},
			{Name: "k", Value: "global"},
			{Name: "m", Value: "+global"},*/
		},
		BrokerConfigGroups: map[string]BrokerConfig{
			"default": {
				Envs: []corev1.EnvVar{
					{Name: "VAR", Value: "group"},
					/*{Name: "b", Value: "group"},
					{Name: "e", Value: "+group"},
					{Name: "g", Value: "group"},
					{Name: "h", Value: "group"},
					{Name: "i", Value: "group"},
					{Name: "j", Value: "+group"},
					{Name: "k", Value: "+group"},
					{Name: "l", Value: "+group"},
					{Name: "m", Value: "+group"},*/
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
