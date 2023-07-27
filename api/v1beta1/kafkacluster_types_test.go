// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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

	"github.com/stretchr/testify/require"
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
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_broker"}},
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
								MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_broker"},
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
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_config_group"}},
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
									MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_config_group"},
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
						MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_broker"},
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
								MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_broker"},
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
									MatchLabels: map[string]string{AppLabelKey: "kafka", KafkaCRLabelKey: "kafka_config_group"},
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

// TestGetBrokerLabels makes sure the reserved labels "app", "brokerId", and "kafka_cr" are not overridden by the BrokerConfig
func TestGetBrokerLabels(t *testing.T) {
	const (
		expectedDefaultLabelApp = "kafka"
		expectedKafkaCRName     = "kafka"

		expectedBrokerId = 0
	)

	expected := map[string]string{
		AppLabelKey:      expectedDefaultLabelApp,
		BrokerIdLabelKey: strconv.Itoa(expectedBrokerId),
		KafkaCRLabelKey:  expectedKafkaCRName,
		"test_label_key": "test_label_value",
	}

	brokerConfig := &BrokerConfig{
		BrokerLabels: map[string]string{
			AppLabelKey:      "test_app",
			BrokerIdLabelKey: "test_id",
			KafkaCRLabelKey:  "test_cr_name",
			"test_label_key": "test_label_value",
		},
	}

	result := brokerConfig.GetBrokerLabels(expectedKafkaCRName, expectedBrokerId)

	if !reflect.DeepEqual(result, expected) {
		t.Error("Expected:", expected, "Got:", result)
	}
}

func TestGetStorageMountPaths(t *testing.T) {
	testCases := []struct {
		testName           string
		brokerConfig       *BrokerConfig
		expectedMountPaths string
	}{
		{
			testName:           "BrokerConfig has no StorageConfigs",
			brokerConfig:       &BrokerConfig{},
			expectedMountPaths: "",
		},
		{
			testName: "BrokerConfig has one storage configuration under StorageConfigs",
			brokerConfig: &BrokerConfig{
				StorageConfigs: []StorageConfig{
					{
						MountPath: "test-log-1",
					},
				},
			},
			expectedMountPaths: "test-log-1",
		},
		{
			testName: "BrokerConfig has multiple storage configuration under StorageConfigs",
			brokerConfig: &BrokerConfig{
				StorageConfigs: []StorageConfig{
					{
						MountPath: "test-log-1",
					},
					{
						MountPath: "test-log-2",
					},
					{
						MountPath: "test-log-3",
					},
					{
						MountPath: "test-log-4",
					},
					{
						MountPath: "test-log-5",
					},
				},
			},
			expectedMountPaths: "test-log-1,test-log-2,test-log-3,test-log-4,test-log-5",
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			gotMountPaths := test.brokerConfig.GetStorageMountPaths()
			require.Equal(t, gotMountPaths, test.expectedMountPaths)
		})
	}
}

func TestIsBrokerOnlyNode(t *testing.T) {
	testCases := []struct {
		testName     string
		broker       Broker
		isBrokerOnly bool
	}{
		{
			testName: "the broker is a broker-only node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"broker"},
				},
			},
			isBrokerOnly: true,
		},
		{
			testName: "the broker is a controller-only node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"controller"},
				},
			},
			isBrokerOnly: false,
		},
		{
			testName: "the broker is a combined node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"controller", "broker"},
				},
			},
			isBrokerOnly: false,
		},
		{
			testName: "the broker has no process roles defined",
			broker: Broker{
				Id:           0,
				BrokerConfig: &BrokerConfig{},
			},
			isBrokerOnly: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			require.Equal(t, test.broker.BrokerConfig.IsBrokerOnlyNode(), test.isBrokerOnly)
		})
	}
}

func TestIsControllerOnlyNode(t *testing.T) {
	testCases := []struct {
		testName         string
		broker           Broker
		isControllerOnly bool
	}{
		{
			testName: "the broker is a controller-only node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"controller"},
				},
			},
			isControllerOnly: true,
		},
		{
			testName: "the broker is a broker-only node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"broker"},
				},
			},
			isControllerOnly: false,
		},
		{
			testName: "the broker is a combined node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"broker", "controller"},
				},
			},
			isControllerOnly: false,
		},
		{
			testName: "the broker has no process roles defined",
			broker: Broker{
				Id: 0,
				// every broker is expected to have the broker configurations set through either "brokerConfig" or "brokerConfigGroup"
				// and this part is tested in other places, therefore setting "brokerConfig" to be empty for this unit test
				BrokerConfig: &BrokerConfig{},
			},
			isControllerOnly: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			require.Equal(t, test.broker.BrokerConfig.IsControllerOnlyNode(), test.isControllerOnly)
		})
	}
}

func TestIsCombinedNode(t *testing.T) {
	testCases := []struct {
		testName   string
		broker     Broker
		isCombined bool
	}{
		{
			testName: "the broker is a broker-only node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"broker"},
				},
			},
			isCombined: false,
		},
		{
			testName: "the broker is a controller-only node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"controller"},
				},
			},
			isCombined: false,
		},
		{
			testName: "the broker is a combined node",
			broker: Broker{
				Id: 0,
				BrokerConfig: &BrokerConfig{
					Roles: []string{"broker", "controller"},
				},
			},
			isCombined: true,
		},
		{
			testName: "the broker has no process roles defined",
			broker: Broker{
				Id:           0,
				BrokerConfig: &BrokerConfig{},
			},
			isCombined: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			require.Equal(t, test.broker.BrokerConfig.IsCombinedNode(), test.isCombined)
		})
	}
}
