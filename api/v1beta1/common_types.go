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

// ClusterState holds info about the cluster state
type ClusterState string

// ConfigurationState holds info about the configuration state
type ConfigurationState string

// PKIBackend represents an interface implementing the PKIManager
type PKIBackend string

const (
	// PKIBackendCertManager invokes cert-manager for user certificate management
	PKIBackendCertManager PKIBackend = "cert-manager"
	// PKIBackendVault invokves vault PKI for user certificate management
	PKIBackendVault PKIBackend = "vault"
)

// GracefulActionState holds information about GracefulAction State
type GracefulActionState struct {
	// ErrorMessage holds the information what happened with CC
	ErrorMessage string `json:"errorMessage"`
	// CruiseControlState holds the information about CC state
	CruiseControlState CruiseControlState `json:"cruiseControlState"`
}

// BrokerState holds information about broker state
type BrokerState struct {
	// RackAwarenessState holds info about rack awareness status
	RackAwarenessState RackAwarenessState `json:"rackAwarenessState"`
	// GracefulActionState holds info about cc action status
	GracefulActionState GracefulActionState `json:"gracefulActionState"`
	// ConfigurationState holds info about the config
	ConfigurationState ConfigurationState `json:"configurationState"`
}

const (
	// Configured states the broker is running
	Configured RackAwarenessState = "Configured"
	// WaitingForRackAwareness states the broker is waiting for the rack awareness config
	WaitingForRackAwareness RackAwarenessState = "WaitingForRackAwareness"
	// GracefulUpdateSucceeded states the broker is updated gracefully
	GracefulUpdateSucceeded CruiseControlState = "GracefulUpdateSucceeded"
	// GracefulUpdateFailed states the broker could not be updated gracefully
	GracefulUpdateFailed CruiseControlState = "GracefulUpdateFailed"
	// GracefulUpdateRequired states the broker requires an
	GracefulUpdateRequired CruiseControlState = "GracefulUpdateRequired"
	// GracefulUpdateNotRequired states the broker is the part of the initial cluster where CC is still in creating stage
	GracefulUpdateNotRequired CruiseControlState = "GracefulUpdateNotRequired"
	// CruiseControlTopicNotReady states the CC required topic is not yet created
	CruiseControlTopicNotReady CruiseControlTopicStatus = "CruiseControlTopicNotReady"
	// CruiseControlTopicReady states the CC required topic is created
	CruiseControlTopicReady CruiseControlTopicStatus = "CruiseControlTopicReady"
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
)
