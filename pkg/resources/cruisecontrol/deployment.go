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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources/cruisecontrolmonitoring"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"
)

func (r *Reconciler) deployment(podAnnotations map[string]string) runtime.Object {
	volume := make([]corev1.Volume, 0)
	volumeMount := make([]corev1.VolumeMount, 0)
	initContainers := make([]corev1.Container, 0)

	initContainers = append(initContainers, r.KafkaCluster.Spec.CruiseControlConfig.InitContainers...)
	volume = append(volume, r.KafkaCluster.Spec.CruiseControlConfig.Volumes...)
	volumeMount = append(volumeMount, r.KafkaCluster.Spec.CruiseControlConfig.VolumeMounts...)

	if r.KafkaCluster.Spec.ListenersConfig.IsClientSSLSecretPresent() && util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners) {
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
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(deploymentNameTemplate, r.KafkaCluster.Name),
			templates.ObjectMetaLabels(r.KafkaCluster, ccLabelSelector(r.KafkaCluster.Name)),
			r.KafkaCluster,
		),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ccLabelSelector(r.KafkaCluster.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      templates.ObjectMetaLabels(r.KafkaCluster, ccLabelSelector(r.KafkaCluster.Name)),
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					SecurityContext:               r.KafkaCluster.Spec.CruiseControlConfig.PodSecurityContext,
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
							Resources: k8sutil.GetDefaultInitContainerResourceRequirements(),
						},
					}...),
					Containers: []corev1.Container{
						{
							Name:            fmt.Sprintf(deploymentNameTemplate, r.KafkaCluster.Name),
							SecurityContext: r.KafkaCluster.Spec.CruiseControlConfig.SecurityContext,
							Env: []corev1.EnvVar{
								{
									Name:  "KAFKA_OPTS",
									Value: "-javaagent:/opt/jmx-exporter/jmx_prometheus.jar=9020:/etc/jmx-exporter/config.yaml",
								},
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"bash", "-c", `
SIGNAL=${SIGNAL:-TERM}
PIDS=$(ps ax | grep -i 'cruise-control' | grep java | grep -v grep | awk '{print $1}')
if [ -z "$PIDS" ]; then
  echo "No cruise-control to stop"
  exit 1
else
  kill -s $SIGNAL $PIDS
fi`},
									},
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

func GeneratePodAnnotations(kafkaCluster *v1beta1.KafkaCluster, capacityConfig string) map[string]string {
	ccAnnotationsFromCR := kafkaCluster.Spec.CruiseControlConfig.GetCruiseControlAnnotations()
	hashedCruiseControlConfigJson := sha256.Sum256([]byte(kafkaCluster.Spec.CruiseControlConfig.Config))
	hashedCruiseControlClusterConfigJson := sha256.Sum256([]byte(kafkaCluster.Spec.CruiseControlConfig.ClusterConfig))
	hashedCruiseControlLogConfigJson := sha256.Sum256([]byte(kafkaCluster.Spec.CruiseControlConfig.GetCCLog4jConfig()))

	annotations := []map[string]string{
		{
			"cruiseControlConfig.json":        hex.EncodeToString(hashedCruiseControlConfigJson[:]),
			"cruiseControlClusterConfig.json": hex.EncodeToString(hashedCruiseControlClusterConfigJson[:]),
			"cruiseControlLogConfig.json":     hex.EncodeToString(hashedCruiseControlLogConfigJson[:]),
		},
		ccAnnotationsFromCR,
	}
	if value, ok := ccAnnotationsFromCR[capacityConfigAnnotation]; !ok ||
		value == string(staticCapacityConfig) {
		hashedCruiseControlCapacityJson := sha256.Sum256([]byte(capacityConfig))
		annotations = append(annotations,
			map[string]string{"cruiseControlCapacity.json": hex.EncodeToString(hashedCruiseControlCapacityJson[:])})
	}

	return util.MergeAnnotations(annotations...)
}

func generateVolumesForSSL(cluster *v1beta1.KafkaCluster) []corev1.Volume {
	secretName := fmt.Sprintf(pkicommon.BrokerControllerTemplate, cluster.Name)
	if cluster.Spec.ListenersConfig.GetClientSSLCertSecretName() != "" {
		secretName = cluster.Spec.ListenersConfig.GetClientSSLCertSecretName()
	}
	return []corev1.Volume{
		{
			Name: keystoreVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretName,
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
