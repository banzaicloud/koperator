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
	"strings"

	"github.com/banzaicloud/istio-client-go/pkg/networking/v1alpha3"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// DefaultServiceAccountName name used for the various ServiceAccounts
	DefaultServiceAccountName = "default"
	defaultAnyCastPort        = 29092
)

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	HeadlessServiceEnabled bool            `json:"headlessServiceEnabled"`
	ListenersConfig        ListenersConfig `json:"listenersConfig"`
	// ZKAddresses specifies the ZooKeeper connection string
	// in the form hostname:port where host and port are the host and port of a ZooKeeper server.
	ZKAddresses []string `json:"zkAddresses"`
	// ZKPath specifies the ZooKeeper chroot path as part
	// of its ZooKeeper connection string which puts its data under some path in the global ZooKeeper namespace.
	ZKPath               string                  `json:"zkPath,omitempty"`
	RackAwareness        *RackAwareness          `json:"rackAwareness,omitempty"`
	ClusterImage         string                  `json:"clusterImage,omitempty"`
	ReadOnlyConfig       string                  `json:"readOnlyConfig,omitempty"`
	ClusterWideConfig    string                  `json:"clusterWideConfig,omitempty"`
	BrokerConfigGroups   map[string]BrokerConfig `json:"brokerConfigGroups,omitempty"`
	Brokers              []Broker                `json:"brokers"`
	DisruptionBudget     DisruptionBudget        `json:"disruptionBudget,omitempty"`
	RollingUpgradeConfig RollingUpgradeConfig    `json:"rollingUpgradeConfig"`
	// +kubebuilder:validation:Enum=envoy;istioingress
	IngressController string `json:"ingressController,omitempty"`
	// If true OneBrokerPerNode ensures that each kafka broker will be placed on a different node unless a custom
	// Affinity definition overrides this behavior
	OneBrokerPerNode        bool                `json:"oneBrokerPerNode"`
	PropagateLabels         bool                `json:"propagateLabels,omitempty"`
	CruiseControlConfig     CruiseControlConfig `json:"cruiseControlConfig"`
	EnvoyConfig             EnvoyConfig         `json:"envoyConfig,omitempty"`
	MonitoringConfig        MonitoringConfig    `json:"monitoringConfig,omitempty"`
	VaultConfig             VaultConfig         `json:"vaultConfig,omitempty"`
	AlertManagerConfig      *AlertManagerConfig `json:"alertManagerConfig,omitempty"`
	IstioIngressConfig      IstioIngressConfig  `json:"istioIngressConfig,omitempty"`
	Envs                    []corev1.EnvVar     `json:"envs,omitempty"`
	KubernetesClusterDomain string              `json:"kubernetesClusterDomain,omitempty"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster
type KafkaClusterStatus struct {
	BrokersState             map[string]BrokerState   `json:"brokersState,omitempty"`
	CruiseControlTopicStatus CruiseControlTopicStatus `json:"cruiseControlTopicStatus,omitempty"`
	State                    ClusterState             `json:"state"`
	RollingUpgrade           RollingUpgradeStatus     `json:"rollingUpgradeStatus,omitempty"`
	AlertCount               int                      `json:"alertCount"`
	ListenerStatuses         ListenerStatuses         `json:"listenerStatuses,omitempty"`
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

// DisruptionBudget defines the configuration for PodDisruptionBudget
type DisruptionBudget struct {
	// If set to true, will create a podDisruptionBudget
	// +optional
	Create bool `json:"create,omitempty"`
	// The budget to set for the PDB, can either be static number or a percentage
	// +kubebuilder:validation:Pattern:="^[0-9]+$|^[0-9]{1,2}%$|^100%$"
	Budget string `json:"budget,omitempty"`
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
	Config             string                        `json:"config,omitempty"`
	StorageConfigs     []StorageConfig               `json:"storageConfigs,omitempty"`
	ServiceAccountName string                        `json:"serviceAccountName,omitempty"`
	Resources          *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ImagePullSecrets   []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector       map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations        []corev1.Toleration           `json:"tolerations,omitempty"`
	KafkaHeapOpts      string                        `json:"kafkaHeapOpts,omitempty"`
	KafkaJVMPerfOpts   string                        `json:"kafkaJvmPerfOpts,omitempty"`
	// Override for the default log4j configuration
	Log4jConfig string `json:"log4jConfig,omitempty"`
	// Custom annotations for the broker pods - e.g.: Prometheus scraping annotations:
	// prometheus.io/scrape: "true"
	// prometheus.io/port: "9020"
	BrokerAnnotations map[string]string `json:"brokerAnnotations,omitempty"`
	// Network throughput information in kB/s used by Cruise Control to determine broker network capacity.
	// By default it is set to `125000` which means 1Gbit/s in network throughput.
	NetworkConfig *NetworkConfig `json:"networkConfig,omitempty"`
	// External listeners that use NodePort type service to expose the broker outside the Kubernetes clusterT and their
	// external IP to advertise Kafka broker external listener. The external IP value is ignored in case of external listeners that use LoadBalancer
	// type service to expose the broker outside the Kubernetes cluster. Also, when "hostnameOverride" field of the external listener is set
	// it will override the broker's external listener advertise address according to the description of the "hostnameOverride" field.
	NodePortExternalIP map[string]string `json:"nodePortExternalIP,omitempty"`
	// Any definition received through this field will override the default behaviour of OneBrokerPerNode flag
	// and the operator supposes that the user is aware of how scheduling is done by kubernetes
	// Affinity could be set through brokerConfigGroups definitions and can be set for individual brokers as well
	// where letter setting will override the group setting
	Affinity           *corev1.Affinity           `json:"affinity,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// SecurityContext allows to set security context for the kafka container
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
	// BrokerIdBindings allows to set specific ingress to a specific broker mappings.
	// If left empty, all broker will inherit the default one specified under external listeners config
	BrokerIdBindings []string `json:"brokerIdBindings,omitempty"`
}

