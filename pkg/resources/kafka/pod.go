package kafka

import (
	"fmt"
	"strings"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) pod(broker banzaicloudv1alpha1.BrokerConfig, pvcs []corev1.PersistentVolumeClaim, log logr.Logger) runtime.Object {

	var kafkaBrokerContainerPorts []corev1.ContainerPort

	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(eListener.Name, "_", "-"),
			ContainerPort: eListener.ContainerPort,
		})
	}

	for _, iListener := range r.KafkaCluster.Spec.ListenersConfig.InternalListeners {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(iListener.Name, "_", "-"),
			ContainerPort: iListener.ContainerPort,
		})
	}

	dataVolume, dataVolumeMount := generateDataVolumeAndVolumeMount(pvcs)

	volume := []corev1.Volume{}
	volumeMount := []corev1.VolumeMount{}
	initContainers := []corev1.Container{}
	command := []string{"sh", "-c", "/opt/kafka/bin/kafka-server-start.sh /config/broker-config"}

	volume = append(volume, dataVolume...)
	volumeMount = append(volumeMount, dataVolumeMount...)

	if r.KafkaCluster.Spec.ListenersConfig.TLSSecretName != "" {
		volume = append(volume, generateVolumesForSSL(r.KafkaCluster.Spec.ListenersConfig.TLSSecretName)...)
		volumeMount = append(volumeMount, generateVolumeMountForSSL())
		initContainers = append(initContainers, generateInitContainerForSSL(broker.Image, generatePassword()))
		command = append(command, fmt.Sprintf("--override ssl.keystore.password=%s --override ssl.truststore.password=%s", generatePassword(), generatePassword()))
	}

	return &corev1.Pod{
		ObjectMeta: templates.ObjectMetaWithGeneratedName(r.KafkaCluster.Name, util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", broker.Id)}), r.KafkaCluster),
		Spec: corev1.PodSpec{
			Hostname:  fmt.Sprintf("%s-%d", r.KafkaCluster.Name, broker.Id),
			Subdomain: fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name),
			InitContainers: append(initContainers, []corev1.Container{
				{
					Name:            "cruise-control-reporter",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Image:           "solsson/kafka-cruise-control@sha256:c70eae329b4ececba58e8cf4fa6e774dd2e0205988d8e5be1a70e622fcc46716",
					Command:         []string{"/bin/sh", "-cex", "cp -v /opt/cruise-control/cruise-control/build/dependant-libs/cruise-control-metrics-reporter.jar /opt/kafka/libs/extensions/cruise-control-metrics-reporter.jar"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "extensions",
						MountPath: "/opt/kafka/libs/extensions",
					}},
				},
			}...),
			Containers: []corev1.Container{
				{
					Name:            "kafka",
					Image:           broker.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  "CLASSPATH",
							Value: "/opt/kafka/libs/extensions/*",
						},
					},
					Command: command,
					Ports:   kafkaBrokerContainerPorts,
					VolumeMounts: append(volumeMount, []corev1.VolumeMount{
						{
							Name:      brokerConfigVolumeMount,
							MountPath: "/config",
						},
						{
							Name:      "extensions",
							MountPath: "/opt/kafka/libs/extensions",
						},
					}...),
				},
			},
			Volumes: append(volume, []corev1.Volume{
				{
					Name: brokerConfigVolumeMount,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, broker.Id)},
						},
					},
				},
				{
					Name: "extensions",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}...),
		},
	}
}

func generateDataVolumeAndVolumeMount(pvcs []corev1.PersistentVolumeClaim) (volume []corev1.Volume, volumeMount []corev1.VolumeMount) {
	for i, pvc := range pvcs {
		volume = append(volume, corev1.Volume{
			Name: fmt.Sprintf(kafkaDataVolumeMount + "-%d", i),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		})
		volumeMount = append(volumeMount, corev1.VolumeMount{
			Name:      fmt.Sprintf(kafkaDataVolumeMount + "-%d", i),
			MountPath: pvc.Annotations["mountPath"],
		})
	}
	return
}


func generateInitContainerForSSL(image, keystorePass string) corev1.Container {
	// Keystore generator
	initPemToKeyStore := corev1.Container{}
	// TODO handle error
	initPemToKeyStore = corev1.Container{
		Name:            "pem-to-jks",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Image:           image,
		Command: []string{"/bin/sh", "-c",
			fmt.Sprintf("apk update && apk add openssl && openssl pkcs12 -export -in /var/run/secrets/pemfiles/peerCert -inkey /var/run/secrets/pemfiles/peerKey -out server.p12 -name localhost -CAfile /var/run/secrets/pemfiles/caCert -caname root -chain -password pass:%s &&", keystorePass) +
				fmt.Sprintf("keytool -importkeystore -deststorepass %s -destkeypass %s -destkeystore /var/run/secrets/java.io/keystores/kafka.server.keystore.jks -srckeystore server.p12 -srcstoretype PKCS12 -srcstorepass %s -alias localhost &&", keystorePass, keystorePass, keystorePass) +
				fmt.Sprintf("keytool -keystore /var/run/secrets/java.io/keystores/kafka.server.truststore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass %s -noprompt && ", keystorePass) +
				fmt.Sprintf("keytool -keystore /var/run/secrets/java.io/keystores/kafka.server.keystore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass %s -noprompt", keystorePass)},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      keystoreVolume,
				MountPath: keystoreVolumePath,
			},
			{
				Name:      pemFilesVolume,
				MountPath: "/var/run/secrets/pemfiles",
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
					SecretName: tlsSecretName,
				},
			},
		},
	}
}

func generateVolumeMountForSSL() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      keystoreVolume,
		MountPath: keystoreVolumePath,
	}
}
