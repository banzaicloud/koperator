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

package webhooks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/go-logr/logr"

	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

type KafkaClusterValidator struct {
	Log logr.Logger
}

func (s KafkaClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList
	kafkaClusterNew := newObj.(*banzaicloudv1beta1.KafkaCluster)
	log := s.Log.WithValues("name", kafkaClusterNew.GetName(), "namespace", kafkaClusterNew.GetNamespace())

	listenerErrs := checkInternalAndExternalListeners(&kafkaClusterNew.Spec)
	if listenerErrs != nil {
		allErrs = append(allErrs, listenerErrs...)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	log.Info("rejected", "invalid field(s)", allErrs.ToAggregate().Error())
	return nil, apierrors.NewInvalid(
		kafkaClusterNew.GroupVersionKind().GroupKind(),
		kafkaClusterNew.Name, allErrs)
}

func (s KafkaClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList
	kafkaCluster := obj.(*banzaicloudv1beta1.KafkaCluster)
	log := s.Log.WithValues("name", kafkaCluster.GetName(), "namespace", kafkaCluster.GetNamespace())

	listenerErrs := checkInternalAndExternalListeners(&kafkaCluster.Spec)
	if listenerErrs != nil {
		allErrs = append(allErrs, listenerErrs...)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	log.Info("rejected", "invalid field(s)", allErrs.ToAggregate().Error())
	return nil, apierrors.NewInvalid(
		kafkaCluster.GroupVersionKind().GroupKind(),
		kafkaCluster.Name, allErrs)
}

func (s KafkaClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

// checkListeners validates the spec.listenersConfig object
func checkInternalAndExternalListeners(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, checkInternalListeners(kafkaClusterSpec)...)

	allErrs = append(allErrs, checkExternalListeners(kafkaClusterSpec)...)

	return allErrs
}

func checkInternalListeners(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	return checkUniqueListenerContainerPort(kafkaClusterSpec.ListenersConfig)
}

func checkExternalListeners(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, checkExternalListenerStartingPort(kafkaClusterSpec)...)

	allErrs = append(allErrs, checkTargetPortsCollisionForEnvoy(kafkaClusterSpec)...)

	return allErrs
}

// checkUniqueListenerContainerPort checks for duplicate containerPort numbers across both internal and external listeners
// which would subsequently generate a "Duplicate value" error when creating a Service which accumulates all these ports.
// The first time a port number is found will not be reported as duplicate; only subsequent instances using that port are.
// (this is done to keep in tune with the way the K8s Service would report the "Duplicate value" error which ignores the first instance)
func checkUniqueListenerContainerPort(listeners banzaicloudv1beta1.ListenersConfig) field.ErrorList {
	var allErrs field.ErrorList
	var containerPorts = make(map[int32]int)

	for i, intListener := range listeners.InternalListeners {
		containerPorts[intListener.ContainerPort] += 1
		if containerPorts[intListener.ContainerPort] > 1 {
			fldErr := field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("internalListeners").Index(i).Child("containerPort"), intListener.ContainerPort)
			allErrs = append(allErrs, fldErr)
		}
	}
	for i, extListener := range listeners.ExternalListeners {
		containerPorts[extListener.ContainerPort] += 1
		if containerPorts[extListener.ContainerPort] > 1 {
			fldErr := field.Duplicate(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("containerPort"), extListener.ContainerPort)
			allErrs = append(allErrs, fldErr)
		}
	}

	return allErrs
}

