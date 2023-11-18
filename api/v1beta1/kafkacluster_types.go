// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"fmt"
	"strings"

	"emperror.dev/errors"

	"dario.cat/mergo"

	"github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/assets"
	"github.com/banzaicloud/koperator/api/util"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	AppLabelKey      = "app"
	KafkaCRLabelKey  = "kafka_cr"
	BrokerIdLabelKey = "brokerId"

	// These are default values for API keys

	/* General Config */
	defaultServiceAccountName = "default"

	/* External Listener Config */

	// defaultAnyCastPort kafka anycast port that can be used by clients for metadata queries
	defaultAnyCastPort                 = 29092
	defaultIngressControllerTargetPort = 29092

	/* Envoy Config */

	// KafkaClusterDeployment.spec.replicas
	defaultEnvoyReplicas = 1
	// KafkaClusterDeployment.spec.template.spec.container["envoy"].port["tcp-admin"].containerPort
	defaultEnvoyAdminPort = 8081
	// KafkaClusterDeployment.spec.template.spec.container["envoy"].port["tcp-health"].containerPort
	defaultEnvoyHealthCheckPort = 8080
	// KafkaClusterDeployment.spec.template.spec.container["envoy"].args
	defaultEnvoyConcurrency = 0

	// KafkaClusterDeployment.spec.template.spec.container["envoy"].resource
	defaultEnvoyRequestResourceCpu    = "100m"
	defaultEnvoyRequestResourceMemory = "100Mi"
	defaultEnvoyLimitResourceCpu      = "100m"
	defaultEnvoyLimitResourceMemory   = "100Mi"

	// KafkaClusterDeployment.spec.template.spec.container["envoy"].image
	defaultEnvoyImage = "envoyproxy/envoy:v1.22.2"

	/* Broker Config */

	// KafkaBrokerPod.spec.terminationGracePeriodSeconds
	defaultBrokerTerminationGracePeriod = 120

	// KafkaBrokerPod.spec.container["kafka"].resource
	defaultBrokerRequestResourceCpu    = "1000m"
	defaultBrokerRequestResourceMemory = "2Gi"
	defaultBrokerLimitResourceCpu      = "1500m"
	defaultBrokerLimitResourceMemory   = "3Gi"

	defaultBrokerHeapOpts    = "-Xmx2G -Xms2G"
	defaultBrokerPerfJvmOpts = "-server -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:+ExplicitGCInvokesConcurrent -Djava.awt.headless=true -Dsun.net.inetaddr.ttl=60"

	/* Cruise Control Config */

	// CruiseControlDeployment.spec.template.spec.container["%s-cruisecontrol"].image
	defaultCruiseControlImage = "ghcr.io/banzaicloud/cruise-control:2.5.123"

	// CruiseControlDeployment.spec.template.spec.container["%s-cruisecontrol"].resources
	defaultCruiseControlRequestResourceCpu    = "200m"
	defaultCruiseControlRequestResourceMemory = "768Mi"
	defaultCruiseControlLimitResourceCpu      = "200m"
	defaultCruiseControlLimitResourceMemory   = "768Mi"

	// Cruise Control Task
	defaultCruiseControlTaskDurationMin = 5

	// Kafka Cluster Spec
	defaultKafkaClusterIngressController = "envoy"
	defaultKafkaClusterK8sClusterDomain  = "cluster.local"

	// KafkaBroker.spec.container["kafka"].image
	defaultKafkaImage = "ghcr.io/banzaicloud/kafka:2.13-3.4.1"

	/* Istio Ingress Config */

	// IstioMeshGateway.spec.deployment.resources
	defaultIstioIngressRequestResourceCpu    = "100m"
	defaultIstioIngressRequestResourceMemory = "128Mi"
	defaultIstioIngressLimitResourceCpu      = "2000m"
	defaultIstioIngressLimitResourceMemory   = "1024Mi"

	// IstioMeshGateway.spec.deployment.replicas.count
	// IstioMeshGateway.spec.deployment.replicas.min
	// IstioMeshGateway.spec.deployment.replicas.max
	defaultReplicas = 1

	/* Monitor Config */

	// KafkaBrokerPod.spec.initContainer[jmx-exporter].image
	// kafkaClusterDeployment.spec.template.spec.initContainer["jmx-exporter"].image
	defaultMonitorImage = "ghcr.io/banzaicloud/jmx-javaagent:0.16.1"

	// KafkaBrokerPod.spec.initContainer["jmx-exporter"].command
	// kafkaClusterDeployment.spec.template.spec.initContainer["jmx-exporter"].command
	defaultMonitorPathToJar = "/jmx_prometheus_javaagent.jar"
)

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	HeadlessServiceEnabled bool            `json:"headlessServiceEnabled"`
	ListenersConfig        ListenersConfig `json:"listenersConfig"`
	// Custom ports to expose in the container. Example use case: a custom kafka distribution, that includes an integrated metrics api endpoint
	AdditionalPorts []corev1.ContainerPort `json:"additionalPorts,omitempty"`
	// ZKAddresses specifies the ZooKeeper connection string
	// in the form hostname:port where host and port are the host and port of a ZooKeeper server.
	ZKAddresses []string `json:"zkAddresses"`
	// ZKPath specifies the ZooKeeper chroot path as part
	// of its ZooKeeper connection string which puts its data under some path in the global ZooKeeper namespace.
	ZKPath                      string                  `json:"zkPath,omitempty"`
	RackAwareness               *RackAwareness          `json:"rackAwareness,omitempty"`
	ClusterImage                string                  `json:"clusterImage,omitempty"`
	ClusterMetricsReporterImage string                  `json:"clusterMetricsReporterImage,omitempty"`
	ReadOnlyConfig              string                  `json:"readOnlyConfig,omitempty"`
	ClusterWideConfig           string                  `json:"clusterWideConfig,omitempty"`
	BrokerConfigGroups          map[string]BrokerConfig `json:"brokerConfigGroups,omitempty"`
	Brokers                     []Broker                `json:"brokers"`
	DisruptionBudget            DisruptionBudget        `json:"disruptionBudget,omitempty"`
	RollingUpgradeConfig        RollingUpgradeConfig    `json:"rollingUpgradeConfig"`
	// +kubebuilder:validation:Enum=envoy;istioingress
	// IngressController specifies the type of the ingress controller to be used for external listeners. The `istioingress` ingress controller type requires the `spec.istioControlPlane` field to be populated as well.
	IngressController string `json:"ingressController,omitempty"`
	// IstioControlPlane is a reference to the IstioControlPlane resource for envoy configuration. It must be specified if istio ingress is used.
	IstioControlPlane *IstioControlPlaneReference `json:"istioControlPlane,omitempty"`
	// If true OneBrokerPerNode ensures that each kafka broker will be placed on a different node unless a custom
	// Affinity definition overrides this behavior
	OneBrokerPerNode bool `json:"oneBrokerPerNode"`
	// RemoveUnusedIngressResources when true, the unnecessary resources from the previous ingress state will be removed.
	// when false, they will be kept so the Kafka cluster remains available for those Kafka clients which are still using the previous ingress setting.
	// +kubebuilder:default=false
	// +optional
	RemoveUnusedIngressResources bool                `json:"removeUnusedIngressResources,omitempty"`
	PropagateLabels              bool                `json:"propagateLabels,omitempty"`
	CruiseControlConfig          CruiseControlConfig `json:"cruiseControlConfig"`
	EnvoyConfig                  EnvoyConfig         `json:"envoyConfig,omitempty"`
	MonitoringConfig             MonitoringConfig    `json:"monitoringConfig,omitempty"`
	AlertManagerConfig           *AlertManagerConfig `json:"alertManagerConfig,omitempty"`
	IstioIngressConfig           IstioIngressConfig  `json:"istioIngressConfig,omitempty"`
	// Envs defines environment variables for Kafka broker Pods.
	// Adding the "+" prefix to the name prepends the value to that environment variable instead of overwriting it.
	// Add the "+" suffix to append.
	Envs                    []corev1.EnvVar `json:"envs,omitempty"`
	KubernetesClusterDomain string          `json:"kubernetesClusterDomain,omitempty"`
	// ClientSSLCertSecret is a reference to the Kubernetes secret where custom client SSL certificate can be provided.
	// It will be used by the koperator, cruise control, cruise control metrics reporter
	// to communicate on SSL with that internal listener which is used for interbroker communication.
	// The client certificate must share the same chain of trust as the server certificate used by the corresponding internal listener.
	// The secret must contain the keystore, truststore jks files and the password for them in base64 encoded format
	// under the keystore.jks, truststore.jks, password data fields.
	ClientSSLCertSecret *corev1.LocalObjectReference `json:"clientSSLCertSecret,omitempty"`
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
	// ErrorCount keeps track the number of errors reported by alerts labeled with 'rollingupgrade'.
	// It's reset once these alerts stop firing.
	ErrorCount int `json:"errorCount"`
}

