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

import "strings"

// RackAwarenessState stores info about rack awareness status
type RackAwarenessState string

// CruiseControlState holds info about the state of Cruise Control
type CruiseControlState string

// CruiseControlTopicStatus holds info about the CC topic status
type CruiseControlTopicStatus string

// CruiseControlUserTaskState holds info about the CC user task state
type CruiseControlUserTaskState string

// ClusterState holds info about the cluster state
type ClusterState string

// ConfigurationState holds info about the configuration state
type ConfigurationState string

// SecurityProtocol is the protocol used to communicate with brokers.
// Valid values are: plaintext, ssl, sasl_plaintext, sasl_ssl.
type SecurityProtocol string

// PerBrokerConfigurationState holds info about the per-broker configuration state
type PerBrokerConfigurationState string

// ExternalListenerConfigNames type describes a collection of external listener names
type ExternalListenerConfigNames []string

// KafkaVersion type describes the kafka version and docker version
type KafkaVersion struct {
	// Version holds the current version of the broker in semver format
	Version string `json:"version,omitempty"`
	// Image specifies the current docker image of the broker
	Image string `json:"image,omitempty"`
}

// PKIBackend represents an interface implementing the PKIManager
type PKIBackend string

// CruiseControlVolumeState holds information about the state of volume rebalance
type CruiseControlVolumeState string

// IsRunningState returns true if CruiseControlVolumeState indicates (GracefulDisk*Running)
// that there is a running operation in Cruise Control related to the resource the CruiseControlVolumeState belongs to.
func (s CruiseControlVolumeState) IsRunningState() bool {
	return s == GracefulDiskRebalanceRunning
}

// IsActive returns true if CruiseControlVolumeState is in active state (GracefulDisk*Running or GracefulDisk*Required)
// the controller needs to take care of.
func (s CruiseControlVolumeState) IsActive() bool {
	return s == GracefulDiskRebalanceRunning || s == GracefulDiskRebalanceRequired
}

// IsUpscale returns true if CruiseControlState in GracefulUpscale* state.
func (r CruiseControlState) IsUpscale() bool {
	return r == GracefulUpscaleRequired || r == GracefulUpscaleSucceeded || r == GracefulUpscaleRunning
}

// IsDownscale returns true if CruiseControlState in GracefulDownscale* state.
func (r CruiseControlState) IsDownscale() bool {
	return r == GracefulDownscaleRequired || r == GracefulDownscaleSucceeded || r == GracefulDownscaleRunning
}

// IsRunningState returns true if CruiseControlState indicates (any of Graceful*Running)
// that there is a running operation in Cruise Control related to the resource the CruiseControlState belongs to.
func (r CruiseControlState) IsRunningState() bool {
	return r == GracefulDownscaleRunning || r == GracefulUpscaleRunning
}

// IsRequiredState returns true if CruiseControlVolumeState indicates that either upscaling or downscaling
// (GracefulDownscaleRequired or GracefulUpscaleRequired) operation needs to be performed.
func (r CruiseControlState) IsRequiredState() bool {
	return r == GracefulDownscaleRequired || r == GracefulUpscaleRequired
}

// IsActive returns true if CruiseControlState is in active state (Graceful*Running or Graceful*Required)
// the controller needs to take care of.
func (r CruiseControlState) IsActive() bool {
	return r.IsRunningState() || r.IsRequiredState()
}

func (r CruiseControlState) Complete() CruiseControlState {
	switch r {
	case GracefulUpscaleRequired, GracefulUpscaleRunning:
		return GracefulUpscaleSucceeded
	case GracefulDownscaleRequired, GracefulDownscaleRunning:
		return GracefulDownscaleSucceeded
	case GracefulUpscaleSucceeded, GracefulDownscaleSucceeded:
		return r
	default:
		return r
	}
}

// IsSSL determines if the receiver is using SSL
func (r SecurityProtocol) IsSSL() bool {
	return r.Equal(SecurityProtocolSaslSSL) || r.Equal(SecurityProtocolSSL)
}

// IsSasl determines if the receiver is using Sasl
func (r SecurityProtocol) IsSasl() bool {
	return r.Equal(SecurityProtocolSaslSSL) || r.Equal(SecurityProtocolSaslPlaintext)
}

