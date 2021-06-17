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
	"reflect"
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
			Name: "name",
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

	mergedAffinityBrokerConfig, _ := broker.GetBrokerConfig(cluster.Spec)

	// expecting old behavior
	affinity = getAffinity(mergedAffinityBrokerConfig, &cluster)
	assert.DeepEqual(t, affinity.PodAntiAffinity, defaultPodAntiAffinity.PodAntiAffinity)

	broker = v1beta1.Broker{
		BrokerConfigGroup: "test",
		BrokerConfig:      &nilAffinityBrokerConfig,
	}

	mergedAffinityBrokerConfig2, _ := broker.GetBrokerConfig(cluster.Spec)

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

func Test_generateEnvConfig(t *testing.T) {
	expected := []corev1.EnvVar{
		{Name: "KAFKA_HEAP_OPTS", Value: "-Xmx2G -Xms2G"},
		{Name: "KAFKA_JVM_PERFORMANCE_OPTS", Value: "-server -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:+ExplicitGCInvokesConcurrent -Djava.awt.headless=true -Dsun.net.inetaddr.ttl=60"},
		{Name: "a", Value: "global"},
		{Name: "b", Value: "group"},
		{Name: "c", Value: "broker"},
		{Name: "d", Value: "global"},
		{Name: "e", Value: "codegroup"},
		{Name: "f", Value: "broker"},
		{Name: "g", Value: "group"},
		{Name: "h", Value: "broker"},
		{Name: "i", Value: "groupbroker"},
		{Name: "j", Value: "broker"},
		{Name: "k", Value: "brokerglobal group"},
		{Name: "l", Value: "broker group"},
		{Name: "m", Value: "globalgroupbroker"},
		{Name: "n", Value: ""},
		{Name: "o", Value: "brokergroupglobal"},
	}

	broker := v1beta1.Broker{
		Id:                0,
		BrokerConfigGroup: "default",
		BrokerConfig: &v1beta1.BrokerConfig{
			Envs: []corev1.EnvVar{
				{Name: "c", Value: "broker"},
				{Name: "f+ ", Value: "broker"},
				{Name: " h", Value: "broker"},
				{Name: "i+", Value: "broker"},
				{Name: "j", Value: "broker"},
				{Name: " +k", Value: "broker"},
				{Name: "+l", Value: "broker"},
				{Name: "m+ ", Value: "broker"},
				{Name: " +o", Value: "broker"},
			},
		},
	}

	spec := v1beta1.KafkaClusterSpec{
		Envs: []corev1.EnvVar{
			{Name: "a", Value: "global"},
			{Name: "d+", Value: "global"},
			{Name: "g", Value: "global"},
			{Name: "i+", Value: "global"},
			{Name: "j", Value: "global"},
			{Name: "k", Value: "global"},
			{Name: "m+", Value: "global"},
			{Name: "n", Value: ""},
			{Name: " +o ", Value: "global"},
		},
		BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
			"default": {
				Envs: []corev1.EnvVar{
					{Name: "b", Value: "group"},
					{Name: "e+", Value: "group"},
					{Name: "g", Value: "group"},
					{Name: "h", Value: "group"},
					{Name: "i", Value: "group"},
					{Name: "j+ ", Value: "group"},
					{Name: "k+", Value: " group"},
					{Name: "l+ ", Value: " group"},
					{Name: "m+", Value: "group"},
					{Name: "+o ", Value: "group"},
				},
			},
		},
	}

	brokerConfig, err := broker.GetBrokerConfig(spec)
	if err != nil {
		t.Error("GetBrokerConfig failed")
	}

	result := generateEnvConfig(brokerConfig, []corev1.EnvVar{
		{Name: "b", Value: "code"},
		{Name: "e", Value: "code"},
	})
	if err != nil {
		t.Error("Error GetBrokerConfig throw an unexpected error")
	}
	if !reflect.DeepEqual(result, expected) {
		t.Error("Expected:", expected, "Got:", result)
	}
}
