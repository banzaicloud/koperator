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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

//TODO remove omitempty where not required

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	ListenersConfig     ListenersConfig     `json:"listenersConfig"`
	ZKAddresses         []string            `json:"zkAddresses"`
	RackAwareness       *RackAwareness      `json:"rackAwareness,omitempty"`
	BrokerConfigs       []BrokerConfig      `json:"brokerConfigs"`
	OneBrokerPerNode    bool                `json:"oneBrokerPerNode"`
	CruiseControlConfig CruiseControlConfig `json:"cruiseControlConfig"`
	ServiceAccount      string              `json:"serviceAccount"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster
type KafkaClusterStatus struct {
	BrokersState             map[int32]*BrokerState    `json:"brokersState,omitempty"`
	CruiseControlTopicStatus *CruiseControlTopicStatus `json:"cruiseControlTopicStatus,omitempty"`
}

// BrokerConfig defines the broker configuration
type BrokerConfig struct {
	Image            string                       `json:"image,omitempty"`
	Id               int32                        `json:"id"`
	NodeAffinity     *corev1.NodeAffinity         `json:"nodeAffinity,omitempty"`
	Config           string                       `json:"config,omitempty"`
	StorageConfigs   []StorageConfig              `json:"storageConfigs"`
	Resources        *corev1.ResourceRequirements `json:"resourceReqs,omitempty"`
	KafkaHeapOpts    string                       `json:"kafkaHeapOpts,omitempty"`
	KafkaJVMPerfOpts string                       `json:"kafkaJvmPerfOpts,omitempty"`
}

// RackAwareness defines the required fields to enable kafka's rack aware feature
type RackAwareness struct {
	Labels []string `json:"labels"`
}

// CruiseControlConfig defines the config for Cruise Control
type CruiseControlConfig struct {
	CruiseControlEndpoint string `json:"cruiseControlEndpoint,omitempty"`
	Config                string `json:"config,omitempty"`
	CapacityConfig        string `json:"capacityConfig,omitempty"`
	ClusterConfigs        string `json:"clusterConfigs,omitempty"`
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

// SSLSecrets defines the Kafka SSL secrets
type SSLSecrets struct {
	TLSSecretName   string `json:"tlsSecretName"`
	JKSPasswordName string `json:"jksPasswordName"`
}

// ExternalListenerConfig defines the external listener config for Kafka
type ExternalListenerConfig struct {
	Type                 string `json:"type"`
	Name                 string `json:"name"`
	ExternalStartingPort int32  `json:"externalStartingPort"`
	ContainerPort        int32  `json:"containerPort"`
}

// InternalListenerConfig defines the internal listener config for Kafka
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

// GetResources returns the broker specific Kubernetes resource
func (bConfig *BrokerConfig) GetResources() *corev1.ResourceRequirements {
	if bConfig.Resources != nil {
		return bConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("500m"),
			"memory": resource.MustParse("1Gi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("500Mi"),
		},
	}
}

// GetKafkaHeapOpts returns the broker specific Heap settings
func (bConfig *BrokerConfig) GetKafkaHeapOpts() string {
	if bConfig.KafkaHeapOpts != "" {
		return bConfig.KafkaHeapOpts
	}

	return "-Xmx2G -Xms2G"
}

// GetKafkaPerfJmvOpts returns the broker specific Perf JVM settings
func (bConfig *BrokerConfig) GetKafkaPerfJmvOpts() string {
	if bConfig.KafkaHeapOpts != "" {
		return bConfig.KafkaJVMPerfOpts
	}

	return "-server -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:+ExplicitGCInvokesConcurrent -Djava.awt.headless=true -Dsun.net.inetaddr.ttl=60"
}
