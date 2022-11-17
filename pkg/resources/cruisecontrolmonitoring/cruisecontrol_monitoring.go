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

package cruisecontrolmonitoring

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources"
)

const (
	// CruiseControlJmxTemplate holds the template for CC monitoring
	CruiseControlJmxTemplate = "%s-cc-jmx-exporter"
	componentName            = "cruisecontrol_monitoring"
)

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for CC Monitoring
func New(client client.Client, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for CC Monitoring
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.V(1).Info("Reconciling")

	if r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint == "" {
		o := r.configMap()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return err
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}

func labelsForJmx(name string) map[string]string {
	return map[string]string{v1beta1.AppLabelKey: "cruisecontrol-jmx", v1beta1.KafkaCRLabelKey: name}
}
