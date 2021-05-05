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

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

func TestGetAffinity(t *testing.T) {
	defaultPodAntiAffinity := &corev1.Affinity{
		NodeAffinity: nil,
		PodAffinity:  nil,
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels:      map[string]string{"app": "kafka", "kafka_cr": "name"},
						MatchExpressions: nil,
					},
					Namespaces:  nil,
					TopologyKey: "kubernetes.io/hostname",
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: nil,
		},
	}

	cluster := v1beta1.KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{
			ClusterName: "name",
		},
		Spec: v1beta1.KafkaClusterSpec{
			BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
				"test": {},
			},
		},
		Status: v1beta1.KafkaClusterStatus{},
	}

	// no affinity defined
	nilAffinityBrokerConfig := v1beta1.BrokerConfig{}

	cluster.Spec.OneBrokerPerNode = true
	// expecting old behavior
	affinity := getAffinity(&nilAffinityBrokerConfig, &cluster)
	assert.DeepEqual(t, affinity.PodAntiAffinity, defaultPodAntiAffinity.PodAntiAffinity)

	broker := v1beta1.Broker{
		BrokerConfig: &nilAffinityBrokerConfig,
	}

	mergedAffinityBrokerConfig, _ := broker.GetBrokerConfig(cluster.Spec.BrokerConfigGroups)

	// expecting old behavior
	affinity = getAffinity(mergedAffinityBrokerConfig, &cluster)
	assert.DeepEqual(t, affinity.PodAntiAffinity, defaultPodAntiAffinity.PodAntiAffinity)

	broker = v1beta1.Broker{
		BrokerConfigGroup: "test",
		BrokerConfig:      &nilAffinityBrokerConfig,
	}

	mergedAffinityBrokerConfig2, _ := broker.GetBrokerConfig(cluster.Spec.BrokerConfigGroups)

	// expecting old behavior
	affinity = getAffinity(mergedAffinityBrokerConfig2, &cluster)
	assert.DeepEqual(t, affinity.PodAntiAffinity, defaultPodAntiAffinity.PodAntiAffinity)

	nonNilAffinityBrokerConfig := v1beta1.BrokerConfig{Affinity: &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{}}}

	cluster.Spec.OneBrokerPerNode = false
	// still expecting old behavior but with only a preferred anti-affinity
	affinity = getAffinity(&nilAffinityBrokerConfig, &cluster)
	if affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight != int32(100) ||
		len(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0 {
		t.Error("Given affinity does not match expectations")
	}

	// should just return what was given as an input
	affinity = getAffinity(&nonNilAffinityBrokerConfig, &cluster)
	assert.DeepEqual(t, affinity, nonNilAffinityBrokerConfig.Affinity.DeepCopy())
}
