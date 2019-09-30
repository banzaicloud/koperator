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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	HeadlessServiceEnabled bool                    `json:"headlessServiceEnabled"`
	ListenersConfig        ListenersConfig         `json:"listenersConfig"`
	ZKAddresses            []string                `json:"zkAddresses"`
	RackAwareness          *RackAwareness          `json:"rackAwareness,omitempty"`
	ClusterImage           string                  `json:"clusterImage,omitempty"`
	ReadOnlyConfig         string                  `json:"readOnlyConfig,omitempty"`
	ClusterWideConfig      string                  `json:"clusterWideConfig,omitempty"`
	BrokerConfigGroups     map[string]BrokerConfig `json:"brokerConfigGroups,omitempty"`
	Brokers                []Broker                `json:"brokers"`
	RollingUpgradeConfig   RollingUpgradeConfig    `json:"rollingUpgradeConfig"`
	OneBrokerPerNode       bool                    `json:"oneBrokerPerNode"`
	CruiseControlConfig    CruiseControlConfig     `json:"cruiseControlConfig"`
	EnvoyConfig            EnvoyConfig             `json:"envoyConfig,omitempty"`
	MonitoringConfig       MonitoringConfig        `json:"monitoringConfig,omitempty"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster
type KafkaClusterStatus struct {
	BrokersState             map[string]BrokerState   `json:"brokersState,omitempty"`
	CruiseControlTopicStatus CruiseControlTopicStatus `json:"cruiseControlTopicStatus,omitempty"`
	State                    ClusterState             `json:"state"`
	RollingUpgrade           RollingUpgradeStatus     `json:"rollingUpgradeStatus,omitempty"`
	AlertCount               int                      `json:"alertCount"`
}

// RollingUpgradeStatus defines status of rolling upgrade
type RollingUpgradeStatus struct {
	LastSuccess string `json:"lastSuccess"`
	ErrorCount  int    `json:"errorCount"`
}

// RollingUpgradeConfig defines the desired config of the RollingUpgrade
type RollingUpgradeConfig struct {
	FailureThreshold int `json:"failureThreshold"`
}

// Broker defines the broker basic configuration
type Broker struct {
	Id                int32         `json:"id"`
	BrokerConfigGroup string        `json:"brokerConfigGroup,omitempty"`
	ReadOnlyConfig    string        `json:"readOnlyConfig,omitempty"`
	BrokerConfig      *BrokerConfig `json:"brokerConfig,omitempty"`
}

// BrokerConfig defines the broker configuration
type BrokerConfig struct {
	Image              string                        `json:"image,omitempty"`
	NodeAffinity       *corev1.NodeAffinity          `json:"nodeAffinity,omitempty"`
	Config             string                        `json:"config,omitempty"`
	StorageConfigs     []StorageConfig               `json:"storageConfigs,omitempty"`
	ServiceAccountName string                        `json:"serviceAccountName,omitempty"`
	Resources          *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ImagePullSecrets   []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector       map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations        []corev1.Toleration           `json:"tolerations,omitempty"`
	KafkaHeapOpts      string                        `json:"kafkaHeapOpts,omitempty"`
	KafkaJVMPerfOpts   string                        `json:"kafkaJvmPerfOpts,omitempty"`
}

// RackAwareness defines the required fields to enable kafka's rack aware feature
type RackAwareness struct {
	Labels []string `json:"labels"`
}

// CruiseControlConfig defines the config for Cruise Control
type CruiseControlConfig struct {
	CruiseControlEndpoint string                        `json:"cruiseControlEndpoint,omitempty"`
	Resources             *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ServiceAccountName    string                        `json:"serviceAccountName,omitempty"`
	ImagePullSecrets      []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector          map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations           []corev1.Toleration           `json:"tolerations,omitempty"`
	Config                string                        `json:"config,omitempty"`
	CapacityConfig        string                        `json:"capacityConfig,omitempty"`
	ClusterConfig         string                        `json:"clusterConfig,omitempty"`
	Image                 string                        `json:"image,omitempty"`
	InitContainerImage    string                        `json:"initContainerImage,omitempty"`
}

// EnvoyConfig defines the config for Envoy
type EnvoyConfig struct {
	Image                    string                        `json:"image"`
	Resources                *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ServiceAccountName       string                        `json:"serviceAccountName,omitempty"`
	ImagePullSecrets         []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector             map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations              []corev1.Toleration           `json:"tolerations,omitempty"`
	Annotations              map[string]string             `json:"annotations,omitempty"`
	LoadBalancerSourceRanges []string                      `json:"loadBalancerSourceRanges,omitempty"`
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
}

// SSLSecrets defines the Kafka SSL secrets
type SSLSecrets struct {
	TLSSecretName   string `json:"tlsSecretName"`
	JKSPasswordName string `json:"jksPasswordName"`
	Create          bool   `json:"create,omitempty"`
	// +kubebuilder:validation:Enum={"cert-manager","vault"}
	PKIBackend PKIBackend `json:"pkiBackend,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KafkaCluster is the Schema for the kafkaclusters API
type KafkaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaClusterSpec   `json:"spec,omitempty"`
	Status KafkaClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaClusterList contains a list of KafkaCluster
type KafkaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaCluster{}, &KafkaClusterList{})
}

//GetInitContainerImage returns the Init container image to use for CruiseControl
func (cConfig *CruiseControlConfig) GetInitContainerImage() string {
	if cConfig.InitContainerImage != "" {
		return cConfig.InitContainerImage
	}
	return "wurstmeister/kafka:2.12-2.1.0"
}

//GetLoadBalancerSourceRanges returns LoadBalancerSourceRanges to use for Envoy generated LoadBalancer
func (eConfig *EnvoyConfig) GetLoadBalancerSourceRanges() []string {
	return eConfig.LoadBalancerSourceRanges
}

//GetAnnotations returns Annotations to use for Envoy generated LoadBalancer
func (eConfig *EnvoyConfig) GetAnnotations() map[string]string {
	return eConfig.Annotations
}

//GetServiceAccount returns the Kubernetes Service Account to use for Kafka Cluster
func (bConfig *BrokerConfig) GetServiceAccount() string {
	if bConfig.ServiceAccountName != "" {
		return bConfig.ServiceAccountName
	}
	return "default"
}

//GetServiceAccount returns the Kubernetes Service Account to use for EnvoyConfig
func (eConfig *EnvoyConfig) GetServiceAccount() string {
	if eConfig.ServiceAccountName != "" {
		return eConfig.ServiceAccountName
	}
	return "default"
}

//GetServiceAccount returns the Kubernetes Service Account to use for CruiseControl
func (cConfig *CruiseControlConfig) GetServiceAccount() string {
	if cConfig.ServiceAccountName != "" {
		return cConfig.ServiceAccountName
	}
	return "default"
}

//GetTolerations returns the tolerations for the given broker
func (bConfig *BrokerConfig) GetTolerations() []corev1.Toleration {
	return bConfig.Tolerations
}

//GetTolerations returns the tolerations for envoy
func (eConfig *EnvoyConfig) GetTolerations() []corev1.Toleration {
	return eConfig.Tolerations
}

//GetTolerations returns the tolerations for cruise control
func (cConfig *CruiseControlConfig) GetTolerations() []corev1.Toleration {
	return cConfig.Tolerations
}

//GetNodeSelector returns the node selector for cruise control
func (cConfig *CruiseControlConfig) GetNodeSelector() map[string]string {
	return cConfig.NodeSelector
}

//GetNodeSelector returns the node selector for envoy
func (eConfig *EnvoyConfig) GetNodeSelector() map[string]string {
	return eConfig.NodeSelector
}

//GetNodeSelector returns the node selector for the given broker
func (bConfig *BrokerConfig) GetNodeSelector() map[string]string {
	return bConfig.NodeSelector
}

//GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (bConfig *BrokerConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return bConfig.ImagePullSecrets
}

//GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (eConfig *EnvoyConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return eConfig.ImagePullSecrets
}

//GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (cConfig *CruiseControlConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return cConfig.ImagePullSecrets
}

// GetResources returns the envoy specific Kubernetes resource
func (eConfig *EnvoyConfig) GetResources() *corev1.ResourceRequirements {
	if eConfig.Resources != nil {
		return eConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("100Mi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("100Mi"),
		},
	}
}

// GetResources returns the CC specific Kubernetes resource
func (cConfig *CruiseControlConfig) GetResources() *corev1.ResourceRequirements {
	if cConfig.Resources != nil {
		return cConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("512Mi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("512Mi"),
		},
	}
}

// GetResources returns the broker specific Kubernetes resource
func (bConfig *BrokerConfig) GetResources() *corev1.ResourceRequirements {
	if bConfig.Resources != nil {
		return bConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("1500m"),
			"memory": resource.MustParse("3Gi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("1000m"),
			"memory": resource.MustParse("2Gi"),
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
	return "solsson/kafka-cruise-control@sha256:f3f3775f3b5e2a5ae2da6fdae60ed118793ac32c80f13fba31be8f025a57f6ac"
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
