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

type RackAwarenessState string

type CruiseControlState string

// GracefulActionState holds information about GracefulAction State
type GracefulActionState struct {
	// ErrorMessage holds the information what happened with CC
	ErrorMessage string
	// CruiseControlState holds the information about CC state
	CruiseControlState CruiseControlState
}

// BrokerState holds information about broker state
type BrokerState struct {
	// RackAwarenessState holds info about rack awareness status
	RackAwarenessState RackAwarenessState
	// GracefulActionState holds info about cc action status
	GracefulActionState GracefulActionState
}

const (
	// Configured states the broker is running
	Configured RackAwarenessState = "Configured"
	// WaitingForRackAwareness states the broker is waiting for the rack awareness config
	WaitingForRackAwareness RackAwarenessState = "WaitingForRackAwareness"
	// Error states the broker is in error
	Error RackAwarenessState = "Error"
	// GracefulUpdateSucceeded states the broker is updated gracefully
	GracefulUpdateSucceeded CruiseControlState = "GracefulUpdateSucceeded"
	// GracefulUpdateFailed states the broker could not be updated gracefully
	GracefulUpdateFailed CruiseControlState = "GracefulUpdateFailed"
	// GracefulUpdateRequired states the broker requires an
	GracefulUpdateRequired CruiseControlState = "GracefulUpdateRequired"
	// ReadyForGracefulUpdate states the broker is

)
