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

package kafka

import (
	"fmt"
	"strings"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafkamonitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) pod(id int32, brokerConfig *v1beta1.BrokerConfig, pvcs []corev1.PersistentVolumeClaim, log logr.Logger) runtime.Object {

	var kafkaBrokerContainerPorts []corev1.ContainerPort

	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(eListener.Name, "_", "-"),
			ContainerPort: eListener.ContainerPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	for _, iListener := range r.KafkaCluster.Spec.ListenersConfig.InternalListeners {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(iListener.Name, "_", "-"),
			ContainerPort: iListener.ContainerPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	dataVolume, dataVolumeMount := generateDataVolumeAndVolumeMount(pvcs)

	volume := []corev1.Volume{}
	volumeMount := []corev1.VolumeMount{}
	initContainers := []corev1.Container{}
	command := []string{"bash", "-c"}

	volume = append(volume, dataVolume...)
	volumeMount = append(volumeMount, dataVolumeMount...)

	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil {
		volume = append(volume, generateVolumesForSSL(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName)...)
		volumeMount = append(volumeMount, generateVolumeMountForSSL()...)
		initContainers = append(initContainers, generateInitContainerForSSL(util.GetBrokerImage(brokerConfig, r.KafkaCluster.Spec.ClusterImage), r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.JKSPasswordName, r.KafkaCluster.Spec.ListenersConfig.InternalListeners))
		command = append(command, "/opt/kafka/bin/kafka-server-start.sh /mod-config/broker-config")
	} else {
		command = append(command, "/opt/kafka/bin/kafka-server-start.sh /config/broker-config")
	}

	pod := &corev1.Pod{
		ObjectMeta: templates.ObjectMetaWithGeneratedNameAndAnnotations(r.KafkaCluster.Name, util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", id)}), util.MergeAnnotations(brokerConfig.GetBrokerAnnotations(), util.MonitoringAnnotations(metricsPort)), r.KafkaCluster),
		Spec: corev1.PodSpec{
			InitContainers: append(initContainers, []corev1.Container{
				{
					Name:    "cruise-control-reporter",
					Image:   r.KafkaCluster.Spec.CruiseControlConfig.GetCCImage(),
					Command: []string{"/bin/sh", "-cex", "cp -v /opt/cruise-control/cruise-control/build/dependant-libs/cruise-control-metrics-reporter.jar /opt/kafka/libs/extensions/cruise-control-metrics-reporter.jar"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "extensions",
						MountPath: "/opt/kafka/libs/extensions",
					}},
				},
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
			Affinity: &corev1.Affinity{
				PodAntiAffinity: generatePodAntiAffinity(r.KafkaCluster.Name, r.KafkaCluster.Spec.OneBrokerPerNode),
			},
			Containers: []corev1.Container{
				{
					Name:  "kafka",
					Image: util.GetBrokerImage(brokerConfig, r.KafkaCluster.Spec.ClusterImage),
					Lifecycle: &corev1.Lifecycle{
						PreStop: &corev1.Handler{
							Exec: &corev1.ExecAction{
								Command: []string{"bash", "-c", "kill -s TERM 1"},
							},
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "CLASSPATH",
							Value: "/opt/kafka/libs/extensions/*",
						},
						{
							Name:  "KAFKA_OPTS",
							Value: "-javaagent:/opt/jmx-exporter/jmx_prometheus.jar=9020:/etc/jmx-exporter/config.yaml",
						},
						{
							Name:  "KAFKA_HEAP_OPTS",
							Value: brokerConfig.GetKafkaHeapOpts(),
						},
						{
							Name:  "KAFKA_JVM_PERFORMANCE_OPTS",
							Value: brokerConfig.GetKafkaPerfJmvOpts(),
						},
					},
					Command: command,
					Ports: append(kafkaBrokerContainerPorts, []corev1.ContainerPort{
						{
							ContainerPort: 9020,
							Protocol:      corev1.ProtocolTCP,
							Name:          "metrics",
						},
					}...),
					VolumeMounts: append(volumeMount, []corev1.VolumeMount{
						{
							Name:      brokerConfigMapVolumeMount,
							MountPath: "/config",
						},
						{
							Name:      "extensions",
							MountPath: "/opt/kafka/libs/extensions",
						},
						{
							Name:      jmxVolumeName,
							MountPath: jmxVolumePath,
						},
						{
							Name:      fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, r.KafkaCluster.Name),
							MountPath: "/etc/jmx-exporter/",
						},
					}...),
					Resources: *brokerConfig.GetResources(),
				},
			},
			Volumes: append(volume, []corev1.Volume{
				{
					Name: brokerConfigMapVolumeMount,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id)},
							DefaultMode:          util.Int32Pointer(0644),
						},
					},
				},
				{
					Name: "extensions",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, r.KafkaCluster.Name),
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, r.KafkaCluster.Name)},
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
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64Pointer(120),
			DNSPolicy:                     corev1.DNSClusterFirst,
			ImagePullSecrets:              brokerConfig.GetImagePullSecrets(),
			ServiceAccountName:            brokerConfig.GetServiceAccount(),
			SecurityContext:               &corev1.PodSecurityContext{},
			Priority:                      util.Int32Pointer(0),
			SchedulerName:                 "default-scheduler",
			Tolerations:                   brokerConfig.GetTolerations(),
			NodeSelector:                  brokerConfig.GetNodeSelector(),
		},
	}
	if r.KafkaCluster.Spec.HeadlessServiceEnabled {
		pod.Spec.Hostname = fmt.Sprintf("%s-%d", r.KafkaCluster.Name, id)
		pod.Spec.Subdomain = fmt.Sprintf(kafkautils.HeadlessServiceTemplate, r.KafkaCluster.Name)
	}
	if brokerConfig.NodeAffinity != nil {
		pod.Spec.Affinity.NodeAffinity = brokerConfig.NodeAffinity
	}
	return pod
}