type NetworkConfig struct {
	IncomingNetworkThroughPut string `json:"incomingNetworkThroughPut,omitempty"`
	OutgoingNetworkThroughPut string `json:"outgoingNetworkThroughPut,omitempty"`
}

// RackAwareness defines the required fields to enable kafka's rack aware feature
type RackAwareness struct {
	Labels []string `json:"labels"`
}

// CruiseControlConfig defines the config for Cruise Control
type CruiseControlConfig struct {
	CruiseControlTaskSpec CruiseControlTaskSpec         `json:"cruiseControlTaskSpec,omitempty"`
	CruiseControlEndpoint string                        `json:"cruiseControlEndpoint,omitempty"`
	Resources             *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ServiceAccountName    string                        `json:"serviceAccountName,omitempty"`
	ImagePullSecrets      []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector          map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations           []corev1.Toleration           `json:"tolerations,omitempty"`
	Config                string                        `json:"config,omitempty"`
	CapacityConfig        string                        `json:"capacityConfig,omitempty"`
	ClusterConfig         string                        `json:"clusterConfig,omitempty"`
	Log4jConfig           string                        `json:"log4jConfig,omitempty"`
	Image                 string                        `json:"image,omitempty"`
	TopicConfig           *TopicConfig                  `json:"topicConfig,omitempty"`
	//  Annotations to be applied to CruiseControl pod
	// +optional
	CruiseControlAnnotations map[string]string `json:"cruiseControlAnnotations,omitempty"`
	// InitContainers add extra initContainers to CruiseControl pod
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	// Volumes define some extra Kubernetes Volumes for the CruiseControl Pods.
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// VolumeMounts define some extra Kubernetes Volume mounts for the CruiseControl Pods.
	VolumeMounts       []corev1.VolumeMount       `json:"volumeMounts,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// SecurityContext allows to set security context for the CruiseControl container
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// CruiseControlTaskSpec specifies the configuration of the CC Tasks
type CruiseControlTaskSpec struct {
	// RetryDurationMinutes describes the amount of time the Operator waits for the task
	RetryDurationMinutes int `json:"RetryDurationMinutes"`
}

// TopicConfig holds info for topic configuration regarding partitions and replicationFactor
type TopicConfig struct {
	Partitions int32 `json:"partitions"`
	// +kubebuilder:validation:Minimum=2
	ReplicationFactor int32 `json:"replicationFactor"`
}

// EnvoyConfig defines the config for Envoy
type EnvoyConfig struct {
	Image     string                       `json:"image,omitempty"`
	Resources *corev1.ResourceRequirements `json:"resourceRequirements,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Replicas           int32                         `json:"replicas,omitempty"`
	ServiceAccountName string                        `json:"serviceAccountName,omitempty"`
	ImagePullSecrets   []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector       map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations        []corev1.Toleration           `json:"tolerations,omitempty"`
	// Annotations defines the annotations placed on the envoy ingress controller deployment
	Annotations              map[string]string `json:"annotations,omitempty"`
	LoadBalancerSourceRanges []string          `json:"loadBalancerSourceRanges,omitempty"`
	// LoadBalancerIP can be used to specify an exact IP for the LoadBalancer service
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`
}

// IstioIngressConfig defines the config for the Istio Ingress Controller
type IstioIngressConfig struct {
	Resources *corev1.ResourceRequirements `json:"resourceRequirements,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Replicas     int32               `json:"replicas,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	// Annotations defines the annotations placed on the istio ingress controller deployment
	Annotations               map[string]string    `json:"annotations,omitempty"`
	TLSOptions                *v1alpha3.TLSOptions `json:"gatewayConfig,omitempty"`
	VirtualServiceAnnotations map[string]string    `json:"virtualServiceAnnotations,omitempty"`
}

