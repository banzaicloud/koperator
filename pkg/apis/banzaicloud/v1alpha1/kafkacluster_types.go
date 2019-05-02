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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

//TODO remove omitempty where not required

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	//MonitoringEnabled    bool            `json:"monitoringEnabled,omitempty"`
	ListenersConfig ListenersConfig `json:"listenersConfig"`
	ZKAddresses     []string        `json:"zkAddresses"`
	//RackAwarenessEnabled bool            `json:"rackAwarenessEnabled,omitempty"`
	BrokerConfigs  []BrokerConfig `json:"brokerConfigs"`
	ServiceAccount string         `json:"serviceAccount"`
	//RestProxyEnabled bool `json:"restProxyEnabled"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster
type KafkaClusterStatus struct {
	BrokersState map[int32]BrokerState `json:"brokersState,omitempty"`
}

// BrokerConfig defines the broker configuration
type BrokerConfig struct {
	Image          string          `json:"image,omitempty"`
	Id             int32           `json:"id"`
	Config         string          `json:"config,omitempty"`
	StorageConfigs []StorageConfig `json:"storageConfigs"`
}

// StorageConfig defines the broker storage configuration
type StorageConfig struct {
	MountPath string                            `json:"mountPath"`
	PVCSpec   *corev1.PersistentVolumeClaimSpec `json:"pvcSpec"`
}

//ListenersConfig defines the Kafka listener types
type ListenersConfig struct {
	ExternalListeners []ExternalListenerConfig `json:"externalListeners,omitempty"`
	InternalListeners []InternalListenerConfig `json:"internalListeners"`
	SSLSecrets        *SSLSecrets              `json:"sslSecrets,omitempty"`
	//SASLSecret        string                   `json:"saslSecret"`
}

type SSLSecrets struct {
	TLSSecretName   string `json:"tlsSecretName"`
	JKSPasswordName string `json:"jksPasswordName"`
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

//GetServiceAccount returns the Kubernetes Service Account to use for Kafka Cluster
func (spec *KafkaClusterSpec) GetServiceAccount() string {
	if spec.ServiceAccount != "" {
		return spec.ServiceAccount
	}
	return "default"
}
