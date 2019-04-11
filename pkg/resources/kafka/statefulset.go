package kafka

import (
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/resources/monitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/sethvargo/go-password/password"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) statefulSet(loadBalancerIP string) runtime.Object {
	ls := labelsForKafka(r.KafkaCluster.Name)
	var kafkaBrokerContainerPorts []corev1.ContainerPort
	var advertisedListenerConfig []string

	for _, eListener := range r.KafkaCluster.Spec.Listeners.ExternalListener {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(eListener.Name, "_", "-"),
			ContainerPort: eListener.ContainerPort,
		})
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s:$((%d+${HOSTNAME##*-}))", strings.ToUpper(eListener.Name), loadBalancerIP, eListener.ExternalStartingPort))
	}

	for _, iListener := range r.KafkaCluster.Spec.Listeners.InternalListener {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(iListener.Name, "_", "-"),
			ContainerPort: iListener.ContainerPort,
		})
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://${HOSTNAME}.%s-headless.${%s}.svc.cluster.local:%d", strings.ToUpper(iListener.Name), r.KafkaCluster.Name, podNamespace, iListener.ContainerPort))
	}
	genSCRAM := corev1.Container{}
	genJAAS := corev1.Container{}
	if r.KafkaCluster.Spec.Listeners.SASLSecret != "" {
		// TODO handle error
		scramPass, _ := password.Generate(8, 4, 0, true, false)
		//Generate SCRAM username/keystorePass to ZK
		genSCRAM = corev1.Container{
			Name:            "gen-scram",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Image:           r.KafkaCluster.Spec.Image,
			Command: []string{"/bin/sh", "-c",
				fmt.Sprintf("/opt/kafka/bin/kafka-configs.sh --zookeeper %s  --alter --add-config 'SCRAM-SHA-256=[iterations=8192,password=%s],SCRAM-SHA-512=[password=%s]' --entity-type users --entity-name admin &&"+
					`/opt/kafka/bin/kafka-configs.sh --zookeeper %s  --alter --add-config "SCRAM-SHA-256=[iterations=8192,password=$(cat /scram-secret/password)],SCRAM-SHA-512=[password=$(cat /scram-secret/password)]" --entity-type users --entity-name $(cat /scram-secret/username)`, r.KafkaCluster.Spec.ZKAddress, scramPass, scramPass, r.KafkaCluster.Spec.ZKAddress)},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      scramSecret,
					MountPath: "/scram-secret/",
				},
			},
		}

		//Generate JAAS config
		genJAAS = corev1.Container{
			Name:            "gen-jaas",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Image:           r.KafkaCluster.Spec.Image,
			Command: []string{"/bin/sh", "-c",
				fmt.Sprintf(`cat >> config/jaas/kafka_server_jaas.conf << EOF
		KafkaServer {
			org.apache.kafka.common.security.scram.ScramLoginModule required
			username="admin"
			password="%s"
			user_admin="%s"
			user_$(cat /scram-secret/username)="$(cat /scram-secret/password)";
		};
EOF`, scramPass, scramPass)},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      jaasConfig,
					MountPath: "/config/jaas",
				},
				{
					Name:      scramSecret,
					MountPath: "/scram-secret/",
				},
			},
		}
	}

	// Keystore generator
	initPemToKeyStore := corev1.Container{}
	// TODO handle error
	keystorePass, _ := password.Generate(8, 4, 0, true, false)
	if r.KafkaCluster.Spec.Listeners.TLSSecretName != "" {
		initPemToKeyStore = corev1.Container{
			Name:            "pem-to-jks",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Image:           r.KafkaCluster.Spec.Image,
			Command: []string{"/bin/sh", "-c",
				fmt.Sprintf("apk update && apk add openssl && openssl pkcs12 -export -in /var/run/secrets/pemfiles/peerCert -inkey /var/run/secrets/pemfiles/peerKey -out server.p12 -name localhost -CAfile /var/run/secrets/pemfiles/caCert -caname root -chain -password pass:%s &&", keystorePass) +
					fmt.Sprintf("keytool -importkeystore -deststorepass %s -destkeypass %s -destkeystore /var/run/secrets/java.io/keystores/kafka.server.keystore.jks -srckeystore server.p12 -srcstoretype PKCS12 -srcstorepass %s -alias localhost &&", keystorePass, keystorePass, keystorePass) +
					fmt.Sprintf("keytool -keystore /var/run/secrets/java.io/keystores/kafka.server.truststore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass %s -noprompt && ", keystorePass) +
					fmt.Sprintf("keytool -keystore /var/run/secrets/java.io/keystores/kafka.server.keystore.jks -alias CARoot -import -file /var/run/secrets/pemfiles/caCert --deststorepass %s -noprompt", keystorePass)},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      keystoreVolume,
					MountPath: "/var/run/secrets/java.io/keystores",
				},
				{
					Name:      pemFilesVolume,
					MountPath: "/var/run/secrets/pemfiles",
				},
			},
		}
	}

	volumes := []corev1.Volume{
		{
			Name: brokerConfigVolumeMount,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate, r.KafkaCluster.Name)},
				},
			},
		},
		{
			Name: "jmx-jar-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "jmx-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(monitoring.BrokerJmxTemplate, r.KafkaCluster.Name)},
				},
			},
		},
		{
			Name: "extensions",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if r.KafkaCluster.Spec.Listeners.TLSSecretName != "" {
		volumes = append(volumes, []corev1.Volume{
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
						SecretName: r.KafkaCluster.Spec.Listeners.TLSSecretName,
					},
				},
			},
		}...)
	}

	if r.KafkaCluster.Spec.Listeners.SASLSecret != "" {
		volumes = append(volumes, []corev1.Volume{
			{
				Name: jaasConfig,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: scramSecret,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: r.KafkaCluster.Spec.Listeners.SASLSecret,
					},
				},
			},
		}...)
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      brokerConfigVolumeMount,
			MountPath: "/config",
		},
		{
			Name:      kafkaDataVolumeMount,
			MountPath: "/kafka-logs",
		},
		{
			Name:      "jmx-jar-data",
			MountPath: "/opt/jmx-exporter/",
			ReadOnly:  true,
		},
		{
			Name:      "jmx-config",
			MountPath: "/etc/jmx-exporter/",
			ReadOnly:  true,
		},
		{
			Name:      "extensions",
			MountPath: "/opt/kafka/libs/extensions",
			ReadOnly:  true,
		},
	}
	if r.KafkaCluster.Spec.Listeners.TLSSecretName != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      keystoreVolume,
			MountPath: "/var/run/secrets/java.io/keystores",
		})
	}

	if r.KafkaCluster.Spec.Listeners.SASLSecret != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      jaasConfig,
			MountPath: "/config/jaas",
		})
	}

	kafkaOpts := "-javaagent:/opt/jmx-exporter/jmx_prometheus_javaagent-0.3.1-SNAPSHOT.jar=9020:/etc/jmx-exporter/config.yaml"

	if r.KafkaCluster.Spec.Listeners.SASLSecret != "" {
		kafkaOpts = kafkaOpts + " -Djava.security.auth.login.config=/config/jaas/kafka_server_jaas.conf"
	}
	initContainers := append([]corev1.Container{}, []corev1.Container{
		{
			Name:            "jmx-export",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Image:           "banzaicloud/jmx_exporter:latest",
			Command:         []string{"cp", "/usr/share/jmx_exporter/jmx_prometheus_javaagent-0.3.1-SNAPSHOT.jar", "/opt/jmx-exporter/"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "jmx-jar-data",
					MountPath: "/opt/jmx-exporter/",
				},
			},
		},
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
	}...)

	if r.KafkaCluster.Spec.Listeners.TLSSecretName != "" {
		initContainers = append(initContainers, initPemToKeyStore)

	}
	if r.KafkaCluster.Spec.Listeners.SASLSecret != "" {
		initContainers = append(initContainers, genSCRAM)
		initContainers = append(initContainers, genJAAS)
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: templates.ObjectMeta(r.KafkaCluster.Name, map[string]string{}, r.KafkaCluster),
		Spec: appsv1.StatefulSetSpec{
			ServiceName: fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name),
			Replicas:    &r.KafkaCluster.Spec.Brokers,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: kafkaDataVolumeMount,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(r.KafkaCluster.Spec.StorageSize),
							},
						},
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/probe":  "kafka",
						"prometheus.io/port":   "9020",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: r.KafkaCluster.Spec.GetServiceAccount(),
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						{
							Image:           r.KafkaCluster.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "kafka",
							Env: []corev1.EnvVar{
								{
									Name:  podNamespace,
									Value: r.KafkaCluster.Namespace,
								},
								{
									Name:  "CLASSPATH",
									Value: "/opt/kafka/libs/extensions/*",
								},
								//{
								//	Name:  "KAFKA_LOG4J_OPTS",
								//	Value: "-Dlog4j.configuration=file:config/log4j.properties",
								//},
								{
									Name:  "JMX_PORT",
									Value: "5555",
								},
								{
									Name:  "KAFKA_JMX_OPTS",
									Value: "-Djava.net.preferIPv4Stack=true -Dcom.sun.management.jmxremote=true -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false -Dcom.sun.management.jmxremote.local.only=false -Dcom.sun.management.jmxremote.rmi.port=5555 -Djava.rmi.server.hostname=127.0.0.1",
								},
								{
									Name:  "KAFKA_OPTS",
									Value: kafkaOpts,
								},
							},
							//TODO add trustore password only when required
							Command: []string{"sh", "-c", fmt.Sprintf("/opt/kafka/bin/kafka-server-start.sh /config/broker-config --override broker.id=${HOSTNAME##*-} --override ssl.keystore.password=%s --override ssl.truststore.password=%s --override advertised.listeners=%s", keystorePass, keystorePass, strings.Join(advertisedListenerConfig, ","))},
							Ports: append(kafkaBrokerContainerPorts, []corev1.ContainerPort{
								{Name: "prometheus", ContainerPort: 9020},
								{Name: "jmx", ContainerPort: 5555}}...),
							//Env: {},
							//LivenessProbe: &corev1.Probe{
							//	Handler: corev1.Handler{
							//		HTTPGet: &corev1.HTTPGetAction{
							//			Port:   intstr.FromString("api-port"),
							//			Path:   "/v1/sys/init",
							//		}},
							//},
							//ReadinessProbe: &corev1.Probe{
							//	Handler: corev1.Handler{
							//		HTTPGet: &corev1.HTTPGetAction{
							//			Port:   intstr.FromString("api-port"),
							//			Path:   "/v1/sys/health",
							//		}},
							//	PeriodSeconds:    5,
							//	FailureThreshold: 2,
							//},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	return statefulSet
}
