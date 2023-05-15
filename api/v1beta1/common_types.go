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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

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

// SSLClientAuthentication specifies whether client authentication is required, requested, or not required.
// Valid values are: required, requested, none
type SSLClientAuthentication string

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

// IsDiskRebalanceRunning returns true if CruiseControlVolumeState indicates
// that the CC rebalance disk operation is scheduled and in-progress
func (s CruiseControlVolumeState) IsDiskRebalanceRunning() bool {
	return s == GracefulDiskRebalanceRunning ||
		s == GracefulDiskRebalanceCompletedWithError ||
		s == GracefulDiskRebalancePaused ||
		s == GracefulDiskRebalanceScheduled
}

// IsDiskRemovalRunning returns true if CruiseControlVolumeState indicates
// that the CC remove disks operation is scheduled and in-progress
func (s CruiseControlVolumeState) IsDiskRemovalRunning() bool {
	return s == GracefulDiskRemovalRunning ||
		s == GracefulDiskRemovalCompletedWithError ||
		s == GracefulDiskRemovalPaused ||
		s == GracefulDiskRemovalScheduled
}

// IsRequiredState returns true if CruiseControlVolumeState is in GracefulDiskRebalanceRequired state or GracefulDiskRemovalRequired state
func (s CruiseControlVolumeState) IsRequiredState() bool {
	return s == GracefulDiskRebalanceRequired ||
		s == GracefulDiskRemovalRequired
}

// IsDiskRebalance returns true if CruiseControlVolumeState is in disk rebalance state
// the controller needs to take care of.
func (s CruiseControlVolumeState) IsDiskRebalance() bool {
	return s.IsDiskRebalanceRunning() || s == GracefulDiskRebalanceRequired
}

// IsDiskRemoval returns true if CruiseControlVolumeState is in disk removal state
func (s CruiseControlVolumeState) IsDiskRemoval() bool {
	return s.IsDiskRemovalRunning() || s == GracefulDiskRemovalRequired
}

// IsUpscale returns true if CruiseControlState in GracefulUpscale* state.
func (r CruiseControlState) IsUpscale() bool {
	return r == GracefulUpscaleRequired ||
		r == GracefulUpscaleSucceeded ||
		r == GracefulUpscaleRunning ||
		r == GracefulUpscaleCompletedWithError ||
		r == GracefulUpscalePaused ||
		r == GracefulUpscaleScheduled
}

// IsDownscale returns true if CruiseControlState in GracefulDownscale* state.
func (r CruiseControlState) IsDownscale() bool {
	return r == GracefulDownscaleRequired ||
		r == GracefulDownscaleSucceeded ||
		r == GracefulDownscaleRunning ||
		r == GracefulDownscaleCompletedWithError ||
		r == GracefulDownscalePaused ||
		r == GracefulDownscaleScheduled
}

// IsRunningState returns true if CruiseControlState indicates
// that the CC operation is scheduled and in-progress
func (r CruiseControlState) IsRunningState() bool {
	return r == GracefulUpscaleRunning ||
		r == GracefulUpscaleCompletedWithError ||
		r == GracefulUpscalePaused ||
		r == GracefulUpscaleScheduled ||
		r == GracefulDownscaleRunning ||
		r == GracefulDownscaleCompletedWithError ||
		r == GracefulDownscalePaused ||
		r == GracefulDownscaleScheduled
}

// IsRequiredState returns true if CruiseControlVolumeState indicates that either upscaling or downscaling
// (GracefulDownscaleRequired or GracefulUpscaleRequired) operation needs to be performed.
func (r CruiseControlState) IsRequiredState() bool {
	return r == GracefulDownscaleRequired ||
		r == GracefulUpscaleRequired
}

// IsActive returns true if CruiseControlState is in active state
// the controller needs to take care of.
func (r CruiseControlState) IsActive() bool {
	return r.IsRunningState() || r.IsRequiredState()
}

// IsSucceeded returns true if CruiseControlState is succeeded
func (r CruiseControlState) IsSucceeded() bool {
	return r == GracefulDownscaleSucceeded ||
		r == GracefulUpscaleSucceeded
}

// IsDiskRebalanceSucceeded returns true if CruiseControlVolumeState is disk rebalance succeeded
func (r CruiseControlVolumeState) IsDiskRebalanceSucceeded() bool {
	return r == GracefulDiskRebalanceSucceeded
}

// IsDiskRemovalSucceeded returns true if CruiseControlVolumeState is disk removal succeeded
func (r CruiseControlVolumeState) IsDiskRemovalSucceeded() bool {
	return r == GracefulDiskRemovalSucceeded
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
	// CruiseControlState holds the information about graceful action state
	CruiseControlState CruiseControlState `json:"cruiseControlState"`
	// CruiseControlOperationReference refers to the created CruiseControlOperation to execute a CC task
	CruiseControlOperationReference *corev1.LocalObjectReference `json:"cruiseControlOperationReference,omitempty"`
	// VolumeStates holds the information about the CC disk rebalance states and CruiseControlOperation reference
	VolumeStates map[string]VolumeState `json:"volumeStates,omitempty"`
}

