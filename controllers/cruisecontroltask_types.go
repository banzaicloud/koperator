// Copyright © 2022 Banzai Cloud
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

package controllers

import (
	kafkav1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
)

type CruiseControlOperation int8

const (
	OperationAddBroker CruiseControlOperation = iota
	OperationRemoveBroker
	OperationRebalanceDisks
)

// CruiseControlTask defines a task to be performed via Cruise Control.
type CruiseControlTask struct {
	TaskID    string
	StartedAt string

	BrokerID    string
	BrokerState kafkav1beta1.CruiseControlState

	Volume      string
	VolumeState kafkav1beta1.CruiseControlVolumeState

	Err       string
	Operation CruiseControlOperation
}

// IsDone returns true if the task is considered finished.
func (t *CruiseControlTask) IsDone() bool {
	if t == nil {
		return true
	}

	switch t.Operation {
	case OperationAddBroker, OperationRemoveBroker:
		return !t.BrokerState.IsActive()
	case OperationRebalanceDisks:
		return !t.VolumeState.IsActive()
	}
	return false
}

// Apply takes a kafkav1beta1.KafkaCluster instance and updates its Status field to reflect the state of the task.
func (t *CruiseControlTask) Apply(instance *kafkav1beta1.KafkaCluster) {
	if t == nil || instance == nil {
		return
	}

	switch t.Operation {
	case OperationAddBroker, OperationRemoveBroker:
		if state, ok := instance.Status.BrokersState[t.BrokerID]; ok {
			state.GracefulActionState.CruiseControlState = t.BrokerState
			state.GracefulActionState.CruiseControlTaskId = t.TaskID
			state.GracefulActionState.TaskStarted = t.StartedAt
			state.GracefulActionState.ErrorMessage = t.Err
			instance.Status.BrokersState[t.BrokerID] = state
		}
	case OperationRebalanceDisks:
		if state, ok := instance.Status.BrokersState[t.BrokerID]; ok {
			if volState, ok := state.GracefulActionState.VolumeStates[t.Volume]; ok {
				volState.CruiseControlVolumeState = t.VolumeState
				volState.CruiseControlTaskId = t.TaskID
				volState.TaskStarted = t.StartedAt
				volState.ErrorMessage = t.Err
				instance.Status.BrokersState[t.BrokerID].GracefulActionState.VolumeStates[t.Volume] = volState
			}
		}
	}
}

// FromResult takes a scale.Result instance returned by scale.CruiseControlScaler and updates its own state accordingly.
func (t *CruiseControlTask) FromResult(result *scale.Result) {
	if t == nil || result == nil {
		return
	}

	switch t.Operation {
	case OperationAddBroker:
		switch result.State {
		case kafkav1beta1.CruiseControlTaskActive, kafkav1beta1.CruiseControlTaskInExecution:
			t.BrokerState = kafkav1beta1.GracefulUpscaleRunning
		case kafkav1beta1.CruiseControlTaskCompleted, kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulUpscaleSucceeded
		}
	case OperationRemoveBroker:
		switch result.State {
		case kafkav1beta1.CruiseControlTaskActive, kafkav1beta1.CruiseControlTaskInExecution:
			t.BrokerState = kafkav1beta1.GracefulDownscaleRunning
		case kafkav1beta1.CruiseControlTaskCompleted, kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulDownscaleSucceeded
		}
	case OperationRebalanceDisks:
		switch result.State {
		case kafkav1beta1.CruiseControlTaskActive, kafkav1beta1.CruiseControlTaskInExecution:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceRunning
		case kafkav1beta1.CruiseControlTaskCompleted, kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceSucceeded
		}
	}

	t.TaskID = result.TaskID
	t.StartedAt = result.StartedAt
	t.Err = result.Err
}

// CruiseControlTasksAndStates is a container for CruiseControlTask objects.
type CruiseControlTasksAndStates struct {
	tasks     []*CruiseControlTask
	tasksByOp map[CruiseControlOperation][]*CruiseControlTask
}

// Add registers the provided CruiseControlTask instance.
func (s *CruiseControlTasksAndStates) Add(t *CruiseControlTask) {
	if t == nil {
		return
	}
	s.tasks = append(s.tasks, t)
	s.tasksByOp[t.Operation] = append(s.tasksByOp[t.Operation], t)
}

// IsEmpty returns true if CruiseControlTasksAndStates has no CruiseControlTask added.
func (s CruiseControlTasksAndStates) IsEmpty() bool {
	return len(s.tasks) == 0
}

// GetActiveTasksByOp returns a list of active CruiseControlTask filtered by the provided CruiseControlOperation type.
func (s *CruiseControlTasksAndStates) GetActiveTasksByOp(o CruiseControlOperation) []*CruiseControlTask {
	tasks := make([]*CruiseControlTask, 0, len(s.tasksByOp[o]))
	for _, task := range s.tasksByOp[o] {
		if task != nil && !task.IsDone() {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// NumActiveTasksByOp the number of active CruiseControlTask instances stored.
func (s *CruiseControlTasksAndStates) NumActiveTasksByOp(o CruiseControlOperation) int {
	return len(s.GetActiveTasksByOp(o))
}

// SyncState makes sure that the status of the provided kafkav1beta1.KafkaCluster reflects the state of the
// CruiseControlTask instances.
func (s *CruiseControlTasksAndStates) SyncState(instance *kafkav1beta1.KafkaCluster) {
	for _, task := range s.tasks {
		task.Apply(instance)
	}
}

// newCruiseControlTasksAndStates returns an initialized CruiseControlTasksAndStates instance.
func newCruiseControlTasksAndStates() *CruiseControlTasksAndStates {
	return &CruiseControlTasksAndStates{
		tasks:     make([]*CruiseControlTask, 0),
		tasksByOp: make(map[CruiseControlOperation][]*CruiseControlTask),
	}
}
