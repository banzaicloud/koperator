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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

const (
	// ErrorPolicyIgnore means the Koperator handles the failed task as completed.
	ErrorPolicyIgnore ErrorPolicyType = "ignore"
	// ErrorPolicyRetry means Koperator re-executes the failed task in every 30 sec (by default).
	ErrorPolicyRetry ErrorPolicyType = "retry"
	// DefaultRetryBackOffDurationSec defines the time between retries of the failed tasks.
	DefaultRetryBackOffDurationSec = 30
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CruiseControlOperation is the Schema for the cruiseControlOperation API.
type CruiseControlOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CruiseControlOperationSpec   `json:"spec,omitempty"`
	Status CruiseControlOperationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// CruiseControlOperationList contains a list of CruiseControlOperation.
type CruiseControlOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CruiseControlOperation `json:"items"`
}

// CruiseControlOperationSpec defines the desired state of CruiseControlOperation.
type CruiseControlOperationSpec struct {
	// ErrorPolicy defines how failed Cruise Control operation should be handled.
	// When it is "retry", the Koperator re-executes the failed task in every 30 sec (by default).
	// When it is "ignore", the Koperator handles the failed task as completed.
	// +kubebuilder:validation:Enum=ignore;retry
	// +kubebuilder:default=retry
	// +optional
	ErrorPolicy ErrorPolicyType `json:"errorPolicy,omitempty"`
}

// ErrorPolicyType defines methods of handling Cruise Control user task errors.
type ErrorPolicyType string

// CruiseControlOperationStatus defines the observed state of CruiseControlOperation.
type CruiseControlOperationStatus struct {
	CurrentTask     *CruiseControlTask  `json:"currentTask,omitempty"`
	ErrorPolicy     ErrorPolicyType     `json:"errorPolicy"`
	NumberOfRetries int                 `json:"numberOfRetries"`
	FailedTasks     []CruiseControlTask `json:"failedTasks,omitempty"`
}

// CruiseControlTask defines the observed state of the Cruise Control user task.
type CruiseControlTask struct {
	ID       string       `json:"id,omitempty"`
	Started  *metav1.Time `json:"started,omitempty"`
	Finished *metav1.Time `json:"finished,omitempty"`
	// Operation defines the Cruise Control operation kind.
	Operation CruiseControlTaskOperation `json:"operation"`
	// Parameters defines the configuration of the operation.
	Parameters map[string]string `json:"parameters,omitempty"`
	// HTTPRequest is a Cruise Control user task HTTP request.
	HTTPRequest      string `json:"httpRequest,omitempty"`
	HTTPResponseCode *int   `json:"httpResponseCode,omitempty"`
	// Summary of the Cruise Control user task execution proposal.
	Summary map[string]string `json:"summary,omitempty"`
	// State is the current state of the Cruise Control user task.
	State        v1beta1.CruiseControlUserTaskState `json:"state,omitempty"`
	ErrorMessage string                             `json:"errorMessage,omitempty"`
}

func init() {
	SchemeBuilder.Register(&CruiseControlOperation{}, &CruiseControlOperationList{})
}

func (o *CruiseControlOperation) GetCurrentTask() *CruiseControlTask {
	return o.Status.CurrentTask
}

func (o *CruiseControlOperation) GetCurrentTaskParameters() map[string]string {
	if o.GetCurrentTask() == nil {
		return nil
	}
	return o.GetCurrentTask().Parameters
}

func (o *CruiseControlOperation) GetClusterRef() string {
	return o.GetLabels()[v1beta1.KafkaCRLabelKey]
}

func (o *CruiseControlOperation) GetCurrentTaskState() v1beta1.CruiseControlUserTaskState {
	if o.GetCurrentTask() != nil {
		return o.GetCurrentTask().State
	}
	return ""
}

func (o *CruiseControlOperation) GetCurrentTaskID() string {
	if o.GetCurrentTask() != nil {
		return o.GetCurrentTask().ID
	}
	return ""
}

func (o *CruiseControlOperation) GetCurrentTaskOp() CruiseControlTaskOperation {
	if o.GetCurrentTask() != nil {
		return o.GetCurrentTask().Operation
	}
	return ""
}

func (o *CruiseControlOperation) IsWaitingForFirstExecution() bool {
	if (o.GetLabels()["pause"] != "true" && o.Spec.ErrorPolicy != ErrorPolicyIgnore) &&
		o.GetCurrentTaskState() == "" && o.GetCurrentTaskID() == "" {
		return true
	}
	return false
}

func (o *CruiseControlOperation) IsInProgress() bool {
	if o.GetCurrentTaskID() != "" && (o.GetCurrentTaskState() == v1beta1.CruiseControlTaskActive || o.GetCurrentTaskState() == v1beta1.CruiseControlTaskInExecution) {
		return true
	}
	return false
}

func (o *CruiseControlOperation) IsWaitingForRetryExecution() bool {
	if (o.GetLabels()["pause"] != "true" && o.Spec.ErrorPolicy != ErrorPolicyIgnore) &&
		o.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompletedWithError && o.GetCurrentTaskID() != "" {
		return true
	}
	return false
}

func (o *CruiseControlOperation) IsReadyForRetryExecution() bool {
	return o.IsWaitingForRetryExecution() && o.GetCurrentTask().Finished != nil && o.GetCurrentTask().Finished.Add(time.Second*DefaultRetryBackOffDurationSec).Before(time.Now())
}

func (o *CruiseControlOperation) IsCurrentTaskRunning() bool {
	return (o.GetCurrentTaskState() == v1beta1.CruiseControlTaskInExecution || o.GetCurrentTaskState() == v1beta1.CruiseControlTaskActive) && o.GetCurrentTask().Finished == nil
}

func (o *CruiseControlOperation) IsCurrentTaskFinished() bool {
	return o.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompleted || o.GetCurrentTaskState() == v1beta1.CruiseControlTaskCompletedWithError
}

func (o *CruiseControlOperation) IsCurrentTaskOperationValid() bool {
	return o.GetCurrentTaskOp() == OperationAddBroker ||
		o.GetCurrentTaskOp() == OperationRebalance || o.GetCurrentTaskOp() == OperationRemoveBroker || o.GetCurrentTaskOp() == OperationStopExecution
}
