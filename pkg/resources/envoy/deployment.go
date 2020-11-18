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

package envoy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) deployment(log logr.Logger) runtime.Object {

	exposedPorts := getExposedContainerPorts(r.KafkaCluster.Spec.ListenersConfig.ExternalListeners,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log))
	volumes := []corev1.Volume{
		{
			Name: envoyVolumeAndConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: envoyVolumeAndConfigName},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      envoyVolumeAndConfigName,
			MountPath: "/etc/envoy",
			ReadOnly:  true,
		},
	}

	return &appsv1.Deployment{
		ObjectMeta: templates.ObjectMeta(envoyDeploymentName, labelSelector, r.KafkaCluster),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelSelector,
			},
			Replicas: util.Int32Pointer(r.KafkaCluster.Spec.EnvoyConfig.GetReplicas()),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labelSelector,
					Annotations: generatePodAnnotations(r.KafkaCluster, log),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: r.KafkaCluster.Spec.EnvoyConfig.GetServiceAccount(),
					ImagePullSecrets:   r.KafkaCluster.Spec.EnvoyConfig.GetImagePullSecrets(),
					Tolerations:        r.KafkaCluster.Spec.EnvoyConfig.GetTolerations(),
					NodeSelector:       r.KafkaCluster.Spec.EnvoyConfig.GetNodeSelector(),
					Containers: []corev1.Container{
						{
							Name:  "envoy",
							Image: r.KafkaCluster.Spec.EnvoyConfig.GetEnvoyImage(),
							Ports: append(exposedPorts, []corev1.ContainerPort{
								{Name: "envoy-admin", ContainerPort: 9901, Protocol: corev1.ProtocolTCP}}...),
							VolumeMounts: volumeMounts,
							Resources:    *r.KafkaCluster.Spec.EnvoyConfig.GetResources(),
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func getExposedContainerPorts(extListeners []v1beta1.ExternalListenerConfig, brokerIds []int) []corev1.ContainerPort {
	var exposedPorts []corev1.ContainerPort

	for _, eListener := range extListeners {
		for _, id := range brokerIds {
			exposedPorts = append(exposedPorts, corev1.ContainerPort{
				Name:          fmt.Sprintf("broker-%d", id),
				ContainerPort: eListener.ExternalStartingPort + int32(id),
				Protocol:      corev1.ProtocolTCP,
			})
		}
	}
	return exposedPorts
}

func generatePodAnnotations(kafkaCluster *v1beta1.KafkaCluster, log logr.Logger) map[string]string {
	hashedEnvoyConfig := sha256.Sum256([]byte(GenerateEnvoyConfig(kafkaCluster, log)))
	annotations := map[string]string{
		"envoy.yaml.hash": hex.EncodeToString(hashedEnvoyConfig[:]),
	}
	return annotations
}
