// Copyright © 2020 Banzai Cloud
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

package nodeportExternalAccess

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

const (
	componentName    = "nodePortExternalAccess"
	serviceName      = "%s-%d-%s"
)

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for NodePort based external access
func New(client client.Client, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for NodePort based external access
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.V(1).Info("Reconciling")
	if r.KafkaCluster.Spec.ListenersConfig.ExternalListeners != nil {
		for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
			if eListener.GetAccessMethod() == "nodePort" {
				for _, broker := range r.KafkaCluster.Spec.Brokers {
					brokerConfig, err := util.GetBrokerConfig(broker, r.KafkaCluster.Spec)
					if err != nil {
						return err
					}
					err = k8sutil.Reconcile(log, r.Client, r.service(log, broker.Id, brokerConfig, eListener), r.KafkaCluster)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}
