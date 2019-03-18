package kafka

import (
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/resources/monitoring"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
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
			Name:          eListener.Name,
			ContainerPort: eListener.ContainerPort,
		})
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s:$((%d+${HOSTNAME##*-}))", strings.ToUpper(eListener.Name), loadBalancerIP, eListener.ExternalStartingPort))
	}

	for _, iListener := range r.KafkaCluster.Spec.Listeners.InternalListener {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          iListener.Name,
			ContainerPort: iListener.ContainerPort,
		})
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://${HOSTNAME}.%s-headless.${%s}.svc.cluster.local:%d", strings.ToUpper(iListener.Name), r.KafkaCluster.Name, podNamespace, iListener.ContainerPort))
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
					InitContainers: []corev1.Container{
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
					},
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
									Value: "-javaagent:/opt/jmx-exporter/jmx_prometheus_javaagent-0.3.1-SNAPSHOT.jar=9020:/etc/jmx-exporter/config.yaml",
								},
							},
							Command: []string{"sh", "-c", fmt.Sprintf("/opt/kafka/bin/kafka-server-start.sh /config/broker-config --override broker.id=${HOSTNAME##*-} --override advertised.listeners=%s", strings.Join(advertisedListenerConfig, ","))},
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