func generatePodAntiAffinity(clusterName string, hardRuleEnabled bool) *corev1.PodAntiAffinity {
	podAntiAffinity := corev1.PodAntiAffinity{}
	if hardRuleEnabled {
		podAntiAffinity = corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: labelsForKafka(clusterName),
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		}
	} else {
		podAntiAffinity = corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: int32(100),
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labelsForKafka(clusterName),
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
	}
	return &podAntiAffinity
}

func generateDataVolumeAndVolumeMount(pvcs []corev1.PersistentVolumeClaim) (volume []corev1.Volume, volumeMount []corev1.VolumeMount) {
	for i, pvc := range pvcs {
		volume = append(volume, corev1.Volume{
			Name: fmt.Sprintf(kafkaDataVolumeMount+"-%d", i),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		})
		volumeMount = append(volumeMount, corev1.VolumeMount{
			Name:      fmt.Sprintf(kafkaDataVolumeMount+"-%d", i),
			MountPath: pvc.Annotations["mountPath"],
		})
	}
	return
}

func generateInitContainerForSSL(image, secretName string, l []v1beta1.InternalListenerConfig) corev1.Container {

	genScript := `apk update && apk add openssl && openssl pkcs12 -export -in /var/run/secrets/pemfiles/peerCert -inkey /var/run/secrets/pemfiles/peerKey -out server.p12 -name localhost -CAfile /var/run/secrets/pemfiles/caCert -caname root -chain -password pass:${SSL_PASSWORD} &&
keytool -importkeystore -deststorepass ${SSL_PASSWORD} -destkeypass ${SSL_PASSWORD} -destkeystore /var/run/secrets/java.io/keystores/kafka.server.keystore.jks -srckeystore server.p12 -srcstoretype PKCS12 -srcstorepass ${SSL_PASSWORD} -alias localhost &&
keytool -keystore /var/run/secrets/java.io/keystores/kafka.server.truststore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass ${SSL_PASSWORD} -noprompt &&
keytool -keystore /var/run/secrets/java.io/keystores/kafka.server.keystore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass ${SSL_PASSWORD} -noprompt &&
openssl pkcs12 -export -in /var/run/secrets/pemfiles/clientCert -inkey /var/run/secrets/pemfiles/clientKey -out client.p12 -name localhost -CAfile /var/run/secrets/pemfiles/caCert -caname root -chain -password pass:${SSL_PASSWORD} &&
keytool -importkeystore -deststorepass ${SSL_PASSWORD} -destkeypass ${SSL_PASSWORD} -destkeystore /var/run/secrets/java.io/keystores/client.keystore.jks -srckeystore client.p12 -srcstoretype PKCS12 -srcstorepass ${SSL_PASSWORD} -alias localhost &&
keytool -keystore /var/run/secrets/java.io/keystores/client.truststore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass ${SSL_PASSWORD} -noprompt &&
keytool -keystore /var/run/secrets/java.io/keystores/client.keystore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass ${SSL_PASSWORD} -noprompt &&
cat /config/broker-config > /mod-config/broker-config && echo -e "\nssl.keystore.password=${SSL_PASSWORD}" >> /mod-config/broker-config && echo -e "\nssl.truststore.password=${SSL_PASSWORD}" >> /mod-config/broker-config`

	if util.IsSSLEnabledForInternalCommunication(l) {
		genScript = genScript + ` && 
echo "cruise.control.metrics.reporter.ssl.keystore.password=${SSL_PASSWORD}" >> /mod-config/broker-config && echo "cruise.control.metrics.reporter.truststore.password=${SSL_PASSWORD}" >> /mod-config/broker-config
`
	}
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
		Command: []string{"bash", "-c", genScript},
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
				Name:      brokerConfigMapVolumeMount,
				MountPath: "/config",
			},
			{
				Name:      modbrokerConfigMapVolumeMount,
				MountPath: "/mod-config",
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
			Name: modbrokerConfigMapVolumeMount,
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
			Name:      modbrokerConfigMapVolumeMount,
			MountPath: "/mod-config",
		},
	}
}