func (iIConfig *IstioIngressConfig) GetAnnotations() map[string]string {
	return cloneAnnotationMap(iIConfig.Annotations)
}

// GetVirtualServiceAnnotations returns a copy of the VirtualServiceAnnotations field
func (iIConfig *IstioIngressConfig) GetVirtualServiceAnnotations() map[string]string {
	return cloneAnnotationMap(iIConfig.VirtualServiceAnnotations)
}

// MonitoringConfig defines the config for monitoring Kafka and Cruise Control
type MonitoringConfig struct {
	JmxImage               string `json:"jmxImage,omitempty"`
	PathToJar              string `json:"pathToJar,omitempty"`
	KafkaJMXExporterConfig string `json:"kafkaJMXExporterConfig,omitempty"`
	CCJMXExporterConfig    string `json:"cCJMXExporterConfig,omitempty"`
}

// StorageConfig defines the broker storage configuration
type StorageConfig struct {
	MountPath string                            `json:"mountPath"`
	PvcSpec   *corev1.PersistentVolumeClaimSpec `json:"pvcSpec"`
}

//ListenersConfig defines the Kafka listener types
type ListenersConfig struct {
	ExternalListeners  []ExternalListenerConfig `json:"externalListeners,omitempty"`
	InternalListeners  []InternalListenerConfig `json:"internalListeners"`
	SSLSecrets         *SSLSecrets              `json:"sslSecrets,omitempty"`
	ServiceAnnotations map[string]string        `json:"serviceAnnotations,omitempty"`
}

// GetServiceAnnotations returns a copy of the ServiceAnnotations field.
func (c ListenersConfig) GetServiceAnnotations() map[string]string {
	return cloneAnnotationMap(c.ServiceAnnotations)
}

func (c ExternalListenerConfig) GetAccessMethod() corev1.ServiceType {
	if c.AccessMethod == "" {
		return corev1.ServiceTypeLoadBalancer
	}
	return c.AccessMethod
}

func (c ExternalListenerConfig) GetAnyCastPort() int32 {
	if c.AnyCastPort == nil {
		return defaultAnyCastPort
	}
	return *c.AnyCastPort
}

// GetServiceAnnotations returns a copy of the ServiceAnnotations field.
func (c IngressServiceSettings) GetServiceAnnotations() map[string]string {
	return cloneAnnotationMap(c.ServiceAnnotations)
}

