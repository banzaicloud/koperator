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

// APIFailure states that something went wrong with the api
type APIFailure struct{ error }

// StatusUpdateError states that the operator failed to update the Status
type StatusUpdateError struct{ error }

// BrokersUnreachable states that the given broker is unreachable
type BrokersUnreachable struct{ error }

// BrokersNotReady states that the broker is not ready
type BrokersNotReady struct{ error }

// BrokersRequestError states that the broker could not understand the request
type BrokersRequestError struct{ error }

// CreateTopicError states that the operator could not create the topic
type CreateTopicError struct{ error }

// TopicNotFound states that the given topic is not found
type TopicNotFound struct{ error }

// GracefulUpscaleFailed states that the operator failed to update the cluster gracefully
type GracefulUpscaleFailed struct{ error }

// TooManyResources states that too many resource found
type TooManyResources struct{ error }

// InternalError states that internal error happened
type InternalError struct{ error }

// FatalReconcileError states that a fatal error happened
type FatalReconcileError struct{ error }

// ReconcileRollingUpgrade states that rolling upgrade is reconciling
type ReconcileRollingUpgrade struct{ error }

// CruiseControlNotReady states that CC is not ready to receive connection
type CruiseControlNotReady struct{ error }

// CruiseControlTaskRunning states that CC task is still running
type CruiseControlTaskRunning struct{ error }

// CruiseControlTaskRunning states that CC task timed out
type CruiseControlTaskTimeout struct{ error }

// CruiseControlTaskFailure states that CC task was not found (CC restart?) or failed
type CruiseControlTaskFailure struct{ error }

// PerBrokerConfigNotReady states that per-broker configurations has been updated for a broker
type PerBrokerConfigNotReady struct{ error }

// LoadBalancerIPNotReady states that the LoadBalancer IP is not yet created
type LoadBalancerIPNotReady struct{ error }

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