// IsPlaintext determines if the receiver is using plaintext
func (r SecurityProtocol) IsPlaintext() bool {
	return r.Equal(SecurityProtocolPlaintext) || r.Equal(SecurityProtocolSaslPlaintext)
}

// ToUpperString converts SecurityProtocol to an upper string
func (r SecurityProtocol) ToUpperString() string {
	return strings.ToUpper(string(r))
}

// Equal checks the equality between two SecurityProtocols
func (r SecurityProtocol) Equal(s SecurityProtocol) bool {
	return r.ToUpperString() == s.ToUpperString()
}

const (
	// PKIBackendCertManager invokes cert-manager for user certificate management
	PKIBackendCertManager PKIBackend = "cert-manager"
	// PKIBackendProvided used to point the operator to use the PKI set in the cluster CR
	// for admin and users required for the cluster to run
	PKIBackendProvided PKIBackend = "pki-backend-provided"
	// PKIBackendK8sCSR invokes kubernetes csr API for user certificate management
	PKIBackendK8sCSR PKIBackend = "k8s-csr"
)

// IstioControlPlaneReference is a reference to the IstioControlPlane resource.
type IstioControlPlaneReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// GracefulActionState holds information about GracefulAction State
type GracefulActionState struct {
	// ErrorMessage holds the information what happened with CC
	ErrorMessage string `json:"errorMessage"`
	// CruiseControlTaskId holds info about the task id ran by CC
	CruiseControlTaskId string `json:"cruiseControlTaskId,omitempty"`
	// TaskStarted hold the time when the execution started
	TaskStarted string `json:"TaskStarted,omitempty"`
	// CruiseControlState holds the information about CC state
	CruiseControlState CruiseControlState `json:"cruiseControlState"`
	// VolumeStates holds the information about the CC disk rebalance states and tasks
	VolumeStates map[string]VolumeState `json:"volumeStates,omitempty"`
}

type VolumeState struct {
	// ErrorMessage holds the information what happened with CC disk rebalance
	ErrorMessage string `json:"errorMessage"`
	// CruiseControlTaskId holds info about the task id ran by CC
	CruiseControlTaskId string `json:"cruiseControlTaskId,omitempty"`
	// TaskStarted hold the time when the execution started
	TaskStarted string `json:"TaskStarted,omitempty"`
	// CruiseControlVolumeState holds the information about the CC disk rebalance state
	CruiseControlVolumeState CruiseControlVolumeState `json:"cruiseControlVolumeState"`
}

// BrokerState holds information about broker state
type BrokerState struct {
	// RackAwarenessState holds info about rack awareness status
	RackAwarenessState RackAwarenessState `json:"rackAwarenessState"`
	// GracefulActionState holds info about cc action status
	GracefulActionState GracefulActionState `json:"gracefulActionState"`
	// ConfigurationState holds info about the config
	ConfigurationState ConfigurationState `json:"configurationState"`
	// PerBrokerConfigurationState holds info about the per-broker (dynamically updatable) config
	PerBrokerConfigurationState PerBrokerConfigurationState `json:"perBrokerConfigurationState"`
	// ExternalListenerConfigNames holds info about what listener config is in use with the broker
	ExternalListenerConfigNames ExternalListenerConfigNames `json:"externalListenerConfigNames,omitempty"`
	// Version holds the current version of the broker in semver format
	Version string `json:"version,omitempty"`
	// Image specifies the current docker image of the broker
	Image string `json:"image,omitempty"`
}