// GetServiceType returns the field value of ServiceType defaults to LoadBalancer.
func (c IngressServiceSettings) GetServiceType() corev1.ServiceType {
	if c.ServiceType == "" {
		return corev1.ServiceTypeLoadBalancer
	}
	return c.ServiceType
}

// SSLSecrets defines the Kafka SSL secrets
type SSLSecrets struct {
	TLSSecretName   string                  `json:"tlsSecretName"`
	JKSPasswordName string                  `json:"jksPasswordName"`
	Create          bool                    `json:"create,omitempty"`
	IssuerRef       *cmmeta.ObjectReference `json:"issuerRef,omitempty"`
	// +kubebuilder:validation:Enum={"cert-manager","vault"}
	PKIBackend PKIBackend `json:"pkiBackend,omitempty"`
}

// TODO (tinyzimmer): The above are all optional now in one way or another.
// Would be another good use-case for a pre-admission hook
// E.g. TLSSecretName and JKSPasswordName are only required if Create is false
// Or heck, do we even want to bother supporting an imported PKI?

// VaultConfig defines the configuration for a vault PKI backend
type VaultConfig struct {
	AuthRole  string `json:"authRole"`
	PKIPath   string `json:"pkiPath"`
	IssuePath string `json:"issuePath"`
	UserStore string `json:"userStore"`
}

// AlertManagerConfig defines configuration for alert manager
type AlertManagerConfig struct {
	// DownScaleLimit the limit for auto-downscaling the Kafka cluster.
	// Once the size of the cluster (number of brokers) reaches or falls below this limit the auto-downscaling triggered by alerts is disabled until the cluster size exceeds this limit.
	// This limit is not enforced if this field is omitted or is <= 0.
	DownScaleLimit int `json:"downScaleLimit,omitempty"`
	// UpScaleLimit the limit for auto-upscaling the Kafka cluster.
	// Once the size of the cluster (number of brokers) reaches or exceeds this limit the auto-upscaling triggered by alerts is disabled until the cluster size falls below this limit.
	// This limit is not enforced if this field is omitted or is <= 0.
	UpScaleLimit int `json:"upScaleLimit,omitempty"`
}

