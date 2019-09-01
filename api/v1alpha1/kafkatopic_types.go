package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KafkaTopicSpec defines the desired state of KafkaTopic
// +k8s:openapi-gen=true
type KafkaTopicSpec struct {
	Name              string            `json:"name"`
	Partitions        int32             `json:"partitions"`
	ReplicationFactor int32             `json:"replicationFactor"`
	Config            map[string]string `json:"config,omitempty"`
	ClusterRef        ClusterReference  `json:"clusterRef"`
}

// KafkaTopicStatus defines the observed state of KafkaTopic
// +k8s:openapi-gen=true
type KafkaTopicStatus struct {
	PartitionCount  int32             `json:"partitionCount"`
	Leaders         map[string]string `json:"leaders"`
	ReplicaCounts   map[string]string `json:"replicaCounts"`
	InSyncReplicas  map[string]string `json:"inSyncReplicas"`
	OfflineReplicas map[string]string `json:"offlineReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// KafkaTopic is the Schema for the kafkatopics API
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
