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

	"github.com/banzaicloud/kafka-operator/pkg/resources/cruisecontrolmonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) deployment(log logr.Logger) runtime.Object {

	volume := []corev1.Volume{}
	volumeMount := []corev1.VolumeMount{}
	initContainers := []corev1.Container{}

	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners) {
		volume = append(volume, generateVolumesForSSL(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName)...)
		volumeMount = append(volumeMount, generateVolumeMountForSSL()...)
		initContainers = append(initContainers, generateInitContainerForSSL(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.JKSPasswordName, r.KafkaCluster.Spec.BrokerConfigs[0].Image, r.KafkaCluster.Name))
	} else {
		volumeMount = append(volumeMount, []corev1.VolumeMount{
			{
				Name:      fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name),
				MountPath: "/opt/cruise-control/config",
			},
		}...)
	}

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
					ServiceAccountName:            r.KafkaCluster.Spec.GetServiceAccount(),
					ImagePullSecrets:              r.KafkaCluster.Spec.GetImagePullSecrets(),
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("512Mi"),
								},
							},
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

func generateInitContainerForSSL(secretName, image, clusterName string) corev1.Container {
	// Keystore generator
	initPemToKeyStore := corev1.Container{
		Name:  "pem-to-jks",
		Image: image,
		Env: []corev1.EnvVar{
			{
				Name: "SSL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "password",
					},
				},
			},
		},
		Command: []string{"bash", "-c", `
		apk update && apk add openssl && openssl pkcs12 -export -in /var/run/secrets/pemfiles/clientCert -inkey /var/run/secrets/pemfiles/clientKey -out client.p12 -name localhost -CAfile /var/run/secrets/pemfiles/caCert -caname root -chain -password pass:${SSL_PASSWORD} &&
		keytool -importkeystore -deststorepass ${SSL_PASSWORD} -destkeypass ${SSL_PASSWORD} -destkeystore /var/run/secrets/java.io/keystores/client.keystore.jks -srckeystore client.p12 -srcstoretype PKCS12 -srcstorepass ${SSL_PASSWORD} -alias localhost &&
		keytool -keystore /var/run/secrets/java.io/keystores/client.truststore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass ${SSL_PASSWORD} -noprompt &&
		keytool -keystore /var/run/secrets/java.io/keystores/client.keystore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass ${SSL_PASSWORD} -noprompt &&
		cat /config/cruisecontrol.properties > /opt/cruise-control/config/cruisecontrol.properties && cat /config/capacity.json > /opt/cruise-control/config/capacity.json &&
		cat /config/clusterConfigs.json > /opt/cruise-control/config/clusterConfigs.json && cat /config/log4j.properties > /opt/cruise-control/config/log4j.properties && cat /config/log4j2.xml > /opt/cruise-control/config/log4j2.xml &&
		echo "ssl.keystore.password=${SSL_PASSWORD}" >> /opt/cruise-control/config/cruisecontrol.properties && echo "ssl.truststore.password=${SSL_PASSWORD}" >> /opt/cruise-control/config/cruisecontrol.properties
		`,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      keystoreVolume,
				MountPath: keystoreVolumePath,
			},
			{
				Name:      pemFilesVolume,
				MountPath: "/var/run/secrets/pemfiles",
			},
			{
				Name:      fmt.Sprintf(configAndVolumeNameTemplate, clusterName),
				MountPath: "/config",
			},
			{
				Name:      modconfigAndVolumeName,
				MountPath: "/opt/cruise-control/config",
			},
		},
	}
	return initPemToKeyStore
}

func generateVolumesForSSL(tlsSecretName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: keystoreVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: pemFilesVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  tlsSecretName,
					DefaultMode: util.Int32Pointer(0644),
				},
			},
		},
		{
			Name: modconfigAndVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
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
		{
			Name:      modconfigAndVolumeName,
			MountPath: "/opt/cruise-control/config",
		},
	}
}
