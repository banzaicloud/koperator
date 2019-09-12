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
	"context"
	"fmt"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	Scheme *runtime.Scheme
}

// New creates a new reconciler for a PKI
func New(client client.Client, scheme *runtime.Scheme, cluster *banzaicloudv1alpha1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Scheme: scheme,
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for a kafka PKI
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets == nil {
		log.V(1).Info("Skipping PKI reconciling due to no SSL config")
		return nil
	}

	log.V(1).Info("Reconciling")

	resources, err := r.kafkapki()
	if err != nil {
		return err
	}

	for _, o := range resources {
		if err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster); err != nil {
			return err
		}
	}

	// I'm going to be lazy for now and make a copy in the format we expect
	// later during pod generation.
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.Create {

		bootSecret, passSecret, err := r.getBootstrapSSLSecret()
		if err != nil {
			return err
		}

		for _, o := range []runtime.Object{bootSecret, passSecret} {
			if err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster); err != nil {
				return err
			}
		}

		// Grab ownership of all secrets we created through cert-manager
		secretNames := []types.NamespacedName{
			{Name: fmt.Sprintf(brokerCACertTemplate, r.KafkaCluster.Name), Namespace: "cert-manager"},
			{Name: fmt.Sprintf(brokerServerCertTemplate, r.KafkaCluster.Name), Namespace: r.KafkaCluster.Namespace},
			{Name: fmt.Sprintf(BrokerControllerTemplate, r.KafkaCluster.Name), Namespace: r.KafkaCluster.Namespace},
		}

		for _, o := range secretNames {
			secret := &corev1.Secret{}
			if err := r.Client.Get(context.TODO(), o, secret); err != nil {
				return errorfactory.New(errorfactory.ResourceNotReady{}, err, "pki secret not ready")
			}

			if err := controllerutil.SetControllerReference(r.KafkaCluster, secret, r.Scheme); err != nil {
				if k8sutil.IsAlreadyOwnedError(err) {
					continue
				} else {
					return errorfactory.New(errorfactory.InternalError{}, err, "failed to set controller reference on secret")
				}
			}

			if err := r.Client.Update(context.TODO(), secret); err != nil {
				return errorfactory.New(errorfactory.APIFailure{}, err, "failed to set controller reference on secret")
			}

		}

	}

	log.V(1).Info("Reconciled")

	return nil
}