// RollingUpgradeConfig defines the desired config of the RollingUpgrade
type RollingUpgradeConfig struct {
	// FailureThreshold controls how many failures the cluster can tolerate during a rolling upgrade. Once the number of
	// failures reaches this threshold a rolling upgrade flow stops. The number of failures is computed as the sum of
	// distinct broker replicas with either offline replicas or out of sync replicas and the number of alerts triggered by
	// alerts with 'rollingupgrade'
	FailureThreshold int `json:"failureThreshold"`

	// ConcurrentBrokerRestartCountPerRack controls how many brokers can be restarted in parallel during a rolling upgrade. If
	// it is set to a value greater than 1, the operator will restart up to that amount of brokers in parallel, if the
	// brokers are within the same rack (as specified by "broker.rack" in broker read-only configs). Since using Kafka broker
	// racks spreads out the replicas, we know that restarting multiple brokers in the same rack will not cause more than
	// 1/Nth of the replicas of a topic-partition to be unavailable at the same time, where N is the number of racks used.
	// This is a safe way to speed up the rolling upgrade. Note that for the rack distribution explained above, Cruise Control
	// requires `com.linkedin.kafka.cruisecontrol.analyzer.goals.RackAwareDistributionGoal` to be configured. Default value is 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	ConcurrentBrokerRestartCountPerRack int `json:"concurrentBrokerRestartCountPerRack,omitempty"`
}

// DisruptionBudget defines the configuration for PodDisruptionBudget where the workload is managed by the kafka-operator
type DisruptionBudget struct {
	// If set to true, will create a podDisruptionBudget
	// +optional
	Create bool `json:"create,omitempty"`
	// The budget to set for the PDB, can either be static number or a percentage
	// +kubebuilder:validation:Pattern:="^[0-9]+$|^[0-9]{1,2}%$|^100%$"
	Budget string `json:"budget,omitempty"`
}

// DisruptionBudgetWithStrategy defines the configuration for PodDisruptionBudget where the workload is managed by an external controller (eg. Deployments)
type DisruptionBudgetWithStrategy struct {
	// PodDisruptionBudget default settings
	DisruptionBudget `json:",inline"`
	// The strategy to be used, either minAvailable or maxUnavailable
	// +kubebuilder:validation:Enum=minAvailable;maxUnavailable
	Stategy string `json:"strategy,omitempty"`
}