type IngressServiceSettings struct {
	// In case of external listeners using LoadBalancer access method the value of this field is used to advertise the
	// Kafka broker external listener instead of the public IP of the provisioned LoadBalancer service (e.g. can be used to
	// advertise the listener using a URL recorded in DNS instead of public IP).
	// In case of external listeners using NodePort access method the broker instead of node public IP (see "brokerConfig.nodePortExternalIP")
	// is advertised on the address having the following format: <kafka-cluster-name>-<broker-id>.<namespace><value-specified-in-hostnameOverride-field>
	HostnameOverride string `json:"hostnameOverride,omitempty"`
	// ServiceAnnotations defines annotations which will
	// be placed to the service or services created for the external listener
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`
	// externalTrafficPolicy denotes if this Service desires to route external
	// traffic to node-local or cluster-wide endpoints. "Local" preserves the
	// client source IP and avoids a second hop for LoadBalancer and Nodeport
	// type services, but risks potentially imbalanced traffic spreading.
	// "Cluster" obscures the client source IP and may cause a second hop to
	// another node, but should have good overall load-spreading.
	// +optional
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
	// Service Type string describes ingress methods for a service
	// Only "NodePort" and "LoadBalancer" is supported.
	// Default value is LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

// ExternalListenerConfig defines the external listener config for Kafka
type ExternalListenerConfig struct {
	CommonListenerSpec     `json:",inline"`
	IngressServiceSettings `json:",inline"`
	ExternalStartingPort   int32 `json:"externalStartingPort"`
	// configuring AnyCastPort allows kafka cluster access without specifying the exact broker
	AnyCastPort *int32 `json:"anyCastPort,omitempty"`
	// +kubebuilder:validation:Enum=LoadBalancer;NodePort
	// accessMethod defines the method which the external listener is exposed through.
	// Two types are supported LoadBalancer and NodePort.
	// The recommended and default is the LoadBalancer.
	// NodePort should be used in Kubernetes environments with no support for provisioning Load Balancers.
	// +optional
	AccessMethod corev1.ServiceType `json:"accessMethod,omitempty"`
	// configuring Config allows to specify ingress controller configuration per external listener
	// if not set the default IstioIngressConfig or EnvoyConfig will be used from the KafkaClusterSpec
	Config *Config `json:"config,omitempty"`
}

// Config defines the external access ingress controller configuration
type Config struct {
	DefaultIngressConfig string                   `json:"defaultIngressConfig"`
	IngressConfig        map[string]IngressConfig `json:"ingressConfig,omitempty"`
}

type IngressConfig struct {
	IngressServiceSettings `json:",inline"`
	IstioIngressConfig     *IstioIngressConfig `json:"istioIngressConfig,omitempty"`
	EnvoyConfig            *EnvoyConfig        `json:"envoyConfig,omitempty"`
}

// InternalListenerConfig defines the internal listener config for Kafka
type InternalListenerConfig struct {
	CommonListenerSpec              `json:",inline"`
	UsedForInnerBrokerCommunication bool `json:"usedForInnerBrokerCommunication"`
	UsedForControllerCommunication  bool `json:"usedForControllerCommunication,omitempty"`
}

// CommonListenerSpec defines the common building block for Listener type
type CommonListenerSpec struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	ContainerPort int32  `json:"containerPort"`
}

// ListenerStatuses holds information about the statuses of the configured listeners.
// The internal and external listeners are stored in separate maps, and each listener can be looked up by name.
type ListenerStatuses struct {
	InternalListeners map[string]ListenerStatusList `json:"internalListeners,omitempty"`
	ExternalListeners map[string]ListenerStatusList `json:"externalListeners,omitempty"`
}

// ListenerStatusList can hold various amount of statuses based on the listener configuration.
type ListenerStatusList []ListenerStatus

// ListenerStatus holds information about the address of the listener
type ListenerStatus struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.state",name="Cluster state",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.alertCount",name="Cluster alert count",type="integer"
// +kubebuilder:printcolumn:JSONPath=".status.rollingUpgradeStatus.lastSuccess",name="Last successful upgrade",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.rollingUpgradeStatus.errorCount",name="Upgrade error count",type="string"
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name="Age",type="date"

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

// GetResources returns the IstioIngress specific Kubernetes resources
func (iIConfig *IstioIngressConfig) GetResources() *corev1.ResourceRequirements {
	if iIConfig.Resources != nil {
		return iIConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("2000m"),
			"memory": resource.MustParse("1024Mi"),
		},
	}
}

// GetListenerName returns the prepared listener name
func (lP *CommonListenerSpec) GetListenerServiceName() string {
	if !strings.HasPrefix(lP.Name, "tcp-") {
		return "tcp-" + lP.Name
	}
	return lP.Name
}

// GetReplicas returns replicas used by the Istio Ingress deployment
func (iIConfig *IstioIngressConfig) GetReplicas() int32 {
	if iIConfig.Replicas == 0 {
		return 1
	}
	return iIConfig.Replicas
}

// GetIngressController returns the default Envoy ingress controller if not specified otherwise
func (kSpec *KafkaClusterSpec) GetIngressController() string {
	if kSpec.IngressController == "" {
		return "envoy"
	}
	return kSpec.IngressController
}

// GetDomain returns the default domain if not specified otherwise
func (kSpec *KafkaClusterSpec) GetKubernetesClusterDomain() string {
	if kSpec.KubernetesClusterDomain == "" {
		return "cluster.local"
	}
	return kSpec.KubernetesClusterDomain
}

// GetZkPath returns the default "/" ZkPath if not specified otherwise
func (kSpec *KafkaClusterSpec) GetZkPath() string {
	const prefix = "/"
	if kSpec.ZKPath == "" {
		return prefix
	} else if !strings.HasPrefix(kSpec.ZKPath, prefix) {
		return prefix + kSpec.ZKPath
	} else {
		return kSpec.ZKPath
	}
}

// GetClusterImage returns the default container image for Kafka Cluster
func (kSpec *KafkaClusterSpec) GetClusterImage() string {
	if kSpec.ClusterImage != "" {
		return kSpec.ClusterImage
	}
	return "ghcr.io/banzaicloud/kafka:2.13-2.6.0-bzc.1"
}

func (cTaskSpec *CruiseControlTaskSpec) GetDurationMinutes() float64 {
	if cTaskSpec.RetryDurationMinutes == 0 {
		return 5
	}
	return float64(cTaskSpec.RetryDurationMinutes)
}

//GetLoadBalancerSourceRanges returns LoadBalancerSourceRanges to use for Envoy generated LoadBalancer
func (eConfig *EnvoyConfig) GetLoadBalancerSourceRanges() []string {
	return eConfig.LoadBalancerSourceRanges
}

//GetAnnotations returns Annotations to use for Envoy generated LoadBalancer
func (eConfig *EnvoyConfig) GetAnnotations() map[string]string {
	return cloneAnnotationMap(eConfig.Annotations)
}

// GetReplicas returns replicas used by the Envoy deployment
func (eConfig *EnvoyConfig) GetReplicas() int32 {
	if eConfig.Replicas == 0 {
		return 1
	}
	return eConfig.Replicas
}

//GetServiceAccount returns the Kubernetes Service Account to use for Kafka Cluster
func (bConfig *BrokerConfig) GetServiceAccount() string {
	if bConfig.ServiceAccountName != "" {
		return bConfig.ServiceAccountName
	}
	return DefaultServiceAccountName
}

//GetServiceAccount returns the Kubernetes Service Account to use for EnvoyConfig
func (eConfig *EnvoyConfig) GetServiceAccount() string {
	if eConfig.ServiceAccountName != "" {
		return eConfig.ServiceAccountName
	}
	return DefaultServiceAccountName
}

//GetServiceAccount returns the Kubernetes Service Account to use for CruiseControl
func (cConfig *CruiseControlConfig) GetServiceAccount() string {
	if cConfig.ServiceAccountName != "" {
		return cConfig.ServiceAccountName
	}
	return DefaultServiceAccountName
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

// GetBrokerAnnotations return the annotations which applied to broker pods
func (bConfig *BrokerConfig) GetBrokerAnnotations() map[string]string {
	return cloneAnnotationMap(bConfig.BrokerAnnotations)
}

// GetCruiseControlAnnotations return the annotations which applied to CruiseControl pod
func (cConfig *CruiseControlConfig) GetCruiseControlAnnotations() map[string]string {
	return cloneAnnotationMap(cConfig.CruiseControlAnnotations)
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

	return "envoyproxy/envoy:v1.14.4"
}

// GetCCImage returns the used Cruise Control image
func (cConfig *CruiseControlConfig) GetCCImage() string {
	if cConfig.Image != "" {
		return cConfig.Image
	}
	return "ghcr.io/banzaicloud/cruise-control:2.5.28"
}

// GetCCLog4jConfig returns the used Cruise Control log4j configuration
func (cConfig *CruiseControlConfig) GetCCLog4jConfig() string {
	if cConfig.Log4jConfig != "" {
		return cConfig.Log4jConfig
	}
	return `log4j.rootLogger = INFO, FILE
log4j.appender.FILE=org.apache.log4j.ConsoleAppender
log4j.appender.FILE.layout=org.apache.log4j.PatternLayout
log4j.appender.FILE.layout.conversionPattern=%-6r [%15.15t] %-5p %30.30c %x - %m%n`
}

// GetImage returns the used image for Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetImage() string {
	if mConfig.JmxImage != "" {
		return mConfig.JmxImage
	}
	return "ghcr.io/banzaicloud/jmx-javaagent:0.14.0"
}

// GetPathToJar returns the path in the used Image for Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetPathToJar() string {
	if mConfig.PathToJar != "" {
		return mConfig.PathToJar
	}
	return "/opt/jmx_exporter/jmx_prometheus_javaagent-0.14.0.jar"
}

// GetKafkaJMXExporterConfig returns the config for Kafka Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetKafkaJMXExporterConfig() string {
	if mConfig.KafkaJMXExporterConfig != "" {
		return mConfig.KafkaJMXExporterConfig
	}
	// Use upstream defined rules https://github.com/prometheus/jmx_exporter/blob/master/example_configs/kafka-2_0_0.yml
	return `lowercaseOutputName: true
rules:
# Special cases and very specific rules
- pattern : 'kafka.server<type=app-info><>version: (.*)'
  name: kafka_server_app_info
  value: 1.0
  labels:
    version: "$1"
- pattern : kafka.server<type=(.+), name=(.+), clientId=(.+), topic=(.+), partition=(.*)><>Value
  name: kafka_server_$1_$2
  type: GAUGE
  cache: true
  labels:
    clientId: "$3"
    topic: "$4"
    partition: "$5"
- pattern : kafka.server<type=(.+), name=(.+), clientId=(.+), brokerHost=(.+), brokerPort=(.+)><>Value
  name: kafka_server_$1_$2
  type: GAUGE
  cache: true
  labels:
    clientId: "$3"
    broker: "$4:$5"
- pattern: kafka.server<type=(.+), cipher=(.+), protocol=(.+), listener=(.+), networkProcessor=(.+)><>connections
  name: kafka_server_$1_connections_tls_info
  type: GAUGE
  cache: true
  labels:
    listener: "$2"
    networkProcessor: "$3"
    protocol: "$4"
    cipher: "$5"
- pattern: kafka.server<type=(.+), clientSoftwareName=(.+), clientSoftwareVersion=(.+), listener=(.+), networkProcessor=(.+)><>connections
  name: kafka_server_$1_connections_software
  type: GAUGE
  cache: true
  labels:
    clientSoftwareName: "$2"
    clientSoftwareVersion: "$3"
    listener: "$4"
    networkProcessor: "$5"
- pattern: kafka.server<type=(.+), listener=(.+), networkProcessor=(.+)><>([a-z-]+)
  name: kafka_server_$1_$4
  type: GAUGE
  cache: true
  labels:
    listener: "$2"
    networkProcessor: "$3"
- pattern : kafka.coordinator.(\w+)<type=(.+), name=(.+)><>Value
  name: kafka_coordinator_$1_$2_$3
  cache: true
  type: GAUGE

# Some percent metrics use *Rate attributes
# e.g kafka.server<type=(KafkaRequestHandlerPool), name=(RequestHandlerAvgIdlePercent)><>MeanRate
- pattern: kafka.(\w+)<type=(.+), name=(.+)Percent\w*><>MeanRate
  name: kafka_$1_$2_$3_meanrate_percent
  type: GAUGE
  cache: true
- pattern: kafka.(\w+)<type=(.+), name=(.+)Percent\w*><>OneMinuteRate
  name: kafka_$1_$2_$3_oneminuterate_percent
  type: GAUGE
  cache: true
- pattern: kafka.(\w+)<type=(.+), name=(.+)Percent\w*><>FiveMinuteRate
  name: kafka_$1_$2_$3_fiveminuterate_percent
  type: GAUGE
  cache: true
- pattern: kafka.(\w+)<type=(.+), name=(.+)Percent\w*><>FifteenMinuteRate
  name: kafka_$1_$2_$3_fifteenminuterate_percent
  type: GAUGE
  cache: true
# Generic gauges for percents
- pattern: kafka.(\w+)<type=(.+), name=(.+)Percent\w*><>Value
  name: kafka_$1_$2_$3_percent
  type: GAUGE
  cache: true
- pattern: kafka.(\w+)<type=(.+), name=(.+)Percent\w*, (.+)=(.+)><>Value
  name: kafka_$1_$2_$3_percent
  type: GAUGE
  cache: true
  labels:
    "$4": "$5"
# Generic per-second counters with 0-3 key/value pairs
- pattern: kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, (.+)=(.+), (.+)=(.+), (.+)=(.+)><>Count
  name: kafka_$1_$2_$3_total
  type: COUNTER
  cache: true
  labels:
    "$4": "$5"
    "$6": "$7"
    "$8": "$9"
- pattern: kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, (.+)=(.+), (.+)=(.+)><>Count
  name: kafka_$1_$2_$3_total
  cache: true
  type: COUNTER
  labels:
    "$4": "$5"
    "$6": "$7"
- pattern: kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, (.+)=(.+)><>Count
  name: kafka_$1_$2_$3_total
  cache: true
  type: COUNTER
  labels:
    "$4": "$5"
- pattern: kafka.(\w+)<type=(.+), name=(.+)PerSec\w*><>Count
  name: kafka_$1_$2_$3_total
  cache: true
  type: COUNTER

- pattern: kafka.server<type=(.+), client-id=(.+)><>([a-z-]+)
  name: kafka_server_quota_$3
  cache: true
  type: GAUGE
  labels:
    resource: "$1"
    clientId: "$2"

- pattern: kafka.server<type=(.+), user=(.+), client-id=(.+)><>([a-z-]+)
  name: kafka_server_quota_$4
  cache: true
  type: GAUGE
  labels:
    resource: "$1"
    user: "$2"
    clientId: "$3"

# Generic gauges with 0-3 key/value pairs
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.+), (.+)=(.+), (.+)=(.+)><>Value
  name: kafka_$1_$2_$3
  type: GAUGE
  cache: true
  labels:
    "$4": "$5"
    "$6": "$7"
    "$8": "$9"
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.+), (.+)=(.+)><>Value
  name: kafka_$1_$2_$3
  cache: true
  type: GAUGE
  labels:
    "$4": "$5"
    "$6": "$7"
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.+)><>Value
  name: kafka_$1_$2_$3
  cache: true
  type: GAUGE
  labels:
    "$4": "$5"