// checkExternalListenerStartingPort checks the generic sanity of the resulting external port (valid number between 1 and 65535)
func checkExternalListenerStartingPort(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	// if there are no externalListeners, there is no need to perform the rest of the checks in this function
	if kafkaClusterSpec.ListenersConfig.ExternalListeners == nil {
		return nil
	}

	var allErrs field.ErrorList
	const maxPort int32 = 65535
	for i, extListener := range kafkaClusterSpec.ListenersConfig.ExternalListeners {
		var outOfRangeBrokerIDs, collidingPortsBrokerIDs []int32
		for _, broker := range kafkaClusterSpec.Brokers {
			externalPort := util.GetExternalPortForBroker(extListener.ExternalStartingPort, broker.Id)
			if externalPort < 1 || externalPort > maxPort {
				outOfRangeBrokerIDs = append(outOfRangeBrokerIDs, broker.Id)
			}

			if externalPort == extListener.GetIngressControllerTargetPort() {
				collidingPortsBrokerIDs = append(collidingPortsBrokerIDs, broker.Id)
			}

			if kafkaClusterSpec.GetIngressController() == "envoy" {
				if externalPort == kafkaClusterSpec.EnvoyConfig.GetEnvoyAdminPort() || externalPort == kafkaClusterSpec.EnvoyConfig.GetEnvoyHealthCheckPort() {
					collidingPortsBrokerIDs = append(collidingPortsBrokerIDs, broker.Id)
				}
			}
		}

		if len(outOfRangeBrokerIDs) > 0 {
			errmsg := invalidExternalListenerStartingPortErrMsg + ": " + fmt.Sprintf("ExternalListener '%s' would generate external access port numbers (externalStartingPort + Broker ID) that are out of range (not between 1 and 65535) for brokers %v", extListener.Name, outOfRangeBrokerIDs)
			fldErr := field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("externalStartingPort"), extListener.ExternalStartingPort, errmsg)
			allErrs = append(allErrs, fldErr)
		}

		if len(collidingPortsBrokerIDs) > 0 {
			errmsg := invalidExternalListenerStartingPortErrMsg + ": " + fmt.Sprintf("ExternalListener '%s' would generate external access port numbers ("+ //nolint:goconst
				"externalStartingPort + Broker ID) that collide with either the envoy admin port ('%d'), the envoy health-check port ('%d'), or the ingressControllerTargetPort ('%d') for brokers %v",
				extListener.Name, kafkaClusterSpec.EnvoyConfig.GetEnvoyAdminPort(), kafkaClusterSpec.EnvoyConfig.GetEnvoyHealthCheckPort(), extListener.GetIngressControllerTargetPort(), collidingPortsBrokerIDs)
			fldErr := field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("externalStartingPort"), extListener.ExternalStartingPort, errmsg)
			allErrs = append(allErrs, fldErr)
		}
	}
	return allErrs
}

// checkTargetPortsCollisionForEnvoy checks if the IngressControllerTargetPort collides with the other container ports for envoy deployment
func checkTargetPortsCollisionForEnvoy(kafkaClusterSpec *banzaicloudv1beta1.KafkaClusterSpec) field.ErrorList {
	if kafkaClusterSpec.GetIngressController() != "envoy" {
		return nil
	}

	var allErrs field.ErrorList

	ap := kafkaClusterSpec.EnvoyConfig.GetEnvoyAdminPort()
	hcp := kafkaClusterSpec.EnvoyConfig.GetEnvoyHealthCheckPort()

	if ap == hcp {
		errmsg := invalidContainerPortForIngressControllerErrMsg + ": The envoy configuration uses an admin port number that collides with the health-check port number" //nolint:goconst
		fldErr := field.Invalid(field.NewPath("spec").Child("envoyConfig").Child("adminPort"), kafkaClusterSpec.EnvoyConfig.GetEnvoyAdminPort(), errmsg)
		allErrs = append(allErrs, fldErr)
	}

	if kafkaClusterSpec.ListenersConfig.ExternalListeners != nil {
		for i, extListener := range kafkaClusterSpec.ListenersConfig.ExternalListeners {
			// the ingress controller target port only has impact while using LoadBalancer to access the Kafka cluster
			if extListener.GetAccessMethod() != corev1.ServiceTypeLoadBalancer {
				continue
			}

			if extListener.GetIngressControllerTargetPort() == ap {
				errmsg := invalidContainerPortForIngressControllerErrMsg + ": " + fmt.Sprintf(
					"ExternalListener '%s' uses an ingress controller target port number that collides with the envoy's admin port", extListener.Name)
				fldErr := field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("ingressControllerTargetPort"), extListener.GetIngressControllerTargetPort(), errmsg)
				allErrs = append(allErrs, fldErr)
			}

			if extListener.GetIngressControllerTargetPort() == hcp {
				errmsg := invalidContainerPortForIngressControllerErrMsg + ": " + fmt.Sprintf(
					"ExternalListener '%s' uses an ingress controller target port number that collides with the envoy's health-check port", extListener.Name)
				fldErr := field.Invalid(field.NewPath("spec").Child("listenersConfig").Child("externalListeners").Index(i).Child("ingressControllerTargetPort"), extListener.GetIngressControllerTargetPort(), errmsg)
				allErrs = append(allErrs, fldErr)
			}
		}
	}

	return allErrs
}