// Broker defines the broker basic configuration
type Broker struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:ExclusiveMaximum=true
	Id                int32         `json:"id"`
	BrokerConfigGroup string        `json:"brokerConfigGroup,omitempty"`
	ReadOnlyConfig    string        `json:"readOnlyConfig,omitempty"`
	BrokerConfig      *BrokerConfig `json:"brokerConfig,omitempty"`
}

// BrokerConfig defines the broker configuration
type BrokerConfig struct {
	Image                string                        `json:"image,omitempty"`
	MetricsReporterImage string                        `json:"metricsReporterImage,omitempty"`
	Config               string                        `json:"config,omitempty"`
	StorageConfigs       []StorageConfig               `json:"storageConfigs,omitempty"`
	ServiceAccountName   string                        `json:"serviceAccountName,omitempty"`
	Resources            *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ImagePullSecrets     []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector         map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations          []corev1.Toleration           `json:"tolerations,omitempty"`
	KafkaHeapOpts        string                        `json:"kafkaHeapOpts,omitempty"`
	KafkaJVMPerfOpts     string                        `json:"kafkaJvmPerfOpts,omitempty"`
	// Override for the default log4j configuration
	Log4jConfig string `json:"log4jConfig,omitempty"`
	// Custom annotations for the broker pods - e.g.: Prometheus scraping annotations:
	// prometheus.io/scrape: "true"
	// prometheus.io/port: "9020"
	BrokerAnnotations map[string]string `json:"brokerAnnotations,omitempty"`
	// Custom labels for the broker pods, example use case: for Prometheus monitoring to capture the group for each broker as a label, e.g.:
	// kafka_broker_group: "default_group"
	// these labels will not override the reserved labels that the operator relies on, for example, "app", "brokerId", and "kafka_cr"
	// +optional
	BrokerLabels map[string]string `json:"brokerLabels,omitempty"`
	// Network throughput information in kB/s used by Cruise Control to determine broker network capacity.
	// By default it is set to `125000` which means 1Gbit/s in network throughput.
	NetworkConfig *NetworkConfig `json:"networkConfig,omitempty"`
	// External listeners that use NodePort type service to expose the broker outside the Kubernetes clusterT and their
	// external IP to advertise Kafka broker external listener. The external IP value is ignored in case of external listeners that use LoadBalancer
	// type service to expose the broker outside the Kubernetes cluster. Also, when "hostnameOverride" field of the external listener is set
	// it will override the broker's external listener advertise address according to the description of the "hostnameOverride" field.
	NodePortExternalIP map[string]string `json:"nodePortExternalIP,omitempty"`
	// When "hostNameOverride" and brokerConfig.nodePortExternalIP are empty and NodePort access method is selected for an external listener
	// the NodePortNodeAdddressType defines the Kafka broker's Kubernetes node's address type that shall be used in the advertised.listeners property.
	// https://kubernetes.io/docs/concepts/architecture/nodes/#addresses
	// The NodePortNodeAddressType's possible values can be Hostname, ExternalIP, InternalIP, InternalDNS,ExternalDNS
	// +kubebuilder:validation:Enum=Hostname;ExternalIP;InternalIP;InternalDNS;ExternalDNS
	// +optional
	NodePortNodeAddressType corev1.NodeAddressType `json:"nodePortNodeAddressType,omitempty"`
	// Any definition received through this field will override the default behaviour of OneBrokerPerNode flag
	// and the operator supposes that the user is aware of how scheduling is done by kubernetes
	// Affinity could be set through brokerConfigGroups definitions and can be set for individual brokers as well
	// where letter setting will override the group setting
	Affinity           *corev1.Affinity           `json:"affinity,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// SecurityContext allows to set security context for the kafka container
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
	// BrokerIngressMapping allows to set specific ingress to a specific broker mappings.
	// If left empty, all broker will inherit the default one specified under external listeners config
	// Only used when ExternalListeners.Config is populated
	BrokerIngressMapping []string `json:"brokerIngressMapping,omitempty"`
	// InitContainers add extra initContainers to the Kafka broker pod
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	// Containers add extra Containers to the Kafka broker pod
	Containers []corev1.Container `json:"containers,omitempty"`
	// Volumes define some extra Kubernetes Volumes for the Kafka broker Pods.
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// VolumeMounts define some extra Kubernetes VolumeMounts for the Kafka broker Pods.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// Envs defines environment variables for Kafka broker Pods.
	// Adding the "+" prefix to the name prepends the value to that environment variable instead of overwriting it.
	// Add the "+" suffix to append.
	Envs []corev1.EnvVar `json:"envs,omitempty"`
	// TerminationGracePeriod defines the pod termination grace period
	// +kubebuilder:default=120
	// +optional
	TerminationGracePeriod *int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// PriorityClassName specifies the priority class name for a broker pod(s).
	// If specified, the PriorityClass resource with this PriorityClassName must be created beforehand.
	// If not specified, the broker pods' priority is default to zero.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
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
	CruiseControlTaskSpec      CruiseControlTaskSpec         `json:"cruiseControlTaskSpec,omitempty"`
	CruiseControlOperationSpec *CruiseControlOperationSpec   `json:"cruiseControlOperationSpec,omitempty"`
	CruiseControlEndpoint      string                        `json:"cruiseControlEndpoint,omitempty"`
	Resources                  *corev1.ResourceRequirements  `json:"resourceRequirements,omitempty"`
	ServiceAccountName         string                        `json:"serviceAccountName,omitempty"`
	ImagePullSecrets           []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector               map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations                []corev1.Toleration           `json:"tolerations,omitempty"`
	Config                     string                        `json:"config,omitempty"`
	CapacityConfig             string                        `json:"capacityConfig,omitempty"`
	ClusterConfig              string                        `json:"clusterConfig,omitempty"`
	Log4jConfig                string                        `json:"log4jConfig,omitempty"`
	Image                      string                        `json:"image,omitempty"`
	TopicConfig                *TopicConfig                  `json:"topicConfig,omitempty"`
	Affinity                   *corev1.Affinity              `json:"affinity,omitempty"`
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
	// PriorityClassName specifies the priority class name for the CruiseControl pod.
	// If specified, the PriorityClass resource with this PriorityClassName must be created beforehand.
	// If not specified, the CruiseControl pod's priority is default to zero.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// CruiseControlOperationSpec specifies the configuration of the CruiseControlOperation handling
type CruiseControlOperationSpec struct {
	// When TTLSecondsAfterFinished is specified, the created and finished (completed successfully or completedWithError and errorPolicy: ignore)
	// cruiseControlOperation custom resource will be deleted after the given time elapsed.
	// When it is 0 then the resource is going to be deleted instantly after the operation is finished.
	// When it is not specified the resource is not going to be removed.
	// Value can be only zero and positive integers.
	// +kubebuilder:validation:Minimum=0
	TTLSecondsAfterFinished *int `json:"ttlSecondsAfterFinished,omitempty"`
}

// GetTTLSecondsAfterFinished returns NIL when CruiseControlOperationSpec is not specified otherwise it returns itself
func (c *CruiseControlOperationSpec) GetTTLSecondsAfterFinished() *int {
	if c == nil {
		return nil
	}
	return c.TTLSecondsAfterFinished
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
	Replicas int32 `json:"replicas,omitempty"`
	// ServiceAccountName is the name of service account
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// ImagePullSecrets for the envoy image pull
	ImagePullSecrets          []corev1.LocalObjectReference     `json:"imagePullSecrets,omitempty"`
	Affinity                  *corev1.Affinity                  `json:"affinity,omitempty"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// NodeSelector is the node selector expression for envoy pods
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	// Annotations defines the annotations placed on the envoy ingress controller deployment
	Annotations map[string]string `json:"annotations,omitempty"`
	// If specified and supported by the platform, traffic through the
	// cloud-provider load-balancer will be restricted to the specified client
	// IPs. This field will be ignored if the
	// cloud-provider does not support the feature.
	// More info: https://kubernetes.io/docs/tasks/access-application-cluster/configure-cloud-provider-firewall/
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
	// LoadBalancerIP can be used to specify an exact IP for the LoadBalancer service
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`
	// Envoy admin port
	AdminPort *int32 `json:"adminPort,omitempty"`
	// Envoy health-check port
	HealthCheckPort *int32 `json:"healthCheckPort,omitempty"`
	// DisruptionBudget is the pod disruption budget attached to Envoy Deployment(s)
	DisruptionBudget *DisruptionBudgetWithStrategy `json:"disruptionBudget,omitempty"`
	// Envoy command line arguments
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	CommandLineArgs *EnvoyCommandLineArgs `json:"envoyCommandLineArgs,omitempty"`
	// PriorityClassName specifies the priority class name for the Envoy pod(s)
	// If specified, the PriorityClass resource with this PriorityClassName must be created beforehand
	// If not specified, the Envoy pods' priority is default to zero
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// EnableHealthCheckHttp10 is a toggle for adding HTTP1.0 support to Envoy health-check, default false
	// +optional
	EnableHealthCheckHttp10 bool `json:"enableHealthCheckHttp10,omitempty"`

	// PodSecurityContext holds pod-level security attributes and common container
	// settings for the Envoy pods.
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

// EnvoyCommandLineArgs defines envoy command line arguments
type EnvoyCommandLineArgs struct {
	// Envoy --concurrency command line argument.
	// See https://www.envoyproxy.io/docs/envoy/latest/operations/cli#cmdoption-concurrency
	// +optional
	// +kubebuilder:validation:Minimum=1
	Concurrency int32 `json:"concurrency,omitempty"`
}

// IstioIngressConfig defines the config for the Istio Ingress Controller
type IstioIngressConfig struct {
	Resources *corev1.ResourceRequirements `json:"resourceRequirements,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Replicas     int32                `json:"replicas,omitempty"`
	NodeSelector map[string]string    `json:"nodeSelector,omitempty"`
	Tolerations  []*corev1.Toleration `json:"tolerations,omitempty"`
	// Annotations defines the annotations placed on the istio ingress controller deployment
	Annotations               map[string]string   `json:"annotations,omitempty"`
	TLSOptions                *v1beta1.TLSOptions `json:"gatewayConfig,omitempty"`
	VirtualServiceAnnotations map[string]string   `json:"virtualServiceAnnotations,omitempty"`
	// Envs allows to add additional env vars to the istio meshgateway resource
	Envs []*corev1.EnvVar `json:"envs,omitempty"`
	// If specified and supported by the platform, traffic through the
	// cloud-provider load-balancer will be restricted to the specified client
	// IPs. This field will be ignored if the
	// cloud-provider does not support the feature."
	// More info: https://kubernetes.io/docs/tasks/access-application-cluster/configure-cloud-provider-firewall/
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
}

