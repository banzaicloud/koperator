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
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

func (r *Reconciler) deployment(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, extListener.Name)

	var configMapName string = util.GenerateEnvoyResourceName(envoyutils.EnvoyVolumeAndConfigName, envoyutils.EnvoyVolumeAndConfigNameWithScope,
		extListener, ingressConfig, ingressConfigName, r.KafkaCluster.GetName())
	var deploymentName string = util.GenerateEnvoyResourceName(envoyutils.EnvoyDeploymentName, envoyutils.EnvoyDeploymentNameWithScope,
		extListener, ingressConfig, ingressConfigName, r.KafkaCluster.GetName())

	exposedPorts := getExposedContainerPorts(extListener,
		util.GetBrokerIdsFromStatusAndSpec(r.KafkaCluster.Status.BrokersState, r.KafkaCluster.Spec.Brokers, log),
		r.KafkaCluster, log, ingressConfigName, defaultIngressConfigName)
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

	arguments := []string{"-c", "/etc/envoy/envoy.yaml"}
	if ingressConfig.EnvoyConfig.GetConcurrency() > 0 {
		arguments = append(arguments, "--concurrency", strconv.Itoa(int(ingressConfig.EnvoyConfig.GetConcurrency())))
	}

	return &appsv1.Deployment{
		ObjectMeta: templates.ObjectMetaWithAnnotations(
			deploymentName,
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName),
			ingressConfig.EnvoyConfig.GetAnnotations(),
			r.KafkaCluster),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName),
			},
			Replicas: util.Int32Pointer(ingressConfig.EnvoyConfig.GetReplicas()),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: templates.ObjectMetaLabels(r.KafkaCluster, labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName)),
					Annotations: generatePodAnnotations(r.KafkaCluster, extListener, ingressConfig, ingressConfigName,
						defaultIngressConfigName, log),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:        ingressConfig.EnvoyConfig.GetServiceAccount(),
					ImagePullSecrets:          ingressConfig.EnvoyConfig.GetImagePullSecrets(),
					Tolerations:               ingressConfig.EnvoyConfig.GetTolerations(),
					NodeSelector:              ingressConfig.EnvoyConfig.GetNodeSelector(),
					Affinity:                  ingressConfig.EnvoyConfig.GetAffinity(),
					TopologySpreadConstraints: ingressConfig.EnvoyConfig.GetTopologySpreadConstaints(),
					Containers: []corev1.Container{
						{
							Name:  "envoy",
							Image: ingressConfig.EnvoyConfig.GetEnvoyImage(),
							Args:  arguments,
							Ports: append(exposedPorts,
								[]corev1.ContainerPort{
									{
										Name:          "tcp-admin",
										ContainerPort: ingressConfig.EnvoyConfig.GetEnvoyAdminPort(),
										Protocol:      corev1.ProtocolTCP,
									},
									{
										Name:          "tcp-health",
										ContainerPort: ingressConfig.EnvoyConfig.GetEnvoyHealthCheckPort(),
										Protocol:      corev1.ProtocolTCP,
									},
								}...),
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
	kafkaCluster *v1beta1.KafkaCluster, log logr.Logger, ingressConfigName, defaultIngressConfigName string) []corev1.ContainerPort {
	var exposedPorts []corev1.ContainerPort

	for _, brokerId := range brokerIds {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kafkaCluster.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kafkaCluster.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
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
	extListener v1beta1.ExternalListenerConfig, ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string,
	log logr.Logger) map[string]string {
	hashedEnvoyConfig := sha256.Sum256([]byte(
		GenerateEnvoyConfig(kafkaCluster, extListener, ingressConfig, ingressConfigName, defaultIngressConfigName, log)))
	annotations := map[string]string{
		"envoy.yaml.hash": hex.EncodeToString(hashedEnvoyConfig[:]),
	}
	return util.MergeAnnotations(ingressConfig.EnvoyConfig.GetAnnotations(), annotations)
}
