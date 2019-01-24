/*
Copyright 2019 Banzai Cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

//TODO remove omitempty where not required

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	Brokers          int32             `json:"brokers,omitempty"`
	Image            string            `json:"image,omitempty"`
	Annotations      map[string]string `json:"annotations"`
	Listeners        Listeners         `json:"listeners"`
	BrokerConfig     string            `json:"brokerConfig"`
	MonitoringConfig MonitoringConfig  `json:"monitoring,omitempty"`
	ServiceAccount   string            `json:"serviceAccount"`
	StorageSize      string            `json:"storageSize"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster
type KafkaClusterStatus struct {
	HealthyBrokers int `json:"healthybrokers,omitempty"`
}

// MonitoringConfig defines the monitoring configuration
type MonitoringConfig struct {
}

//Listeners defines the Kafka listener types
type Listeners struct {
	ExternalListener []ExternalListenerConfig `json:"externalListener"`
	InternalListener []InternalListenerConfig `json:"internalListener"`
}

type ExternalListenerConfig struct {
	Type                 string `json:"type"`
	Name                 string `json:"name"`
	ExternalStartingPort int32  `json:"externalStartingPort"`
	ContainerPort        int32  `json:"containerPort"`
}

type InternalListenerConfig struct {
	Type                            string `json:"type"`
	Name                            string `json:"name"`
	UsedForInnerBrokerCommunication bool   `json:"usedForInnerBrokerCommunication"`
	ContainerPort                   int32  `json:"containerPort"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaCluster is the Schema for the kafkaclusters API
// +k8s:openapi-gen=true
type KafkaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaClusterSpec   `json:"spec,omitempty"`
	Status KafkaClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaClusterList contains a list of KafkaCluster
type KafkaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaCluster{}, &KafkaClusterList{})
}

// GetServiceAccount returns the Kubernetes Service Account to use for Kafka Cluster
func (spec *KafkaClusterSpec) GetServiceAccount() string {
	if spec.ServiceAccount != "" {
		return spec.ServiceAccount
	}
	return "default"
}

func (spec *KafkaClusterSpec) GenerateDefaultConfig() string {
	return spec.BrokerConfig
}
