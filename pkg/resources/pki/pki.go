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

package pki

import (
	"fmt"

	"emperror.dev/emperror"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName            = "pki"
	brokerSelfSignerTemplate = "%s-self-signer"
	brokerCACertTemplate     = "%s-ca-certificate"
	brokerServerCertTemplate = "%s-server-certificate"
	BrokerIssuerTemplate     = "%s-issuer"
	// exported for lookups
	BrokerControllerTemplate = "%s-crd-controller"
)

func labelsForKafkaPKI(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_issuer": fmt.Sprintf(BrokerIssuerTemplate, name)}
}

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for a PKI
func New(client client.Client, cluster *banzaicloudv1alpha1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for a kafka PKI
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.V(1).Info("Reconciling")

	resources, err := r.kafkapki()
	if err != nil {
		return err
	}

	for _, o := range resources {
		if err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster); err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	// I'm going to be lazy for now and make a copy in the format we expect
	// later during pod generation.
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.Create {

		bootSecret, passSecret, err := r.getBootstrapSSLSecret()
		if err != nil {
			// our secret may not be ready, we can come back to it
			return emperror.WrapWith(err, "failed to retrieve bootstrap secret from pki")
		}

		for _, o := range []runtime.Object{bootSecret, passSecret} {
			if err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster); err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}

	}

	log.V(1).Info("Reconciled")

	return nil
}
