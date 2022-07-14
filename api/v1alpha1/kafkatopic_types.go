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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MinPartitions        = -1
	MinReplicationFactor = -1
)

// KafkaTopicSpec defines the desired state of KafkaTopic
// +k8s:openapi-gen=true
type KafkaTopicSpec struct {
	Name string `json:"name"`
	// Partitions defines the desired number of partitions; must be positive, or -1 to signify using the broker's default
	// +kubebuilder:validation:Minimum=-1
	Partitions int32 `json:"partitions"`
	// ReplicationFactor defines the desired replication factor; must be positive, or -1 to signify using the broker's default
	// +kubebuilder:validation:Minimum=-1
	ReplicationFactor int32             `json:"replicationFactor"`
	Config            map[string]string `json:"config,omitempty"`
	ClusterRef        ClusterReference  `json:"clusterRef"`
}

// KafkaTopicStatus defines the observed state of KafkaTopic
// +k8s:openapi-gen=true
type KafkaTopicStatus struct {
	State TopicState `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
// KafkaTopic is the Schema for the kafkatopics API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type KafkaTopic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaTopicSpec   `json:"spec,omitempty"`
	Status KafkaTopicStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaTopicList contains a list of KafkaTopic
type KafkaTopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaTopic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaTopic{}, &KafkaTopicList{})
}
