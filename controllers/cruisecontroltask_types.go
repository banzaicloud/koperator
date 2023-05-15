// Copyright © 2022 Cisco Systems, Inc. and/or its affiliates
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

	koperatorv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	koperatorv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
)

// CruiseControlTask defines a task to be performed via Cruise Control.
type CruiseControlTask struct {
	BrokerID                        string
	BrokerState                     koperatorv1beta1.CruiseControlState
	Volume                          string
	VolumeState                     koperatorv1beta1.CruiseControlVolumeState
	Operation                       koperatorv1alpha1.CruiseControlTaskOperation
	CruiseControlOperationReference *corev1.LocalObjectReference
}

// IsRequired returns true if the task needs to be executed.
func (t *CruiseControlTask) IsRequired() bool {
	if t == nil {
		return true
	}

	// nolint:exhaustive // Note: Not all CC operations have to be checked.
	switch t.Operation {
	case koperatorv1alpha1.OperationAddBroker, koperatorv1alpha1.OperationRemoveBroker:
		return t.BrokerState.IsRequiredState()
	case koperatorv1alpha1.OperationRebalance, koperatorv1alpha1.OperationRemoveDisks:
		return t.VolumeState.IsRequiredState()
	}
	return false
}

// Apply takes a koperatorv1beta1.KafkaCluster instance and updates its Status field to reflect the state of the task.
func (t *CruiseControlTask) Apply(instance *koperatorv1beta1.KafkaCluster) {
	if t == nil || instance == nil {
		return
	}

	// nolint:exhaustive // Note: Not all CC operations require updates to the KafkaCluster status.
	switch t.Operation {
	case koperatorv1alpha1.OperationAddBroker, koperatorv1alpha1.OperationRemoveBroker:
		if state, ok := instance.Status.BrokersState[t.BrokerID]; ok {
			state.GracefulActionState.CruiseControlState = t.BrokerState
			state.GracefulActionState.CruiseControlOperationReference = t.CruiseControlOperationReference
			instance.Status.BrokersState[t.BrokerID] = state
		}
	case koperatorv1alpha1.OperationRebalance, koperatorv1alpha1.OperationRemoveDisks:
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
	// nolint:exhaustive // Note: Not all CC operations require updates to the CruiseControlTask state.
	switch t.Operation {
	case koperatorv1alpha1.OperationAddBroker:
		t.BrokerState = koperatorv1beta1.GracefulUpscaleScheduled
	case koperatorv1alpha1.OperationRemoveBroker:
		t.BrokerState = koperatorv1beta1.GracefulDownscaleScheduled
	case koperatorv1alpha1.OperationRebalance:
		t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceScheduled
	case koperatorv1alpha1.OperationRemoveDisks:
		t.VolumeState = koperatorv1beta1.GracefulDiskRemovalScheduled
	}
}

// FromResult takes a scale.Result instance returned by scale.CruiseControlScaler and updates its own state accordingly.
//
//nolint:gocyclo
func (t *CruiseControlTask) FromResult(operation *koperatorv1alpha1.CruiseControlOperation) {
	if t == nil {
		return
	}

	//nolint:exhaustive // Note: Not all CC operations require updates to kafkaCluster's GracefulActionState
	switch t.Operation {
	case koperatorv1alpha1.OperationAddBroker:
		switch {
		// When CruiseControlOperation is missing
		case operation == nil:
			t.BrokerState = koperatorv1beta1.GracefulUpscaleSucceeded
		case operation.IsErrorPolicyIgnore() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = koperatorv1beta1.GracefulUpscaleSucceeded
		case operation.IsPaused() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = koperatorv1beta1.GracefulUpscalePaused
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskActive, operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskInExecution:
			t.BrokerState = koperatorv1beta1.GracefulUpscaleRunning
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompleted:
			t.BrokerState = koperatorv1beta1.GracefulUpscaleSucceeded
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = koperatorv1beta1.GracefulUpscaleCompletedWithError
		case operation.CurrentTaskState() == "":
			t.BrokerState = koperatorv1beta1.GracefulUpscaleScheduled
		}
	case koperatorv1alpha1.OperationRemoveBroker:
		switch {
		case operation == nil:
			t.BrokerState = koperatorv1beta1.GracefulDownscaleSucceeded
		case operation.IsErrorPolicyIgnore() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = koperatorv1beta1.GracefulDownscaleSucceeded
		case operation.IsPaused() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = koperatorv1beta1.GracefulDownscalePaused
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskActive, operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskInExecution:
			t.BrokerState = koperatorv1beta1.GracefulDownscaleRunning
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompleted:
			t.BrokerState = koperatorv1beta1.GracefulDownscaleSucceeded
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.BrokerState = koperatorv1beta1.GracefulDownscaleCompletedWithError
		case operation.CurrentTaskState() == "":
			t.BrokerState = koperatorv1beta1.GracefulDownscaleScheduled
		}

	case koperatorv1alpha1.OperationRemoveDisks:
		switch {
		case operation == nil:
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalSucceeded
		case operation.IsErrorPolicyIgnore() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalSucceeded
		case operation.IsPaused() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalPaused
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskActive, operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskInExecution:
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalRunning
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompleted:
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalSucceeded
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalCompletedWithError
		case operation.CurrentTaskState() == "":
			t.VolumeState = koperatorv1beta1.GracefulDiskRemovalScheduled
		}

	case koperatorv1alpha1.OperationRebalance:
		switch {
		case operation == nil:
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceSucceeded
		case operation.IsErrorPolicyIgnore() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceSucceeded
		case operation.IsPaused() && operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalancePaused
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskActive, operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskInExecution:
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceRunning
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompleted:
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceSucceeded
		case operation.CurrentTaskState() == koperatorv1beta1.CruiseControlTaskCompletedWithError:
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceCompletedWithError
		case operation.CurrentTaskState() == "":
			t.VolumeState = koperatorv1beta1.GracefulDiskRebalanceScheduled
		}
	}
}

// CruiseControlTasksAndStates is a container for CruiseControlTask objects.
type CruiseControlTasksAndStates struct {
	tasks     []*CruiseControlTask
	tasksByOp map[koperatorv1alpha1.CruiseControlTaskOperation][]*CruiseControlTask
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
func (s *CruiseControlTasksAndStates) GetActiveTasksByOp(o koperatorv1alpha1.CruiseControlTaskOperation) []*CruiseControlTask {
	tasks := make([]*CruiseControlTask, 0, len(s.tasksByOp[o]))
	for _, task := range s.tasksByOp[o] {
		if task != nil && task.IsRequired() {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// NumActiveTasksByOp the number of active CruiseControlTask instances stored.
func (s *CruiseControlTasksAndStates) NumActiveTasksByOp(o koperatorv1alpha1.CruiseControlTaskOperation) int {
	return len(s.GetActiveTasksByOp(o))
}

// SyncState makes sure that the status of the provided koperatorv1beta1.KafkaCluster reflects the state of the
// CruiseControlTask instances.
func (s *CruiseControlTasksAndStates) SyncState(instance *koperatorv1beta1.KafkaCluster) {
	for _, task := range s.tasks {
		task.Apply(instance)
	}
}

// newCruiseControlTasksAndStates returns an initialized CruiseControlTasksAndStates instance.
func newCruiseControlTasksAndStates() *CruiseControlTasksAndStates {
	return &CruiseControlTasksAndStates{
		tasks:     make([]*CruiseControlTask, 0),
		tasksByOp: make(map[koperatorv1alpha1.CruiseControlTaskOperation][]*CruiseControlTask),
	}
}
