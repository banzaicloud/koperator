package cruisecontrol

import (
	"fmt"
	"strings"

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

	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && isSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners){
		volume = append(volume, generateVolumesForSSL(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName)...)
		volumeMount = append(volumeMount, generateVolumeMountForSSL()...)
		initContainers = append(initContainers, generateInitContainerForSSL(r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.JKSPasswordName))
	} else {
		volumeMount = append(volumeMount, []corev1.VolumeMount{
			{
				Name:      configAndVolumeName,
				MountPath: "/opt/cruise-control/config",
			},
		}...)
	}

	return &appsv1.Deployment{
		ObjectMeta: templates.ObjectMeta(deploymentName, labelSelector, r.KafkaCluster),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelSelector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelSelector,
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: util.Int64Pointer(30),
					InitContainers: append(initContainers, []corev1.Container{
						{
							Name:            "create-topic",
							Image:           "wurstmeister/kafka:2.12-2.1.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/bash", "-c", fmt.Sprintf("until /opt/kafka/bin/kafka-topics.sh --zookeeper %s --create --if-not-exists --topic __CruiseControlMetrics --partitions 12 --replication-factor 3; do echo waiting for kafka; sleep 3; done ;", strings.Join(r.KafkaCluster.Spec.ZKAddresses, ","))},
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					}...),
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           "solsson/kafka-cruise-control@sha256:d5e05c95d6e8fddc3e607ec3cdfa2a113b76eabca4aefe6c382f5b3d7d990505",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8090,
									Protocol: corev1.ProtocolTCP,
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
								TimeoutSeconds: int32(1),
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
								SuccessThreshold:    1,
							},
							VolumeMounts: volumeMount,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
					Volumes: append(volume, []corev1.Volume{
						{
							Name: configAndVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configAndVolumeName},
									DefaultMode: util.Int32Pointer(0644),
								},
							},
						},
					}...),
				},
			},
		},
	}
}

func generateInitContainerForSSL(secretName string) corev1.Container {
	// Keystore generator
	initPemToKeyStore := corev1.Container{}
	initPemToKeyStore = corev1.Container{
		Name:            "pem-to-jks",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Image:           "wurstmeister/kafka:2.12-2.1.0",
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
				Name:      configAndVolumeName,
				MountPath: "/config",
			},
			{
				Name:      modconfigAndVolumeName,
				MountPath: "/opt/cruise-control/config",
			},
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
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
