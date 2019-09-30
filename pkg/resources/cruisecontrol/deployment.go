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

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrolmonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) deployment(log logr.Logger, clientPass string) runtime.Object {

	volume := []corev1.Volume{}
	volumeMount := []corev1.VolumeMount{}
	initContainers := []corev1.Container{}

	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners) {
		volume = append(volume, generateVolumesForSSL(r.KafkaCluster)...)
		volumeMount = append(volumeMount, generateVolumeMountForSSL()...)
	}
	volumeMount = append(volumeMount, []corev1.VolumeMount{
		{
			Name:      fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name),
			MountPath: "/opt/cruise-control/config",
		},
	}...)

	return &appsv1.Deployment{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(deploymentNameTemplate, r.KafkaCluster.Name), labelSelector, r.KafkaCluster),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelSelector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labelSelector,
					Annotations: util.MonitoringAnnotations(metricsPort),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            r.KafkaCluster.Spec.CruiseControlConfig.GetServiceAccount(),
					ImagePullSecrets:              r.KafkaCluster.Spec.CruiseControlConfig.GetImagePullSecrets(),
					Tolerations:                   r.KafkaCluster.Spec.CruiseControlConfig.GetTolerations(),
					NodeSelector:                  r.KafkaCluster.Spec.CruiseControlConfig.GetNodeSelector(),
					TerminationGracePeriodSeconds: util.Int64Pointer(30),
					InitContainers: append(initContainers, []corev1.Container{
						{
							Name:    "jmx-exporter",
							Image:   r.KafkaCluster.Spec.MonitoringConfig.GetImage(),
							Command: []string{"cp", r.KafkaCluster.Spec.MonitoringConfig.GetPathToJar(), "/opt/jmx-exporter/jmx_prometheus.jar"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      jmxVolumeName,
									MountPath: jmxVolumePath,
								},
							},
						},
					}...),
					Containers: []corev1.Container{
						{
							Name: fmt.Sprintf(deploymentNameTemplate, r.KafkaCluster.Name),
							Env: []corev1.EnvVar{
								{
									Name:  "KAFKA_OPTS",
									Value: "-javaagent:/opt/jmx-exporter/jmx_prometheus.jar=9020:/etc/jmx-exporter/config.yaml",
								},
							},
							Image: r.KafkaCluster.Spec.CruiseControlConfig.GetCCImage(),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8090,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: 9020,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: *r.KafkaCluster.Spec.CruiseControlConfig.GetResources(),
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: *util.IntstrPointer(8090),
									},
								},
								TimeoutSeconds:      int32(1),
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
								SuccessThreshold:    1,
							},
							VolumeMounts: append(volumeMount, []corev1.VolumeMount{
								{
									Name:      jmxVolumeName,
									MountPath: jmxVolumePath,
								},
								{
									Name:      fmt.Sprintf(cruisecontrolmonitoring.CruiseControlJmxTemplate, r.KafkaCluster.Name),
									MountPath: "/etc/jmx-exporter/",
								},
							}...),
						},
					},
					Volumes: append(volume, []corev1.Volume{
						{
							Name: fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name),
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name)},
									DefaultMode:          util.Int32Pointer(0644),
								},
							},
						},
						{
							Name: fmt.Sprintf(cruisecontrolmonitoring.CruiseControlJmxTemplate, r.KafkaCluster.Name),
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(cruisecontrolmonitoring.CruiseControlJmxTemplate, r.KafkaCluster.Name)},
									DefaultMode:          util.Int32Pointer(0644),
								},
							},
						},
						{
							Name: jmxVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					}...),
				},
			},
		},
	}
}

func generateVolumesForSSL(cluster *v1beta1.KafkaCluster) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: keystoreVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  fmt.Sprintf(pkicommon.BrokerControllerTemplate, cluster.Name),
					DefaultMode: util.Int32Pointer(0644),
				},
			},
		},
	}
}

func generateVolumeMountForSSL() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      keystoreVolume,
			MountPath: keystoreVolumePath,
		},
	}
}
