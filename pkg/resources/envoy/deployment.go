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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) deployment(log logr.Logger, extListener v1beta1.ExternalListenerConfig) runtime.Object {

	configMapName := fmt.Sprintf(envoyVolumeAndConfigName, extListener.Name, r.KafkaCluster.GetName())
	exposedPorts := getExposedContainerPorts(extListener,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log))
	volumes := []corev1.Volume{
		{
			Name: configMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      configMapName,
			MountPath: "/etc/envoy",
			ReadOnly:  true,
		},
	}

	return &appsv1.Deployment{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(envoyDeploymentName, extListener.Name, r.KafkaCluster.GetName()),
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), extListener.Name), r.KafkaCluster),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForEnvoyIngress(r.KafkaCluster.GetName(), extListener.Name),
			},
			Replicas: util.Int32Pointer(r.KafkaCluster.Spec.EnvoyConfig.GetReplicas()),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labelsForEnvoyIngress(r.KafkaCluster.GetName(), extListener.Name),
					Annotations: generatePodAnnotations(r.KafkaCluster, extListener, log),
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

func getExposedContainerPorts(extListener v1beta1.ExternalListenerConfig, brokerIds []int) []corev1.ContainerPort {
	var exposedPorts []corev1.ContainerPort

	for _, id := range brokerIds {
		exposedPorts = append(exposedPorts, corev1.ContainerPort{
			Name:          fmt.Sprintf("broker-%d", id),
			ContainerPort: extListener.ExternalStartingPort + int32(id),
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return exposedPorts
}

func generatePodAnnotations(kafkaCluster *v1beta1.KafkaCluster,
	extListener v1beta1.ExternalListenerConfig, log logr.Logger) map[string]string {
	hashedEnvoyConfig := sha256.Sum256([]byte(GenerateEnvoyConfig(kafkaCluster, extListener, log)))
	annotations := map[string]string{
		"envoy.yaml.hash": hex.EncodeToString(hashedEnvoyConfig[:]),
	}
	return annotations
}
