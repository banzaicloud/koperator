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

// PerBrokerConfigurationState holds info about the per-broker configuration state
type PerBrokerConfigurationState string

// ExternalListenerConfigNames holds the name of the brokerIdBindings
type ExternalListenerConfigNames []string

// PKIBackend represents an interface implementing the PKIManager
type PKIBackend string

// CruiseControlVolumeState holds information about the state of volume rebalance
type CruiseControlVolumeState string

func (r CruiseControlState) IsUpscale() bool {
	return r == GracefulUpscaleRequired || r == GracefulUpscaleSucceeded || r == GracefulUpscaleRunning
}

func (r CruiseControlState) IsDownscale() bool {
	return r == GracefulDownscaleRequired || r == GracefulDownscaleSucceeded || r == GracefulDownscaleRunning
}

func (r CruiseControlState) IsRunningState() bool {
	return r == GracefulDownscaleRunning || r == GracefulUpscaleRunning
}

func (r CruiseControlState) IsRequiredState() bool {
	return r == GracefulDownscaleRequired || r == GracefulUpscaleRequired
}

func (r CruiseControlState) Complete() CruiseControlState {
	switch r {
	case GracefulUpscaleRequired, GracefulUpscaleRunning:
		return GracefulUpscaleSucceeded
	case GracefulDownscaleRequired, GracefulDownscaleRunning:
		return GracefulDownscaleSucceeded
	default:
		return r
	}
}

const (
	// PKIBackendCertManager invokes cert-manager for user certificate management
	PKIBackendCertManager PKIBackend = "cert-manager"
	// PKIBackendVault invokes vault PKI for user certificate management
	PKIBackendVault PKIBackend = "vault"
	// PKIBackendProvided used to point the operator to use the PKI set in the cluster CR
	// for admin and users required for the cluster to run
	PKIBackendProvided PKIBackend = "pki-backend-provided"
)

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
	// GracefulUpscaleSucceeded states that the broker downscaled gracefully
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
	// CruiseControlTaskNotFound states the CC task is not found (can happen when CC is restarted during operation)
	CruiseControlTaskNotFound CruiseControlUserTaskState = "NotFound"
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
)