func (iIConfig *IstioIngressConfig) GetAnnotations() map[string]string {
	return util.CloneMap(iIConfig.Annotations)
}

// GetVirtualServiceAnnotations returns a copy of the VirtualServiceAnnotations field
func (iIConfig *IstioIngressConfig) GetVirtualServiceAnnotations() map[string]string {
	return util.CloneMap(iIConfig.VirtualServiceAnnotations)
}

// GetLoadBalancerSourceRanges returns LoadBalancerSourceRanges to use for Istio Meshagetway generated LoadBalancer
func (iIConfig *IstioIngressConfig) GetLoadBalancerSourceRanges() []string {
	return iIConfig.LoadBalancerSourceRanges
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
	MountPath string `json:"mountPath"`

	// If set https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim is used
	// as storage for Kafka broker log dirs. Either `pvcSpec` or `emptyDir` has to be set.
	// When both `pvcSpec` and `emptyDir` fields are set
	// the `pvcSpec` is used by default.
	// +optional
	PvcSpec *corev1.PersistentVolumeClaimSpec `json:"pvcSpec,omitempty"`

	// If set https://kubernetes.io/docs/concepts/storage/volumes#emptydir is used
	// as storage for Kafka broker log dirs. The use of empty dir as Kafka broker storage is useful in development
	// environments where data loss is not a concern as data stored on emptydir backed storage is lost at pod restarts.
	// Either `pvcSpec` or `emptyDir` has to be set.
	// When both `pvcSpec` and `emptyDir` fields are set
	// the `pvcSpec` is used by default.
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
}

