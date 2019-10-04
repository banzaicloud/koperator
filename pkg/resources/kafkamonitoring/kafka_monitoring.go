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

package kafkamonitoring

import (
	"github.com/banzaicloud/kafka-operator/api/kafka/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// BrokerJmxTemplate holds the template for Kafka monitoring
	BrokerJmxTemplate = "%s-kafka-jmx-exporter"
	componentName     = "kafka_monitoring"
)

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for Kafka Monitoring
func New(client client.Client, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for Kafka Monitoring
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.V(1).Info("Reconciling")

	o := r.configMap()
	err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
	if err != nil {
		return err
	}

	log.V(1).Info("Reconciled")

	return nil
}

func labelsForJmx(name string) map[string]string {
	return map[string]string{"app": "kafka-jmx", "kafka_cr": name}
}
