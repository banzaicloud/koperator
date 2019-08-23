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

package resources

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Reconciler holds client and CR for Kafka
type Reconciler struct {
	client.Client
	KafkaCluster *banzaicloudv1alpha1.KafkaCluster
}

// ComponentReconciler describes the Reconcile method
type ComponentReconciler interface {
	Reconcile(log logr.Logger) error
}

// Resource simple function without parameter
type Resource func() runtime.Object

// ResourceWithLogs function with log parameter
type ResourceWithLogs func(log logr.Logger) runtime.Object

// ResourceWithBrokerAndVolume function with brokerConfig, persistenVolumeClaims and log parameters
type ResourceWithBrokerAndVolume func(broker banzaicloudv1alpha1.BrokerConfig, pvcs []corev1.PersistentVolumeClaim, log logr.Logger) runtime.Object

// ResourceWithBrokerAndString function with brokerConfig, string and log parameters
type ResourceWithBrokerAndString func(broker banzaicloudv1alpha1.BrokerConfig, t string, log logr.Logger) runtime.Object

// ResourceWithBrokerAndStorage function with brokerConfig, storageConfig and log parameters
type ResourceWithBrokerAndStorage func(broker banzaicloudv1alpha1.BrokerConfig, storage banzaicloudv1alpha1.StorageConfig, log logr.Logger) runtime.Object

// ResourceWithBrokerAndLog function with brokerConfig and log parameters
type ResourceWithBrokerAndLog func(broker banzaicloudv1alpha1.BrokerConfig, log logr.Logger) runtime.Object