// ListenersConfig defines the Kafka listener types
type ListenersConfig struct {
	ExternalListeners  []ExternalListenerConfig `json:"externalListeners,omitempty"`
	InternalListeners  []InternalListenerConfig `json:"internalListeners"`
	SSLSecrets         *SSLSecrets              `json:"sslSecrets,omitempty"`
	ServiceAnnotations map[string]string        `json:"serviceAnnotations,omitempty"`
}

// GetServiceAnnotations returns a copy of the ServiceAnnotations field.
func (c ListenersConfig) GetServiceAnnotations() map[string]string {
	return util.CloneMap(c.ServiceAnnotations)
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

// GetIngressControllerTargetPort returns the IngressControllerTargetPort if it is defined,
// otherwise it returns the DefaultIngressControllerTargetPort value
func (c ExternalListenerConfig) GetIngressControllerTargetPort() int32 {
	if c.IngressControllerTargetPort == nil {
		return defaultIngressControllerTargetPort
	}
	return *c.IngressControllerTargetPort
}

// GetServiceAnnotations returns a copy of the ServiceAnnotations field.
func (c IngressServiceSettings) GetServiceAnnotations() map[string]string {
	return util.CloneMap(c.ServiceAnnotations)
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
	JKSPasswordName string                  `json:"jksPasswordName,omitempty"`
	Create          bool                    `json:"create,omitempty"`
	IssuerRef       *cmmeta.ObjectReference `json:"issuerRef,omitempty"`
	// +kubebuilder:validation:Enum={"cert-manager"}
	PKIBackend PKIBackend `json:"pkiBackend,omitempty"`
}

// TODO (tinyzimmer): The above are all optional now in one way or another.
// Would be another good use-case for a pre-admission hook
// E.g. TLSSecretName and JKSPasswordName are only required if Create is false
// Or heck, do we even want to bother supporting an imported PKI?

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
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// externalStartingPort is added to each broker ID to get the port number that will be used for external access to the broker.
	// The choice of broker ID and externalStartingPort must satisfy 0 < broker ID + externalStartingPort <= 65535
	// If accessMethod is Nodeport and externalStartingPort is set to 0 then the broker IDs are not added and the Nodeport port numbers will be chosen automatically by the K8s Service controller
	ExternalStartingPort int32 `json:"externalStartingPort"`
	// configuring AnyCastPort allows kafka cluster access without specifying the exact broker
	// If not defined, 29092 will be used for external clients to reach the kafka cluster
	AnyCastPort *int32 `json:"anyCastPort,omitempty"`
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	// IngressControllerTargetPort defines the container port that the ingress controller uses for handling external traffic.
	// If not defined, 29092 will be used as the default IngressControllerTargetPort value.
	IngressControllerTargetPort *int32 `json:"ingressControllerTargetPort,omitempty"`
	// +kubebuilder:validation:Enum=LoadBalancer;NodePort
	// accessMethod defines the method which the external listener is exposed through.
	// Two types are supported LoadBalancer and NodePort.
	// The recommended and default is the LoadBalancer.
	// NodePort should be used in Kubernetes environments with no support for provisioning Load Balancers.
	// +optional
	AccessMethod corev1.ServiceType `json:"accessMethod,omitempty"`
	// Config allows to specify ingress controller configuration per external listener
	// if set, it overrides the default `KafkaClusterSpec.IstioIngressConfig` or `KafkaClusterSpec.EnvoyConfig` for this external listener.
	// +optional
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
	// +kubebuilder:validation:Enum=ssl;plaintext;sasl_ssl;sasl_plaintext
	Type SecurityProtocol `json:"type"`
	// ServerSSLCertSecret is a reference to the Kubernetes secret that contains the server certificate for the listener to be used for SSL communication.
	// The secret must contain the keystore, truststore jks files and the password for them in base64 encoded format under the keystore.jks, truststore.jks, password data fields.
	// If this field is omitted koperator will auto-create a self-signed server certificate using the configuration provided in 'sslSecrets' field.
	ServerSSLCertSecret *corev1.LocalObjectReference `json:"serverSSLCertSecret,omitempty"`
	// SSLClientAuth specifies whether client authentication is required, requested, or not required.
	// This field defaults to "required" if it is omitted
	// +kubebuilder:validation:Enum=required;requested;none
	SSLClientAuth SSLClientAuthentication `json:"sslClientAuth,omitempty"`
	// +kubebuilder:validation:Pattern=^[a-z0-9\-]+
	Name string `json:"name"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:ExclusiveMinimum=true
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32 `json:"containerPort"`
}

func (c *CommonListenerSpec) GetServerSSLCertSecretName() string {
	if c.ServerSSLCertSecret == nil {
		return ""
	}
	return c.ServerSSLCertSecret.Name
}

// ListenerStatuses holds information about the statuses of the configured listeners.
// The internal and external listeners are stored in separate maps, and each listener can be looked up by name.
type ListenerStatuses struct {
	InternalListeners map[string]ListenerStatusList `json:"internalListeners,omitempty"`
	ExternalListeners map[string]ListenerStatusList `json:"externalListeners,omitempty"`
}

// ListenerStatusList can hold various amount of statuses based on the listener configuration.
type ListenerStatusList []ListenerStatus

func (l ListenerStatusList) Len() int {
	return len(l)
}

func (l ListenerStatusList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

func (l ListenerStatusList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

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
// +kubebuilder:webhook:verbs=create;update,path=/validate-kafka-banzaicloud-io-v1beta1-kafkacluster,mutating=false,failurePolicy=fail,groups=kafka.banzaicloud.io,resources=kafkaclusters,versions=v1beta1,name=kafkaclusters.kafka.banzaicloud.io,sideEffects=None,admissionReviewVersions=v1

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
			"cpu":    resource.MustParse(defaultIstioIngressRequestResourceCpu),
			"memory": resource.MustParse(defaultIstioIngressRequestResourceMemory),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultIstioIngressLimitResourceCpu),
			"memory": resource.MustParse(defaultIstioIngressLimitResourceMemory),
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
		return defaultReplicas
	}
	return iIConfig.Replicas
}

// GetClientSSLCertSecretName returns the ClientSSLCertSecretName. It returns empty string if It's not specified
func (k *KafkaClusterSpec) GetClientSSLCertSecretName() string {
	if k.ClientSSLCertSecret == nil {
		return ""
	}
	return k.ClientSSLCertSecret.Name
}

// IsClientSSLSecretPresent returns true if ssl client certifications have been set for the operator and cruise control.
func (k *KafkaClusterSpec) IsClientSSLSecretPresent() bool {
	return k.ListenersConfig.SSLSecrets != nil || k.GetClientSSLCertSecretName() != ""
}

// GetIngressController returns the default Envoy ingress controller if not specified otherwise
func (kSpec *KafkaClusterSpec) GetIngressController() string {
	if kSpec.IngressController == "" {
		return defaultKafkaClusterIngressController
	}
	return kSpec.IngressController
}

// GetKubernetesClusterDomain returns the default domain if not specified otherwise
func (kSpec *KafkaClusterSpec) GetKubernetesClusterDomain() string {
	if kSpec.KubernetesClusterDomain == "" {
		return defaultKafkaClusterK8sClusterDomain
	}
	return kSpec.KubernetesClusterDomain
}

// GetZkPath returns the default "/" ZkPath if not specified otherwise
func (kSpec *KafkaClusterSpec) GetZkPath() string {
	const prefix = "/"
	if kSpec.ZKPath == "" {
		return prefix
	}

	if !strings.HasPrefix(kSpec.ZKPath, prefix) {
		return prefix + kSpec.ZKPath
	}

	return kSpec.ZKPath
}

// GetClusterImage returns the default container image for Kafka Cluster
func (kSpec *KafkaClusterSpec) GetClusterImage() string {
	if kSpec.ClusterImage != "" {
		return kSpec.ClusterImage
	}
	return defaultKafkaImage
}

// GetClusterMetricsReporterImage returns the default container image for Kafka Cluster
func (kSpec *KafkaClusterSpec) GetClusterMetricsReporterImage() string {
	if kSpec.ClusterMetricsReporterImage != "" {
		return kSpec.ClusterMetricsReporterImage
	}
	return kSpec.CruiseControlConfig.GetCCImage()
}

func (cTaskSpec *CruiseControlTaskSpec) GetDurationMinutes() float64 {
	if cTaskSpec.RetryDurationMinutes == 0 {
		return defaultCruiseControlTaskDurationMin
	}
	return float64(cTaskSpec.RetryDurationMinutes)
}

// GetLoadBalancerSourceRanges returns LoadBalancerSourceRanges to use for Envoy generated LoadBalancer
func (eConfig *EnvoyConfig) GetLoadBalancerSourceRanges() []string {
	return eConfig.LoadBalancerSourceRanges
}

// GetAnnotations returns Annotations to use for Envoy generated Deployment and Pods
func (eConfig *EnvoyConfig) GetAnnotations() map[string]string {
	return util.CloneMap(eConfig.Annotations)
}

// GetDistruptionBudget returns DisruptionBudget to use for Envoy generated Pods
func (eConfig *EnvoyConfig) GetDistruptionBudget() DisruptionBudgetWithStrategy {
	if eConfig.DisruptionBudget != nil {
		return *eConfig.DisruptionBudget
	}
	return DisruptionBudgetWithStrategy{}
}

// GetReplicas returns replicas used by the Envoy deployment
func (eConfig *EnvoyConfig) GetReplicas() int32 {
	if eConfig.Replicas == 0 {
		return defaultEnvoyReplicas
	}
	return eConfig.Replicas
}

// GetServiceAccount returns the Kubernetes Service Account to use for Kafka Cluster
func (bConfig *BrokerConfig) GetServiceAccount() string {
	if bConfig.ServiceAccountName != "" {
		return bConfig.ServiceAccountName
	}
	return defaultServiceAccountName
}

// GetServiceAccount returns the Kubernetes Service Account to use for EnvoyConfig
func (eConfig *EnvoyConfig) GetServiceAccount() string {
	if eConfig.ServiceAccountName != "" {
		return eConfig.ServiceAccountName
	}
	return defaultServiceAccountName
}

// GetServiceAccount returns the Kubernetes Service Account to use for CruiseControl
func (cConfig *CruiseControlConfig) GetServiceAccount() string {
	if cConfig.ServiceAccountName != "" {
		return cConfig.ServiceAccountName
	}
	return defaultServiceAccountName
}

// GetTolerations returns the tolerations for the given broker
func (bConfig *BrokerConfig) GetTolerations() []corev1.Toleration {
	return bConfig.Tolerations
}

// GetTolerations returns the tolerations for envoy
func (eConfig *EnvoyConfig) GetTolerations() []corev1.Toleration {
	return eConfig.Tolerations
}

// GetTolerations returns the tolerations for cruise control
func (cConfig *CruiseControlConfig) GetTolerations() []corev1.Toleration {
	return cConfig.Tolerations
}

// GetTerminationGracePeriod returns the termination grace period for the broker pod
func (bConfig *BrokerConfig) GetTerminationGracePeriod() int64 {
	if bConfig.TerminationGracePeriod == nil {
		return defaultBrokerTerminationGracePeriod
	}
	return *bConfig.TerminationGracePeriod
}

// GetNodeSelector returns the node selector for cruise control
func (cConfig *CruiseControlConfig) GetNodeSelector() map[string]string {
	return cConfig.NodeSelector
}

// GetAffinity returns the Affinity config for cruise control
func (cConfig *CruiseControlConfig) GetAffinity() *corev1.Affinity {
	return cConfig.Affinity
}

// GetNodeSelector returns the node selector for envoy
func (eConfig *EnvoyConfig) GetNodeSelector() map[string]string {
	return eConfig.NodeSelector
}

// GetAffinity returns the Affinity config for envoy
func (eConfig *EnvoyConfig) GetAffinity() *corev1.Affinity {
	return eConfig.Affinity
}

// GetTopologySpreadConstaints returns the Affinity config for envoy
func (eConfig *EnvoyConfig) GetTopologySpreadConstaints() []corev1.TopologySpreadConstraint {
	return eConfig.TopologySpreadConstraints
}

// GetPodSecurityContext returns the security context for the envoy deployment podspec.
func (eConfig *EnvoyConfig) GetPodSecurityContext() *corev1.PodSecurityContext {
	if eConfig == nil {
		return nil
	}

	return eConfig.PodSecurityContext
}

// GetPriorityClassName returns the priority class name for envoy
func (eConfig *EnvoyConfig) GetPriorityClassName() string {
	return eConfig.PriorityClassName
}

// GetNodeSelector returns the node selector for the given broker
func (bConfig *BrokerConfig) GetNodeSelector() map[string]string {
	return bConfig.NodeSelector
}

// GetPriorityClassName returns the priority class name for the given broker
func (bConfig *BrokerConfig) GetPriorityClassName() string {
	return bConfig.PriorityClassName
}

// GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (bConfig *BrokerConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return bConfig.ImagePullSecrets
}

// GetBrokerAnnotations returns the annotations that are applied to broker pods
func (bConfig *BrokerConfig) GetBrokerAnnotations() map[string]string {
	return util.CloneMap(bConfig.BrokerAnnotations)
}

// GetBrokerLabels returns the labels that are applied to broker pods
func (bConfig *BrokerConfig) GetBrokerLabels(kafkaClusterName string, brokerId int32) map[string]string {
	return util.MergeLabels(
		bConfig.BrokerLabels,
		util.LabelsForKafka(kafkaClusterName),
		map[string]string{BrokerIdLabelKey: fmt.Sprintf("%d", brokerId)},
	)
}

// GetCruiseControlAnnotations return the annotations which applied to CruiseControl pod
func (cConfig *CruiseControlConfig) GetCruiseControlAnnotations() map[string]string {
	return util.CloneMap(cConfig.CruiseControlAnnotations)
}

// GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (eConfig *EnvoyConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return eConfig.ImagePullSecrets
}

// GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories
func (cConfig *CruiseControlConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return cConfig.ImagePullSecrets
}

// GetPriorityClassName returns the priority class name for the CruiseControl pod
func (cConfig *CruiseControlConfig) GetPriorityClassName() string {
	return cConfig.PriorityClassName
}

// GetResources returns the envoy specific Kubernetes resource
func (eConfig *EnvoyConfig) GetResources() *corev1.ResourceRequirements {
	if eConfig.Resources != nil {
		return eConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultEnvoyRequestResourceCpu),
			"memory": resource.MustParse(defaultEnvoyRequestResourceMemory),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultEnvoyLimitResourceCpu),
			"memory": resource.MustParse(defaultEnvoyLimitResourceMemory),
		},
	}
}

// GetConcurrency returns envoy concurrency.
// Defines the number of worker threads envoy pod should run.
// If not specified defaults to the number of hardware threads on the underlying kubernetes node.
// See https://www.envoyproxy.io/docs/envoy/latest/operations/cli#cmdoption-concurrency
func (eConfig *EnvoyConfig) GetConcurrency() int32 {
	if eConfig.CommandLineArgs == nil {
		return defaultEnvoyConcurrency
	}
	return eConfig.CommandLineArgs.Concurrency
}

// GetResources returns the CC specific Kubernetes resource
func (cConfig *CruiseControlConfig) GetResources() *corev1.ResourceRequirements {
	if cConfig.Resources != nil {
		return cConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultCruiseControlRequestResourceCpu),
			"memory": resource.MustParse(defaultCruiseControlRequestResourceMemory),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultCruiseControlLimitResourceCpu),
			"memory": resource.MustParse(defaultCruiseControlLimitResourceMemory),
		},
	}
}

// GetResources returns the broker specific Kubernetes resource
func (bConfig *BrokerConfig) GetResources() *corev1.ResourceRequirements {
	if bConfig.Resources != nil {
		return bConfig.Resources
	}
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultBrokerRequestResourceCpu),
			"memory": resource.MustParse(defaultBrokerRequestResourceMemory),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultBrokerLimitResourceCpu),
			"memory": resource.MustParse(defaultBrokerLimitResourceMemory),
		},
	}
}

// GetKafkaHeapOpts returns the broker specific Heap settings
func (bConfig *BrokerConfig) GetKafkaHeapOpts() string {
	if bConfig.KafkaHeapOpts != "" {
		return bConfig.KafkaHeapOpts
	}

	return defaultBrokerHeapOpts
}

// GetKafkaPerfJvmOpts returns the broker specific Perf JVM settings
func (bConfig *BrokerConfig) GetKafkaPerfJvmOpts() string {
	if bConfig.KafkaJVMPerfOpts != "" {
		return bConfig.KafkaJVMPerfOpts
	}

	return defaultBrokerPerfJvmOpts
}

// GetEnvoyImage returns the used envoy image
func (eConfig *EnvoyConfig) GetEnvoyImage() string {
	if eConfig.Image != "" {
		return eConfig.Image
	}

	return defaultEnvoyImage
}

// GetEnvoyAdminPort returns the envoy admin port
func (eConfig *EnvoyConfig) GetEnvoyAdminPort() int32 {
	if eConfig.AdminPort != nil {
		return *eConfig.AdminPort
	}
	return defaultEnvoyAdminPort
}

// GetEnvoyHealthCheckPort returns the envoy admin port
func (eConfig *EnvoyConfig) GetEnvoyHealthCheckPort() int32 {
	if eConfig.HealthCheckPort != nil {
		return *eConfig.HealthCheckPort
	}
	return defaultEnvoyHealthCheckPort
}

// GetCCImage returns the used Cruise Control image
func (cConfig *CruiseControlConfig) GetCCImage() string {
	if cConfig.Image != "" {
		return cConfig.Image
	}
	return defaultCruiseControlImage
}

// GetCCLog4jConfig returns the used Cruise Control log4j configuration
func (cConfig *CruiseControlConfig) GetCCLog4jConfig() string {
	if cConfig.Log4jConfig != "" {
		return cConfig.Log4jConfig
	}
	return assets.CruiseControlLog4jProperties
}

// GetImage returns the used image for Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetImage() string {
	if mConfig.JmxImage != "" {
		return mConfig.JmxImage
	}
	return defaultMonitorImage
}

// GetPathToJar returns the path in the used Image for Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetPathToJar() string {
	if mConfig.PathToJar != "" {
		return mConfig.PathToJar
	}
	return defaultMonitorPathToJar
}

// GetKafkaJMXExporterConfig returns the config for Kafka Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetKafkaJMXExporterConfig() string {
	if mConfig.KafkaJMXExporterConfig != "" {
		return mConfig.KafkaJMXExporterConfig
	}
	// Use upstream defined rules https://github.com/prometheus/jmx_exporter/blob/master/example_configs/kafka-2_0_0.yml
	return assets.KafkaJmxExporterYaml
}

// GetCCJMXExporterConfig returns the config for CC Prometheus JMX exporter
func (mConfig *MonitoringConfig) GetCCJMXExporterConfig() string {
	if mConfig.CCJMXExporterConfig != "" {
		return mConfig.CCJMXExporterConfig
	}
	return assets.CruiseControlJmxExporterYaml
}

// GetBrokerConfig composes the brokerConfig for a given broker using the broker's config group
func (b *Broker) GetBrokerConfig(kafkaClusterSpec KafkaClusterSpec) (*BrokerConfig, error) {
	brokerConfigGroups := kafkaClusterSpec.BrokerConfigGroups

	bConfig := &BrokerConfig{}
	if b.BrokerConfigGroup == "" {
		return b.BrokerConfig, nil
	} else if b.BrokerConfig != nil {
		bConfig = b.BrokerConfig.DeepCopy()
	}

	groupConfig, exists := brokerConfigGroups[b.BrokerConfigGroup]
	if !exists {
		return nil, errors.NewWithDetails("missing brokerConfigGroup", "key", b.BrokerConfigGroup)
	}

	dstAffinity, err := mergeAffinity(groupConfig, bConfig)
	if err != nil {
		return nil, errors.WrapIf(err, "could not merge brokerConfig.Affinity with ConfigGroup.Affinity")
	}
	envs := mergeEnvs(kafkaClusterSpec, &groupConfig, bConfig)

	err = mergo.Merge(bConfig, groupConfig, mergo.WithAppendSlice)
	if err != nil {
		return nil, errors.WrapIf(err, "could not merge brokerConfig with ConfigGroup")
	}

	bConfig.StorageConfigs = dedupStorageConfigs(bConfig.StorageConfigs)
	if groupConfig.Affinity != nil || bConfig.Affinity != nil {
		bConfig.Affinity = dstAffinity
	}
	bConfig.Envs = envs

	return bConfig, nil
}

func mergeEnvs(kafkaClusterSpec KafkaClusterSpec, groupConfig, bConfig *BrokerConfig) []corev1.EnvVar {
	var envs []corev1.EnvVar
	envs = append(envs, kafkaClusterSpec.Envs...)
	envs = append(envs, groupConfig.Envs...)
	if bConfig != nil {
		envs = append(envs, bConfig.Envs...)
	}
	return envs
}

func mergeAffinity(groupConfig BrokerConfig, bConfig *BrokerConfig) (*corev1.Affinity, error) {
	dstAffinity := &corev1.Affinity{}
	srcAffinity := &corev1.Affinity{}

	if groupConfig.Affinity != nil {
		dstAffinity = groupConfig.Affinity.DeepCopy()
	}
	if bConfig.Affinity != nil {
		srcAffinity = bConfig.Affinity
	}

	if err := mergo.Merge(dstAffinity, srcAffinity, mergo.WithOverride); err != nil {
		return nil, err
	}
	return dstAffinity, nil
}

func dedupStorageConfigs(elements []StorageConfig) []StorageConfig {
	encountered := make(map[string]struct{})
	var result []StorageConfig

	for _, v := range elements {
		if _, ok := encountered[v.MountPath]; !ok {
			encountered[v.MountPath] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}
