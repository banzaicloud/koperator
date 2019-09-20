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
	"fmt"

	"emperror.dev/errors"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentNameTemplate       = "%s-cruisecontrol"
	serviceNameTemplate         = "%s-cruisecontrol-svc"
	configAndVolumeNameTemplate = "%s-cruisecontrol-config"
	modconfigAndVolumeName      = "cruisecontrol-modconfig"
	deploymentNameTemplate      = "%s-cruisecontrol"
	keystoreVolume              = "ks-files"
	keystoreVolumePath          = "/var/run/secrets/java.io/keystores"
	pemFilesVolume              = "pem-files"
	jmxVolumePath               = "/opt/jmx-exporter/"
	jmxVolumeName               = "jmx-jar-data"
	metricsPort                 = 9020
)

var labelSelector = map[string]string{
	"app": "cruisecontrol",
}

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// New creates a new reconciler for CC
func New(client client.Client, cluster *banzaicloudv1alpha1.KafkaCluster) *Reconciler {
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

	if r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint == "" {

		if r.KafkaCluster.Status.CruiseControlTopicStatus == "" || r.KafkaCluster.Status.CruiseControlTopicStatus == banzaicloudv1alpha1.CruiseControlTopicNotReady {
			err := generateCCTopic(r.KafkaCluster, r.Client, log)
			if err != nil {
				k8sutil.UpdateCRStatus(r.Client, r.KafkaCluster, banzaicloudv1alpha1.CruiseControlTopicNotReady, log)
				return err
			}
			statusErr := k8sutil.UpdateCRStatus(r.Client, r.KafkaCluster, banzaicloudv1alpha1.CruiseControlTopicReady, log)
			if statusErr != nil {
				return errors.WrapIf(statusErr, "could not update CC topic status")
			}
		}

		if r.KafkaCluster.Status.CruiseControlTopicStatus == banzaicloudv1alpha1.CruiseControlTopicReady {
			for _, res := range []resources.ResourceWithLogs{
				r.service,
				r.configMap,
				r.deployment,
			} {
				o := res(log)
				err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
				if err != nil {
					return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
				}
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}
