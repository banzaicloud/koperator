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

package controllers

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/banzaicloud/koperator/pkg/util"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
)

// clusterRefLabel is the label key used for referencing KafkaUsers/KafkaTopics
// to a KafkaCluster
var clusterRefLabel = "kafkaCluster"

// newKafkaFromCluster points to the function for retrieving kafka clients,
// use as var so it can be overwritten from unit tests
var newKafkaFromCluster = kafkaclient.NewFromCluster

func requeueAfter(sec int) (ctrl.Result, error) {
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Duration(sec) * time.Second,
	}, nil
}

// requeueWithError is a convenience wrapper around logging an error message
// separate from the stacktrace and then passing the error through to the controller
// manager
func requeueWithError(logger logr.Logger, msg string, err error) (ctrl.Result, error) {
	// Info log the error message and then let the reconciler dump the stacktrace
	logger.Info(msg)
	return ctrl.Result{
		Requeue: true,
	}, err
}

// reconciledWithError is a convenience wrapper around logging an error message
// separate from the stacktrace and then passing the error through to the controller
// manager. In this case there will be no requeue.
func reconciledWithError(logger logr.Logger, msg string, err error) (ctrl.Result, error) {
	// Info log the error message and then let the reconciler dump the stacktrace
	logger.Info(msg)
	return ctrl.Result{
		Requeue: false,
	}, err
}

// reconciled returns an empty result with nil error to signal a successful reconcile
// to the controller manager
func reconciled() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// getClusterRefNamespace returns the expected namespace for a kafka cluster
// referenced by a user/topic CR. It takes the namespace of the CR as the first
// argument and the reference itself as the second.
func getClusterRefNamespace(ns string, ref v1alpha1.ClusterReference) string {
	clusterNamespace := ref.Namespace
	if clusterNamespace == "" {
		return ns
	}
	return clusterNamespace
}

// clusterLabelString returns the label value for a cluster reference
func clusterLabelString(cluster *v1beta1.KafkaCluster) string {
	return fmt.Sprintf("%s.%s", cluster.Name, cluster.Namespace)
}

// checkBrokerConnectionError is a convenience wrapper for returning from common
// broker connection errors
func checkBrokerConnectionError(logger logr.Logger, err error) (ctrl.Result, error) {
	switch errors.Cause(err).(type) {
	case errorfactory.BrokersUnreachable:
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Duration(15) * time.Second,
		}, nil
	case errorfactory.BrokersNotReady:
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Duration(15) * time.Second,
		}, nil
	case errorfactory.ResourceNotReady:
		logger.Info("Needed resource for broker connection not found, may not be ready")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Duration(5) * time.Second,
		}, nil
	default:
		return requeueWithError(logger, err.Error(), err)
	}
}

// applyClusterRefLabel ensures a map of labels contains a reference to a parent kafka cluster
func applyClusterRefLabel(cluster *v1beta1.KafkaCluster, labels map[string]string) map[string]string {
	labelValue := clusterLabelString(cluster)
	if labels == nil {
		labels = make(map[string]string)
	}
	if label, ok := labels[clusterRefLabel]; ok {
		if label != labelValue {
			labels[clusterRefLabel] = labelValue
		}
	} else {
		labels[clusterRefLabel] = labelValue
	}
	return labels
}

func SetNewKafkaFromCluster(f func(k8sclient client.Client, cluster *v1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error)) {
	newKafkaFromCluster = f
}

// SkipClusterRegistryOwnedResourcePredicate returns a controller event filter that filters
// out events triggered by Cluster Registry owned resources
type SkipClusterRegistryOwnedResourcePredicate struct{}

func (SkipClusterRegistryOwnedResourcePredicate) Create(e event.CreateEvent) bool {
	return !util.ObjectManagedByClusterRegistry(e.Object)
}

func (SkipClusterRegistryOwnedResourcePredicate) Delete(e event.DeleteEvent) bool {
	return !util.ObjectManagedByClusterRegistry(e.Object)
}

func (p SkipClusterRegistryOwnedResourcePredicate) Update(e event.UpdateEvent) bool {
	return !util.ObjectManagedByClusterRegistry(e.ObjectNew)
}

func (p SkipClusterRegistryOwnedResourcePredicate) Generic(e event.GenericEvent) bool {
	return !util.ObjectManagedByClusterRegistry(e.Object)
}
