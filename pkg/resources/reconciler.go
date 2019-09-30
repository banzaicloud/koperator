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
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// Reconciler holds client and CR for Kafka
type Reconciler struct {
	client.Client
	KafkaCluster *v1beta1.KafkaCluster
}

// ComponentReconciler describes the Reconcile method
type ComponentReconciler interface {
	Reconcile(log logr.Logger) error
}

// Resource simple function without parameter
type Resource func() runtime.Object

// ResourceWithLogs function with log parameter
type ResourceWithLogs func(log logr.Logger) runtime.Object

// ResourceWithLogsAndClientPassowrd function with log and password parameter
type ResourceWithLogsAndClientPassowrd func(log logr.Logger, clientPass string) runtime.Object

// ResourceWithBrokerConfigAndVolume function with brokerConfig, persistenVolumeClaims and log parameters
type ResourceWithBrokerConfigAndVolume func(id int32, brokerConfig *v1beta1.BrokerConfig, pvcs []corev1.PersistentVolumeClaim, log logr.Logger) runtime.Object

// ResourceWithBrokerConfigAndString function with brokerConfig, string and log parameters
type ResourceWithBrokerConfigAndString func(id int32, brokerConfig *v1beta1.BrokerConfig, t string, su []string, log logr.Logger) runtime.Object

// ResourceWithBrokerIdAndStorage function with brokerConfig, storageConfig and log parameters
type ResourceWithBrokerIdAndStorage func(id int32, storage v1beta1.StorageConfig, log logr.Logger) runtime.Object

// ResourceWithBrokerIdAndLog function with brokerConfig and log parameters
type ResourceWithBrokerIdAndLog func(id int32, log logr.Logger) runtime.Object
