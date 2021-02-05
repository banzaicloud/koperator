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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
)

func (r *Reconciler) deployment(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {

	var configMapName string
	var deploymentName string
	if ingressConfigName == util.IngressConfigGlobalName {
		configMapName = fmt.Sprintf(envoyVolumeAndConfigName, extListener.Name, r.KafkaCluster.GetName())
		deploymentName = fmt.Sprintf(envoyDeploymentName, extListener.Name, r.KafkaCluster.GetName())
	} else {
		configMapName = fmt.Sprintf(envoyVolumeAndConfigNameWithScope, extListener.Name, ingressConfigName, r.KafkaCluster.GetName())
		deploymentName = fmt.Sprintf(envoyDeploymentNameWithScope, extListener.Name, ingressConfigName, r.KafkaCluster.GetName())
	}

	exposedPorts := getExposedContainerPorts(extListener,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log),
		r.KafkaCluster.Spec, log, ingressConfigName, defaultIngressConfigName)
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
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			deploymentName,
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), annotationName),
			ingressConfig.EnvoyConfig.GetAnnotations(),
			r.KafkaCluster),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForEnvoyIngress(r.KafkaCluster.GetName(), annotationName),
			},
			Replicas: util.Int32Pointer(ingressConfig.EnvoyConfig.GetReplicas()),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForEnvoyIngress(r.KafkaCluster.GetName(), annotationName),
					Annotations: generatePodAnnotations(r.KafkaCluster, extListener, ingressConfigName,
						defaultIngressConfigName, log),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ingressConfig.EnvoyConfig.GetServiceAccount(),
					ImagePullSecrets:   ingressConfig.EnvoyConfig.GetImagePullSecrets(),
					Tolerations:        ingressConfig.EnvoyConfig.GetTolerations(),
					NodeSelector:       ingressConfig.EnvoyConfig.GetNodeSelector(),
					Containers: []corev1.Container{
						{
							Name:  "envoy",
							Image: ingressConfig.EnvoyConfig.GetEnvoyImage(),
							Ports: append(exposedPorts, []corev1.ContainerPort{
								{Name: "envoy-admin", ContainerPort: 9901, Protocol: corev1.ProtocolTCP}}...),
							VolumeMounts: volumeMounts,
							Resources:    *ingressConfig.EnvoyConfig.GetResources(),
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func getExposedContainerPorts(extListener v1beta1.ExternalListenerConfig, brokerIds []int,
	kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger, ingressConfigName, defaultIngressConfigName string) []corev1.ContainerPort {
	var exposedPorts []corev1.ContainerPort

	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kafkaClusterSpec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, defaultIngressConfigName, ingressConfigName) {
			exposedPorts = append(exposedPorts, corev1.ContainerPort{
				Name:          fmt.Sprintf("broker-%d", brokerId),
				ContainerPort: extListener.ExternalStartingPort + int32(brokerId),
				Protocol:      corev1.ProtocolTCP,
			})
		}
	}
	exposedPorts = append(exposedPorts, corev1.ContainerPort{
		Name:          fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, "tcp"),
		ContainerPort: extListener.GetAnyCastPort(),
		Protocol:      corev1.ProtocolTCP,
	})

	return exposedPorts
}

func generatePodAnnotations(kafkaCluster *v1beta1.KafkaCluster,
	extListener v1beta1.ExternalListenerConfig, ingressConfigName, defaultIngressConfigName string,
	log logr.Logger) map[string]string {
	hashedEnvoyConfig := sha256.Sum256([]byte(
		GenerateEnvoyConfig(kafkaCluster, extListener, ingressConfigName, defaultIngressConfigName, log)))
	annotations := map[string]string{
		"envoy.yaml.hash": hex.EncodeToString(hashedEnvoyConfig[:]),
	}
	return annotations
}
