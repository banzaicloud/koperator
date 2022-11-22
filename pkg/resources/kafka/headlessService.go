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

package kafka

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiutil "github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

func (r *Reconciler) headlessService() runtime.Object {
	var usedPorts []corev1.ServicePort
	// Append internal listener ports
	usedPorts = append(usedPorts,
		generateServicePortForIListeners(r.KafkaCluster.Spec.ListenersConfig.InternalListeners)...)

	//Append external listener ports as well to allow using this service for metadata fetch
	usedPorts = append(usedPorts,
		generateServicePortForEListeners(r.KafkaCluster.Spec.ListenersConfig.ExternalListeners)...)

	// prometheus metrics port for servicemonitor
	usedPorts = append(usedPorts, corev1.ServicePort{
		Name:       "metrics",
		Port:       MetricsPort,
		TargetPort: intstr.FromInt(MetricsPort),
		Protocol:   corev1.ProtocolTCP,
	})

	return &corev1.Service{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			fmt.Sprintf(kafkautils.HeadlessServiceTemplate, r.KafkaCluster.GetName()),
			apiutil.MergeLabels(apiutil.LabelsForKafka(r.KafkaCluster.GetName()), r.KafkaCluster.GetLabels()),
			r.KafkaCluster.Spec.ListenersConfig.GetServiceAnnotations(),
			r.KafkaCluster,
		),
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			SessionAffinity:          corev1.ServiceAffinityNone,
			Selector:                 apiutil.LabelsForKafka(r.KafkaCluster.Name),
			Ports:                    usedPorts,
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
		},
	}
}

// deleteHeadlessService deletes the headless service that was created for the current KafkaCluster
// if there is any
func (r *Reconciler) deleteHeadlessService() error {
	ctx := context.Background()

	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.KafkaCluster.GetNamespace(),
			Name:      fmt.Sprintf(kafkautils.HeadlessServiceTemplate, r.KafkaCluster.GetName()),
		},
	}

	err := r.Client.Delete(ctx, &svc)
	if err != nil {
		err = client.IgnoreNotFound(err)
	}

	return err
}