- pattern: kafka.(\w+)<type=(.+), name=(.+)><>Value
  name: kafka_$1_$2_$3
  cache: true
  type: GAUGE

# Emulate Prometheus 'Summary' metrics for the exported 'Histogram's.
#
# Note that these are missing the '_sum' metric!
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.+), (.+)=(.+)><>Count
  name: kafka_$1_$2_$3_count
  cache: true
  type: COUNTER
  labels:
    "$4": "$5"
    "$6": "$7"
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.*), (.+)=(.+)><>(\d+)thPercentile
  name: kafka_$1_$2_$3
  cache: true
  type: GAUGE
  labels:
    "$4": "$5"
    "$6": "$7"
    quantile: "0.$8"
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.+)><>Count
  name: kafka_$1_$2_$3_count
  cache: true
  type: COUNTER
  labels:
    "$4": "$5"
- pattern: kafka.(\w+)<type=(.+), name=(.+), (.+)=(.*)><>(\d+)thPercentile
  name: kafka_$1_$2_$3
  cache: true
  type: GAUGE
  labels:
    "$4": "$5"
    quantile: "0.$6"
- pattern: kafka.(\w+)<type=(.+), name=(.+)><>Count
  cache: true
  name: kafka_$1_$2_$3_count
  type: COUNTER