type VolumeState struct {
	// CruiseControlVolumeState holds the information about CC disk rebalance state
	CruiseControlVolumeState CruiseControlVolumeState `json:"cruiseControlVolumeState"`
	// CruiseControlOperationReference refers to the created CruiseControlOperation to execute a CC task
	CruiseControlOperationReference *corev1.LocalObjectReference `json:"cruiseControlOperationReference,omitempty"`
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
	// Compressed data from broker configuration to restore broker pod in specific cases
	ConfigurationBackup string `json:"configurationBackup,omitempty"`
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
	// GracefulUpscaleScheduled states that the broker upscale CCOperation is created and the task is waiting for execution
	GracefulUpscaleScheduled CruiseControlState = "GracefulUpscaleScheduled"
	// GracefulUpscaleSucceeded states the broker is updated gracefully OR
	// states that the broker is part of the initial cluster creation where CC topic is still in creating stage
	GracefulUpscaleSucceeded CruiseControlState = "GracefulUpscaleSucceeded"
	// GracefulUpscaleCompletedWithError states that the broker upscale task completed with an error
	GracefulUpscaleCompletedWithError CruiseControlState = "GracefulUpscaleCompletedWithError"
	// GracefulUpscalePaused states that the broker upscale task is completed with an error and it will not be retried, it is paused
	GracefulUpscalePaused CruiseControlState = "GracefulUpscalePaused"
	// Downscale cruise control states
	// GracefulDownscaleRequired states that a broker downscale is required
	GracefulDownscaleRequired CruiseControlState = "GracefulDownscaleRequired"
	// GracefulDownscaleScheduled states that the broker downscale CCOperation is created and the task is waiting for execution
	GracefulDownscaleScheduled CruiseControlState = "GracefulDownscaleScheduled"
	// GracefulDownscaleRunning states that the broker downscale is still running in CC
	GracefulDownscaleRunning CruiseControlState = "GracefulDownscaleRunning"
	// GracefulDownscaleSucceeded states that the broker downscaled gracefully
	GracefulDownscaleSucceeded CruiseControlState = "GracefulDownscaleSucceeded"
	// GracefulDownscaleCompletedWithError states that the broker downscale task completed with an error
	GracefulDownscaleCompletedWithError CruiseControlState = "GracefulDownscaleCompletedWithError"
	// GracefulDownscalePaused states that the broker downscale task is completed with an error and it will not be retried, it is paused. In this case further downscale tasks can be executed
	GracefulDownscalePaused CruiseControlState = "GracefulDownscalePaused"

	// Disk removal cruise control states
	// GracefulDiskRemovalRequired states that the broker volume needs to be removed
	GracefulDiskRemovalRequired CruiseControlVolumeState = "GracefulDiskRemovalRequired"
	// GracefulDiskRemovalRunning states that for the broker volume a CC disk removal is in progress
	GracefulDiskRemovalRunning CruiseControlVolumeState = "GracefulDiskRemovalRunning"
	// GracefulDiskRemovalSucceeded states that the for the broker volume removal has succeeded
	GracefulDiskRemovalSucceeded CruiseControlVolumeState = "GracefulDiskRemovalSucceeded"
	// GracefulDiskRemovalScheduled states that the broker volume removal CCOperation is created and the task is waiting for execution
	GracefulDiskRemovalScheduled CruiseControlVolumeState = "GracefulDiskRemovalScheduled"
	// GracefulDiskRemovalCompletedWithError states that the broker volume removal task completed with an error
	GracefulDiskRemovalCompletedWithError CruiseControlVolumeState = "GracefulDiskRemovalCompletedWithError"
	// GracefulDiskRemovalPaused states that the broker volume removal task is completed with an error and it will not be retried, it is paused
	GracefulDiskRemovalPaused CruiseControlVolumeState = "GracefulDiskRemovalPaused"

	// Disk rebalance cruise control states
	// GracefulDiskRebalanceRequired states that the broker volume needs a CC disk rebalance
	GracefulDiskRebalanceRequired CruiseControlVolumeState = "GracefulDiskRebalanceRequired"
	// GracefulDiskRebalanceRunning states that for the broker volume a CC disk rebalance is in progress
	GracefulDiskRebalanceRunning CruiseControlVolumeState = "GracefulDiskRebalanceRunning"
	// GracefulDiskRebalanceSucceeded states that the for the broker volume rebalance has succeeded
	GracefulDiskRebalanceSucceeded CruiseControlVolumeState = "GracefulDiskRebalanceSucceeded"
	// GracefulDiskRebalanceScheduled states that the broker volume rebalance CCOperation is created and the task is waiting for execution
	GracefulDiskRebalanceScheduled CruiseControlVolumeState = "GracefulDiskRebalanceScheduled"
	// GracefulDiskRebalanceCompletedWithError states that the broker volume rebalance task completed with an error
	GracefulDiskRebalanceCompletedWithError CruiseControlVolumeState = "GracefulDiskRebalanceCompletedWithError"
	// GracefulDiskRebalancePaused states that the broker volume rebalance task is completed with an error and it will not be retried, it is paused
	GracefulDiskRebalancePaused CruiseControlVolumeState = "GracefulDiskRebalancePaused"

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

	// SSLClientAuthRequired states that the client authentication is required when SSL is enabled
	SSLClientAuthRequired SSLClientAuthentication = "required"
)