const (
	// Configured states the broker is running
	Configured RackAwarenessState = "Configured"
	// WaitingForRackAwareness states the broker is waiting for the rack awareness config
	WaitingForRackAwareness RackAwarenessState = "WaitingForRackAwareness"

	// Upscale cruise control states
	// GracefulUpscaleRequired states that a broker upscale is required
	GracefulUpscaleRequired CruiseControlState = "GracefulUpscaleRequired"
	// GracefulUpscaleRunning states that the broker upscale task is still running in CC
	GracefulUpscaleRunning CruiseControlState = "GracefulUpscaleRunning"
	// GracefulUpscaleSucceeded states the broker is updated gracefully OR
	// states that the broker is part of the initial cluster creation where CC topic is still in creating stage
	GracefulUpscaleSucceeded CruiseControlState = "GracefulUpscaleSucceeded"

	// Downscale cruise control states
	// GracefulDownscaleRequired states that a broker downscale is required
	GracefulDownscaleRequired CruiseControlState = "GracefulDownscaleRequired"
	// GracefulDownscaleRunning states that the broker downscale is still running in CC
	GracefulDownscaleRunning CruiseControlState = "GracefulDownscaleRunning"
	// GracefulDownscaleSucceeded states that the broker downscaled gracefully
	GracefulDownscaleSucceeded CruiseControlState = "GracefulDownscaleSucceeded"

	// Disk rebalance cruise control states
	// GracefulDiskRebalanceRequired states that the broker volume needs a CC disk rebalance
	GracefulDiskRebalanceRequired CruiseControlVolumeState = "GracefulDiskRebalanceRequired"
	// GracefulDiskRebalanceRunning states that for the broker volume a CC disk rebalance is in progress
	GracefulDiskRebalanceRunning CruiseControlVolumeState = "GracefulDiskRebalanceRunning"
	// GracefulDiskRebalanceSucceeded states that the for the broker volume rebalance has succeeded
	GracefulDiskRebalanceSucceeded CruiseControlVolumeState = "GracefulDiskRebalanceSucceeded"

	// CruiseControlTopicNotReady states the CC required topic is not yet created
	CruiseControlTopicNotReady CruiseControlTopicStatus = "CruiseControlTopicNotReady"
	// CruiseControlTopicReady states the CC required topic is created
	CruiseControlTopicReady CruiseControlTopicStatus = "CruiseControlTopicReady"
	// CruiseControlTaskActive states the CC task is scheduled but not yet running
	CruiseControlTaskActive CruiseControlUserTaskState = "Active"
	// CruiseControlTaskInExecution states the CC task is executing
	CruiseControlTaskInExecution CruiseControlUserTaskState = "InExecution"
	// CruiseControlTaskCompleted states the CC task completed successfully
	CruiseControlTaskCompleted CruiseControlUserTaskState = "Completed"
	// CruiseControlTaskCompletedWithError states the CC task completed with error
	CruiseControlTaskCompletedWithError CruiseControlUserTaskState = "CompletedWithError"
	// KafkaClusterReconciling states that the cluster is still in reconciling stage
	KafkaClusterReconciling ClusterState = "ClusterReconciling"
	// KafkaClusterRollingUpgrading states that the cluster is rolling upgrading
	KafkaClusterRollingUpgrading ClusterState = "ClusterRollingUpgrading"
	// KafkaClusterRunning states that the cluster is in running state
	KafkaClusterRunning ClusterState = "ClusterRunning"

	// ConfigInSync states that the generated brokerConfig is in sync with the Broker
	ConfigInSync ConfigurationState = "ConfigInSync"
	// ConfigOutOfSync states that the generated brokerConfig is out of sync with the Broker
	ConfigOutOfSync ConfigurationState = "ConfigOutOfSync"
	// PerBrokerConfigInSync states that the generated per-broker brokerConfig is in sync with the Broker
	PerBrokerConfigInSync PerBrokerConfigurationState = "PerBrokerConfigInSync"
	// PerBrokerConfigOutOfSync states that the generated per-broker brokerConfig is out of sync with the Broker
	PerBrokerConfigOutOfSync PerBrokerConfigurationState = "PerBrokerConfigOutOfSync"
	// PerBrokerConfigError states that the generated per-broker brokerConfig can not be set in the Broker
	PerBrokerConfigError PerBrokerConfigurationState = "PerBrokerConfigError"

	// SecurityProtocolSSL
	SecurityProtocolSSL SecurityProtocol = "ssl"
	// SecurityProtocolPlaintext
	SecurityProtocolPlaintext SecurityProtocol = "plaintext"
	// SecurityProtocolSaslSSL
	SecurityProtocolSaslSSL SecurityProtocol = "sasl_ssl"
	// SecurityProtocolSaslPlaintext
	SecurityProtocolSaslPlaintext SecurityProtocol = "sasl_plaintext"
)