- pattern: kafka.(\w+)<type=(.+), name=(.+)><>(\d+)thPercentile
  cache: true
  name: kafka_$1_$2_$3
  type: GAUGE
  labels:
    quantile: "0.$4"
# Catch all other GAUGES with other types with 0-2 key-value pairs
- pattern : kafka.(\w+)<type=([A-Za-z-]+), (.+)=(.+), (.+)=(.+)><>([A-Za-z-]+)
  name: kafka_$1_$2_$7
  type: GAUGE
  cache: true
  labels:
    "$3": "$4"
    "$5": "$6"
- pattern : kafka.(\w+)<type=([A-Za-z-]+), (.+)=(.+)><>([A-Za-z-]+)
  name: kafka_$1_$2_$5
  type: GAUGE
  cache: true
  labels:
    "$3": "$4"
- pattern : kafka.(\w+)<type=([A-Za-z-]+)><>([A-Za-z-]+)
  name: kafka_$1_$2_$3
  type: GAUGE
  cache: true
# Export all other java.{lang,nio}* beans using default format
- pattern: java.lang.+
- pattern: java.nio.+`
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

func cloneAnnotationMap(original map[string]string) map[string]string {
	m := make(map[string]string, len(original))
	for k, v := range original {
		m[k] = v
	}
	return m
}
