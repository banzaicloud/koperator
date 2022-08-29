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
	corev1 "k8s.io/api/core/v1"

	kafkav1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	kafkav1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
)

// CruiseControlTask defines a task to be performed via Cruise Control.
type CruiseControlTask struct {
	BrokerID                        string
	BrokerState                     kafkav1beta1.CruiseControlState
	Volume                          string
	VolumeState                     kafkav1beta1.CruiseControlVolumeState
	Operation                       kafkav1alpha1.CruiseControlTaskOperation
	CruiseControlOperationReference *corev1.LocalObjectReference
}

// IsDone returns true if the task is considered finished.
func (t *CruiseControlTask) IsRequired() bool {
	if t == nil {
		return true
	}

	//nolint:exhaustive
	switch t.Operation {
	case kafkav1alpha1.OperationAddBroker, kafkav1alpha1.OperationRemoveBroker:
		return t.BrokerState.IsRequiredState()
	case kafkav1alpha1.OperationRebalance:
		return t.VolumeState.IsRequiredState()
	}
	return false
}

func (t *CruiseControlTask) IsRunning() bool {
	if t == nil {
		return true
	}

	//nolint:exhaustive
	switch t.Operation {
	case kafkav1alpha1.OperationAddBroker, kafkav1alpha1.OperationRemoveBroker:
		return t.BrokerState.IsRunningState()
	case kafkav1alpha1.OperationRebalance:
		return t.VolumeState.IsRunningState()
	}
	return false
}

// Apply takes a kafkav1beta1.KafkaCluster instance and updates its Status field to reflect the state of the task.
func (t *CruiseControlTask) Apply(instance *kafkav1beta1.KafkaCluster) {
	if t == nil || instance == nil {
		return
	}
	//nolint:exhaustive
	switch t.Operation {
	case kafkav1alpha1.OperationAddBroker, kafkav1alpha1.OperationRemoveBroker:
		if state, ok := instance.Status.BrokersState[t.BrokerID]; ok {
			state.GracefulActionState.CruiseControlState = t.BrokerState
			state.GracefulActionState.CruiseControlOperationReference = t.CruiseControlOperationReference
			instance.Status.BrokersState[t.BrokerID] = state
		}
	case kafkav1alpha1.OperationRebalance:
		if state, ok := instance.Status.BrokersState[t.BrokerID]; ok {
			if volState, ok := state.GracefulActionState.VolumeStates[t.Volume]; ok {
				volState.CruiseControlVolumeState = t.VolumeState
				volState.CruiseControlOperationReference = t.CruiseControlOperationReference
				instance.Status.BrokersState[t.BrokerID].GracefulActionState.VolumeStates[t.Volume] = volState
			}
		}
	}
}

func (t *CruiseControlTask) SetCruiseControlOperationRef(ref corev1.LocalObjectReference) {
	if t == nil {
		return
	}
	t.CruiseControlOperationReference = &ref
}

func (t *CruiseControlTask) SetStateScheduled() {
	//nolint:exhaustive
	switch t.Operation {
	case kafkav1alpha1.OperationAddBroker:
		t.BrokerState = kafkav1beta1.GracefulUpscaleScheduled
	case kafkav1alpha1.OperationRemoveBroker:
		t.BrokerState = kafkav1beta1.GracefulDownscaleScheduled
	case kafkav1alpha1.OperationRebalance:
		t.VolumeState = kafkav1beta1.GracefulDiskRebalanceScheduled
	}
}

// FromResult takes a scale.Result instance returned by scale.CruiseControlScaler and updates its own state accordingly.
func (t *CruiseControlTask) FromResult(operation *kafkav1alpha1.CruiseControlOperation) {
	if t == nil {
		return
	}

	//nolint:exhaustive
	switch t.Operation {
	case kafkav1alpha1.OperationAddBroker:
		switch {
		// When CruiseControlOperation is missing
		case operation == nil:
			t.BrokerState = kafkav1beta1.GracefulUpscaleSucceeded
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskActive, operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskInExecution:
			t.BrokerState = kafkav1beta1.GracefulUpscaleRunning
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompleted:
			t.BrokerState = kafkav1beta1.GracefulUpscaleSucceeded
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulUpscaleCompletedWithError
		case operation.IsErrorPolicyIgnore() && operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulUpscaleSucceeded
		case operation.IsPaused() && operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulUpscalePaused
		case operation.GetCurrentTaskState() == "":
			t.BrokerState = kafkav1beta1.GracefulUpscaleScheduled
		}
	case kafkav1alpha1.OperationRemoveBroker:
		switch {
		case operation == nil:
			t.BrokerState = kafkav1beta1.GracefulDownscaleSucceeded
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskActive, operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskInExecution:
			t.BrokerState = kafkav1beta1.GracefulDownscaleRunning
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompleted:
			t.BrokerState = kafkav1beta1.GracefulDownscaleSucceeded
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulDownscaleCompletedWithError
		case operation.IsErrorPolicyIgnore() && operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulDownscaleSucceeded
		case operation.IsPaused() && operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = kafkav1beta1.GracefulDownscalePaused
		case operation.GetCurrentTaskState() == "":
			t.BrokerState = kafkav1beta1.GracefulDownscaleScheduled
		}

	case kafkav1alpha1.OperationRebalance:
		switch {
		case operation == nil:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceSucceeded
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskActive, operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskInExecution:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceRunning
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompleted:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceSucceeded
		case operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceCompletedWithError
		case operation.IsErrorPolicyIgnore() && operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceSucceeded
		case operation.IsPaused() && operation.GetCurrentTaskState() == kafkav1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = kafkav1beta1.GracefulDiskRebalancePaused
		case operation.GetCurrentTaskState() == "":
			t.VolumeState = kafkav1beta1.GracefulDiskRebalanceScheduled
		}
	}
}

// CruiseControlTasksAndStates is a container for CruiseControlTask objects.
type CruiseControlTasksAndStates struct {
	tasks     []*CruiseControlTask
	tasksByOp map[kafkav1alpha1.CruiseControlTaskOperation][]*CruiseControlTask
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
func (s *CruiseControlTasksAndStates) GetActiveTasksByOp(o kafkav1alpha1.CruiseControlTaskOperation) []*CruiseControlTask {
	tasks := make([]*CruiseControlTask, 0, len(s.tasksByOp[o]))
	for _, task := range s.tasksByOp[o] {
		if task != nil && task.IsRequired() {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// NumActiveTasksByOp the number of active CruiseControlTask instances stored.
func (s *CruiseControlTasksAndStates) NumActiveTasksByOp(o kafkav1alpha1.CruiseControlTaskOperation) int {
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
		tasksByOp: make(map[kafkav1alpha1.CruiseControlTaskOperation][]*CruiseControlTask),
	}
}
