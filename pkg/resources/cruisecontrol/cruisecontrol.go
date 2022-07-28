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

package cruisecontrol

import (
	"context"
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"
)

const (
	componentNameTemplate                                = "%s-cruisecontrol"
	serviceNameTemplate                                  = "%s-cruisecontrol-svc"
	configAndVolumeNameTemplate                          = "%s-cruisecontrol-config"
	deploymentNameTemplate                               = "%s-cruisecontrol"
	keystoreVolume                                       = "ks-files"
	keystoreVolumePath                                   = "/var/run/secrets/java.io/keystores"
	jmxVolumePath                                        = "/opt/jmx-exporter/"
	jmxVolumeName                                        = "jmx-jar-data"
	metricsPort                                          = 9020
	capacityConfigAnnotation                             = "cruise-control.banzaicloud.com/broker-capacity-config"
	staticCapacityConfig        CapacityConfigAnnotation = "static"
	warnLevel                                            = -1
)

type CapacityConfigAnnotation string

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

func ccLabelSelector(kafkaCluster string) map[string]string {
	return map[string]string{
		v1beta1.AppLabelKey:     "cruisecontrol",
		v1beta1.KafkaCRLabelKey: kafkaCluster,
	}
}

// New creates a new reconciler for CC
func New(client client.Client, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

// Reconcile implements the reconcile logic for CC
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", fmt.Sprintf(componentNameTemplate, r.KafkaCluster.Name))

	log.V(1).Info("Reconciling")

	clientPass, err := r.getClientPassword()
	if err != nil {
		return err
	}

	if r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint == "" {
		genErr := generateCCTopic(r.KafkaCluster, r.Client, log.WithName("generateCCTopic"))
		if genErr != nil {
			updateErr := k8sutil.UpdateCRStatus(r.Client, r.KafkaCluster, v1beta1.CruiseControlTopicNotReady, log)
			return errors.Combine(genErr, updateErr)
		}
		statusErr := k8sutil.UpdateCRStatus(r.Client, r.KafkaCluster, v1beta1.CruiseControlTopicReady, log)
		if statusErr != nil {
			return errors.WrapIf(statusErr, "could not update CC topic status")
		}

		if r.KafkaCluster.Status.CruiseControlTopicStatus == v1beta1.CruiseControlTopicReady {
			o := r.service()
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}

			var config *corev1.ConfigMap
			if isBrokerDeletionInProgress(r.KafkaCluster.Status.BrokersState) {
				key := types.NamespacedName{
					Name:      fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name),
					Namespace: r.KafkaCluster.Namespace,
				}
				config = &corev1.ConfigMap{}
				err := r.Client.Get(context.Background(), key, config)
				if err != nil && !apierrors.IsNotFound(err) {
					return errorfactory.New(
						errorfactory.APIFailure{},
						err,
						"getting cruise control configmap failed",
						"name", key.Name,
					)
				}
			}
			capacityConfig, err := GenerateCapacityConfig(r.KafkaCluster, log, config)
			if err != nil {
				return errors.WrapIf(err, "failed to generate capacity config")
			}

			o = r.configMap(clientPass, capacityConfig, log)
			err = k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}

			podAnnotations := GeneratePodAnnotations(
				r.KafkaCluster.Spec.CruiseControlConfig.GetCruiseControlAnnotations(),
				o.(*corev1.ConfigMap).Data,
			)

			o = r.deployment(podAnnotations)
			err = k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}

func (r *Reconciler) getClientPassword() (string, error) {
	var clientPass string
	if r.KafkaCluster.Spec.IsClientSSLSecretPresent() {
		// Use that secret as default which has autogenerated for clients by us
		clientNamespacedName := types.NamespacedName{Name: fmt.Sprintf(pkicommon.BrokerControllerTemplate, r.KafkaCluster.Name), Namespace: r.KafkaCluster.Namespace}
		if r.KafkaCluster.Spec.GetClientSSLCertSecretName() != "" {
			clientNamespacedName = types.NamespacedName{Name: r.KafkaCluster.Spec.GetClientSSLCertSecretName(), Namespace: r.KafkaCluster.Namespace}
		}
		clientSecret := &corev1.Secret{}
		if err := r.Client.Get(context.TODO(), clientNamespacedName, clientSecret); err != nil {
			// We only return with ResourceNotReady (which is goin to retry after period time)
			// when we use our cert generation for client cert
			if apierrors.IsNotFound(err) && r.KafkaCluster.Spec.GetClientSSLCertSecretName() == "" {
				return "", errorfactory.New(errorfactory.ResourceNotReady{}, err, "client secret not ready")
			}
			return "", errors.WrapIfWithDetails(err, "failed to get client secret")
		}
		clientPass = string(clientSecret.Data[v1alpha1.PasswordKey])
	}
	return clientPass, nil
}

func isBrokerDeletionInProgress(brokerState map[string]v1beta1.BrokerState) bool {
	for _, state := range brokerState {
		if state.GracefulActionState.CruiseControlState.IsDownscale() {
			return true
		}
	}
	return false
}
