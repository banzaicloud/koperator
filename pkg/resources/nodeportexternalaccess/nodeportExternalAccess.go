// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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

package nodeportexternalaccess

import (
	"context"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources"
)

const (
	componentName = "nodePortExternalAccess"
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
			for _, broker := range r.KafkaCluster.Spec.Brokers {
				brokerConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
				if err != nil {
					return err
				}

				service := r.service(log, broker.Id, brokerConfig, eListener)

				if eListener.GetAccessMethod() == corev1.ServiceTypeNodePort {
					err = k8sutil.Reconcile(log, r.Client, service, r.KafkaCluster)
					if err != nil {
						return err
					}
				} else {
					// Cleaning up unused nodeport services
					if err := r.Delete(context.Background(), service.(client.Object)); client.IgnoreNotFound(err) != nil {
						return errors.Wrap(err, "error happened when removing unused nodeport services")
					}
				}
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}
