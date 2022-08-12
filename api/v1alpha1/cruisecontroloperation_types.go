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

import (
	"github.com/banzaicloud/koperator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// CruiseControlOperationList contains a list of CruiseControlOperation.
type CruiseControlOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CruiseControlOperation `json:"items"`
}

// CruiseControlOperationSpec defines the desired state of CruiseControlOperation.
type CruiseControlOperationSpec struct {
	// FailurePolicy defines how failed downscale operations should be handled.
	// When it is "retry" the koperator re-executes the failed task in every 30 sec.
	// When it is "ignore" the koperator handles the failed task as completed.
	// +kubebuilder:validation:Enum=ignore;retry
	// +optional
	ErrorPolicy ErrorPolicyType `json:"errorPolicy,omitempty"`
}

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
	ID       string       `json:"id"`
	Started  metav1.Time  `json:"started"`
	Finished *metav1.Time `json:"finished,omitempty"`
	// Operation defines the Cruise Control operation kind.
	Operation CruiseControlTaskOperation `json:"operation"`
	// Parameters defines the configuration of the operation.
	Parameters map[string]string `json:"parameters,omitempty"`
	// HTTPRequest is a Cruise Control user task HTTP request.
	HTTPRequest      string `json:"httpRequest"`
	HTTPResponseCode *int   `json:"httpResponseCode,omitempty"`
	// Summary of the Cruise Control user task execution proposal.
	Summary map[string]string `json:"summary,omitempty"`
	// State is the current state of the Cruise Control user task.
	State        *v1beta1.CruiseControlUserTaskState `json:"state,omitempty"`
	ErrorMessage *string                             `json:"errorMessage,omitempty"`
}
