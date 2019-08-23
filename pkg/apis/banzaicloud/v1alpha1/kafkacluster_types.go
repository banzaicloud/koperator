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
	HeadlessServiceEnabled bool                          `json:"headlessServiceEnabled"`
	ListenersConfig        ListenersConfig               `json:"listenersConfig"`
	ZKAddresses            []string                      `json:"zkAddresses"`
	RackAwareness          *RackAwareness                `json:"rackAwareness,omitempty"`
	BrokerConfigs          []BrokerConfig                `json:"brokerConfigs"`
	BrokerConfigGroups     map[string]*BrokerConfig      `json:"brokerConfigGroups,omitempty"`
	OneBrokerPerNode       bool                          `json:"oneBrokerPerNode"`
	CruiseControlConfig    CruiseControlConfig           `json:"cruiseControlConfig"`
	EnvoyConfig            EnvoyConfig                   `json:"envoyConfig,omitempty"`
	ServiceAccount         string                        `json:"serviceAccount"`
	ImagePullSecrets       []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	MonitoringConfig       MonitoringConfig              `json:"monitoringConfig,omitempty"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster
type KafkaClusterStatus struct {
	BrokersState             map[int32]*BrokerState   `json:"brokersState,omitempty"`
	CruiseControlTopicStatus CruiseControlTopicStatus `json:"cruiseControlTopicStatus,omitempty"`
}

// BrokerConfig defines the broker configuration
type BrokerConfig struct {
	Image            string                       `json:"image,omitempty"`
	Id               int32                        `json:"id"`
	NodeAffinity     *corev1.NodeAffinity         `json:"nodeAffinity,omitempty"`
	NodeSelector     map[string]string            `json:"nodeSelector,omitempty"`
	Tolerations      []corev1.Toleration          `json:"tolerations,omitempty"`
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
	Image                 string `json:"image,omitempty"`
}

// EnvoyConfig defines the config for Envoy
type EnvoyConfig struct {
	Image string `json:"image"`
}

// MonitoringConfig defines the config for monitoring Kafka and Cruise Control
type MonitoringConfig struct {
	JmxImage               string `json:"jmxImage"`
	PathToJar              string `json:"pathToJar"`
	KafkaJMXExporterConfig string `json:"kafkaJMXExporterConfig,omitempty"`
	CCJMXExporterConfig    string `json:"cCJMXExporterConfig,omitempty"`
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

//GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (spec *KafkaClusterSpec) GetImagePullSecrets() []corev1.LocalObjectReference {
	return spec.ImagePullSecrets
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
	if bConfig.KafkaJVMPerfOpts != "" {
		return bConfig.KafkaJVMPerfOpts
	}

	return "-server -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:+ExplicitGCInvokesConcurrent -Djava.awt.headless=true -Dsun.net.inetaddr.ttl=60"
}

// GetEnvoyImage returns the used envoy image
func (eConfig *EnvoyConfig) GetEnvoyImage() string {
	if eConfig.Image != "" {
		return eConfig.Image
	}

	return "banzaicloud/envoy:0.1.0"
}

// GetCCImage returns the used Cruise Control image
func (cConfig *CruiseControlConfig) GetCCImage() string {
	if cConfig.Image != "" {
		return cConfig.Image
	}
	return "solsson/kafka-cruise-control@sha256:c70eae329b4ececba58e8cf4fa6e774dd2e0205988d8e5be1a70e622fcc46716"
}

// GetImage returns the used image for Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetImage() string {
	if mConfig.JmxImage != "" {
		return mConfig.JmxImage
	}
	return "banzaicloud/jmx-javaagent:0.12.0"
}

// GetPathToJar returns the path in the used Image for Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetPathToJar() string {
	if mConfig.PathToJar != "" {
		return mConfig.PathToJar
	}
	return "/opt/jmx_exporter/jmx_prometheus_javaagent-0.12.0.jar"
}

// GetKafkaJMXExporterConfig returns the config for Kafka Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetKafkaJMXExporterConfig() string {
	if mConfig.KafkaJMXExporterConfig != "" {
		return mConfig.KafkaJMXExporterConfig
	}
	return `
    lowercaseOutputName: true
    rules:
    - pattern : kafka.cluster<type=(.+), name=(.+), topic=(.+), partition=(.+)><>Value
      name: kafka_cluster_$1_$2
      labels:
        topic: "$3"
        partition: "$4"
    - pattern : kafka.log<type=Log, name=(.+), topic=(.+), partition=(.+)><>Value
      name: kafka_log_$1
      labels:
        topic: "$2"
        partition: "$3"
    - pattern : kafka.controller<type=(.+), name=(.+)><>(Count|Value)
      name: kafka_controller_$1_$2
    - pattern : kafka.network<type=(.+), name=(.+)><>Value
      name: kafka_network_$1_$2
    - pattern : kafka.network<type=(.+), name=(.+)PerSec, request=(.+)><>Count
      name: kafka_network_$1_$2_total
      labels:
        request: "$3"
    - pattern : kafka.network<type=(.+), name=(\w+), networkProcessor=(.+)><>Count
      name: kafka_network_$1_$2
      labels:
        request: "$3"
      type: COUNTER
    - pattern : kafka.network<type=(.+), name=(\w+), request=(\w+)><>Count
      name: kafka_network_$1_$2
      labels:
        request: "$3"
      type: COUNTER
    - pattern : kafka.network<type=(.+), name=(\w+)><>Count
      name: kafka_network_$1_$2
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+)PerSec\w*, topic=(.+)><>Count
      name: kafka_server_$1_$2_total
      labels:
        topic: "$3"
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+)PerSec\w*><>Count
      name: kafka_server_$1_$2_total
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+), clientId=(.+), topic=(.+), partition=(.*)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        clientId: "$3"
        topic: "$4"
        partition: "$5"
    - pattern : kafka.server<type=(.+), name=(.+), topic=(.+), partition=(.*)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        topic: "$3"
        partition: "$4"
    - pattern : kafka.server<type=(.+), name=(.+), topic=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        topic: "$3"
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+), clientId=(.+), brokerHost=(.+), brokerPort=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        clientId: "$3"
        broker: "$4:$5"
    - pattern : kafka.server<type=(.+), name=(.+), clientId=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        clientId: "$3"
    - pattern : kafka.server<type=(.+), name=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
    - pattern : kafka.(\w+)<type=(.+), name=(.+)PerSec\w*><>Count
      name: kafka_$1_$2_$3_total
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, topic=(.+)><>Count
      name: kafka_$1_$2_$3_total
      labels:
        topic: "$4"
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, topic=(.+), partition=(.+)><>Count
      name: kafka_$1_$2_$3_total
      labels:
        topic: "$4"
        partition: "$5"
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+)><>(Count|Value)
      name: kafka_$1_$2_$3_$4
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+), (\w+)=(.+)><>(Count|Value)
      name: kafka_$1_$2_$3_$6
      labels:
        "$4": "$5"
`
}

// GetCCJMXExporterConfig returns the config for CC Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetCCJMXExporterConfig() string {
	if mConfig.CCJMXExporterConfig != "" {
		return mConfig.CCJMXExporterConfig
	}
	return `
    lowercaseOutputName: true
`
}
