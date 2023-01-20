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

package errorfactory

import "emperror.dev/errors"

// ResourceNotReady states that resource is not ready
type ResourceNotReady struct{ error }

func (e ResourceNotReady) Unwrap() error { return e.error }

// APIFailure states that something went wrong with the api
type APIFailure struct{ error }

func (e APIFailure) Unwrap() error { return e.error }

// StatusUpdateError states that the operator failed to update the Status
type StatusUpdateError struct{ error }

func (e StatusUpdateError) Unwrap() error { return e.error }

// BrokersUnreachable states that the given broker is unreachable
type BrokersUnreachable struct{ error }

func (e BrokersUnreachable) Unwrap() error { return e.error }

// BrokersNotReady states that the broker is not ready
type BrokersNotReady struct{ error }

func (e BrokersNotReady) Unwrap() error { return e.error }

// BrokersRequestError states that the broker could not understand the request
type BrokersRequestError struct{ error }

func (e BrokersRequestError) Unwrap() error { return e.error }

// CreateTopicError states that the operator could not create the topic
type CreateTopicError struct{ error }

func (e CreateTopicError) Unwrap() error { return e.error }

// TopicNotFound states that the given topic is not found
type TopicNotFound struct{ error }

func (e TopicNotFound) Unwrap() error { return e.error }

// GracefulUpscaleFailed states that the operator failed to update the cluster gracefully
type GracefulUpscaleFailed struct{ error }

func (e GracefulUpscaleFailed) Unwrap() error { return e.error }

// TooManyResources states that too many resource found
type TooManyResources struct{ error }

func (e TooManyResources) Unwrap() error { return e.error }

// InternalError states that internal error happened
type InternalError struct{ error }

func (e InternalError) Unwrap() error { return e.error }

// FatalReconcileError states that a fatal error happened
type FatalReconcileError struct{ error }

func (e FatalReconcileError) Unwrap() error { return e.error }

// ReconcileRollingUpgrade states that rolling upgrade is reconciling
type ReconcileRollingUpgrade struct{ error }

func (e ReconcileRollingUpgrade) Unwrap() error { return e.error }

// CruiseControlNotReady states that CC is not ready to receive connection
type CruiseControlNotReady struct{ error }

func (e CruiseControlNotReady) Unwrap() error { return e.error }

// CruiseControlTaskRunning states that CC task is still running
type CruiseControlTaskRunning struct{ error }

func (e CruiseControlTaskRunning) Unwrap() error { return e.error }

// CruiseControlTaskRunning states that CC task timed out
type CruiseControlTaskTimeout struct{ error }

func (e CruiseControlTaskTimeout) Unwrap() error { return e.error }

// CruiseControlTaskFailure states that CC task was not found (CC restart?) or failed
type CruiseControlTaskFailure struct{ error }

func (e CruiseControlTaskFailure) Unwrap() error { return e.error }

// PerBrokerConfigNotReady states that per-broker configurations has been updated for a broker
type PerBrokerConfigNotReady struct{ error }

func (e PerBrokerConfigNotReady) Unwrap() error { return e.error }

// LoadBalancerIPNotReady states that the LoadBalancer IP is not yet created
type LoadBalancerIPNotReady struct{ error }

func (e LoadBalancerIPNotReady) Unwrap() error { return e.error }

// New creates a new error factory error
func New(t interface{}, err error, msg string, wrapArgs ...interface{}) error {
	wrapped := errors.WrapIfWithDetails(err, msg, wrapArgs...)
	switch t.(type) {
	case ResourceNotReady:
		return ResourceNotReady{wrapped}
	case APIFailure:
		return APIFailure{wrapped}
	case StatusUpdateError:
		return StatusUpdateError{wrapped}
	case BrokersUnreachable:
		return BrokersUnreachable{wrapped}
	case BrokersNotReady:
		return BrokersNotReady{wrapped}
	case BrokersRequestError:
		return BrokersRequestError{wrapped}
	case GracefulUpscaleFailed:
		return GracefulUpscaleFailed{wrapped}
	case TopicNotFound:
		return TopicNotFound{wrapped}
	case CreateTopicError:
		return CreateTopicError{wrapped}
	case TooManyResources:
		return TooManyResources{wrapped}
	case InternalError:
		return InternalError{wrapped}
	case FatalReconcileError:
		return FatalReconcileError{wrapped}
	case ReconcileRollingUpgrade:
		return ReconcileRollingUpgrade{wrapped}
	case CruiseControlNotReady:
		return CruiseControlNotReady{wrapped}
	case CruiseControlTaskRunning:
		return CruiseControlTaskRunning{wrapped}
	case CruiseControlTaskTimeout:
		return CruiseControlTaskTimeout{wrapped}
	case CruiseControlTaskFailure:
		return CruiseControlTaskFailure{wrapped}
	case PerBrokerConfigNotReady:
		return PerBrokerConfigNotReady{wrapped}
	case LoadBalancerIPNotReady:
		return LoadBalancerIPNotReady{wrapped}
	}
	return wrapped
}
