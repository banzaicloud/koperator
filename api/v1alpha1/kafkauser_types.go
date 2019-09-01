package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaUserSpec defines the desired state of KafkaUser
// +k8s:openapi-gen=true
type KafkaUserSpec struct {
	Name        string           `json:"name"`
	SecretName  string           `json:"secretName"`
	TopicGrants []UserTopicGrant `json:"topicGrants"`
	IncludeJKS  bool             `json:"includeJKS,omitempty"`
	ClusterRef  ClusterReference `json:"clusterRef"`
}

// UserTopicGrant is the desired permissions for the KafkaUser
type UserTopicGrant struct {
	TopicName  string          `json:"topicName"`
	AccessType KafkaAccessType `json:"accessType"`
}

// KafkaUserStatus defines the observed state of KafkaUser
// +k8s:openapi-gen=true
type KafkaUserStatus struct {
	Name        string           `json:"name"`
	SecretName  string           `json:"secretName"`
	TopicGrants []UserTopicGrant `json:"topicGrants"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// KafkaUser is the Schema for the kafkausers API
type KafkaUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaUserSpec   `json:"spec,omitempty"`
	Status KafkaUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaUserList contains a list of KafkaUser
type KafkaUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaUser{}, &KafkaUserList{})
}
