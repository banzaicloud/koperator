package kafka

import (
	"errors"
	"fmt"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/monitoring"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
)

var log = logf.Log.WithName("kafka-components-builder")

//// LoadBalancerForKafka return a Loadbalancer service for Kafka
//func LoadBalancerForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.Service {
//
//	var exposedPorts []corev1.ServicePort
//
//	for _, eListener := range kc.Spec.Listeners.ExternalListener {
//		switch eListener.Type {
//		case "plaintext":
//			for brokerSize := int32(0); brokerSize < kc.Spec.Brokers; brokerSize++ {
//				exposedPorts = append(exposedPorts, corev1.ServicePort{
//					Name:       fmt.Sprintf("broker-%d", brokerSize),
//					Port:       eListener.ExternalStartingPort + brokerSize,
//					TargetPort: intstr.FromInt(int(eListener.ContainerPort)),
//				})
//			}
//		case "tls":
//			log.Error(errors.New("TLS listener type is not supported yet"), "not supported")
//		case "both":
//			log.Error(errors.New("both listener type is not supported yet"), "not supported")
//		}
//	}
//
//	service := &corev1.Service{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "v1",
//			Kind:       "Service",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      fmt.Sprintf(loadBalancerServiceTemplate, kc.Name),
//			Namespace: kc.Namespace,
//		},
//		Spec: corev1.ServiceSpec{
//			Selector: labelsForKafka(kc.Name),
//			Type:     corev1.ServiceTypeLoadBalancer,
//			Ports:    exposedPorts,
//		},
//	}
//	return service
//}

// HeadlessServiceForKafka return a HeadLess service for Kafka
func HeadlessServiceForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.Service {

	var usedPorts []corev1.ServicePort

	for _, iListeners := range kc.Spec.Listeners.InternalListener {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name: iListeners.Name,
			Port: iListeners.ContainerPort,
		})
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(headlessServiceTemplate, kc.Name),
			Namespace: kc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector:  labelsForKafka(kc.Name),
			ClusterIP: corev1.ClusterIPNone,
			Ports:     usedPorts,
		},
	}
	return service
}

func ConfigMapForKafka(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(brokerConfigTemplate, kc.Name),
			Namespace: kc.Namespace,
			Labels:    labelsForKafka(kc.Name),
		},
		Data: map[string]string{"broker-config": generateListenerSpecificConfig(&kc.Spec.Listeners) + kc.Spec.GenerateDefaultConfig()},
	}
	return configMap
}

func generateListenerSpecificConfig(l *banzaicloudv1alpha1.Listeners) string {

	var interBrokerListener string
	var securityProtocolMapConfig []string
	var listenerConfig []string

	for _, iListener := range l.InternalListener {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListener == "" {
				interBrokerListener = strings.ToUpper(iListener.Name)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
		UpperedListenerType := strings.ToUpper(iListener.Type)
		UpperedListenerName := strings.ToUpper(iListener.Name)
		switch UpperedListenerType {
		case "PLAINTEXT":
			securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		case "TLS":
		}
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, iListener.ContainerPort))
	}
	for _, eListener := range l.ExternalListener {
		UpperedListenerType := strings.ToUpper(eListener.Type)
		UpperedListenerName := strings.ToUpper(eListener.Name)
		switch UpperedListenerType {
		case "PLAINTEXT":
			securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		case "TLS":
		}
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
	}
	return "listener.security.protocol.map=" + strings.Join(securityProtocolMapConfig, ",") + "\n" +
		"inter.broker.listener.name=" + interBrokerListener + "\n" +
		"listeners=" + strings.Join(listenerConfig, ",") + "\n"
}

// StatefulSetForKafka returns a Kafka StatefulSet object
func StatefulSetForKafka(kc *banzaicloudv1alpha1.KafkaCluster, loadBalancerIP string) *appsv1.StatefulSet {
	ls := labelsForKafka(kc.Name)
	var kafkaBrokerContainerPorts []corev1.ContainerPort
	var advertisedListenerConfig []string

	for _, eListener := range kc.Spec.Listeners.ExternalListener {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          eListener.Name,
			ContainerPort: eListener.ContainerPort,
		})
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s:$((%d+${HOSTNAME##*-}))", strings.ToUpper(eListener.Name), loadBalancerIP, eListener.ExternalStartingPort))
	}

	for _, iListener := range kc.Spec.Listeners.InternalListener {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          iListener.Name,
			ContainerPort: iListener.ContainerPort,
		})
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://${HOSTNAME}.%s-headless.${%s}.svc.cluster.local:%d", strings.ToUpper(iListener.Name), kc.Name, podNamespace, iListener.ContainerPort))
	}

	volumes := []corev1.Volume{
		{
			Name: brokerConfigVolumeMount,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate, kc.Name)},
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
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(monitoring.BrokerJmxTemplate, kc.Name)},
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
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kc.Name,
			Namespace: kc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: fmt.Sprintf(headlessServiceTemplate, kc.Name),
			Replicas:    &kc.Spec.Brokers,
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
								corev1.ResourceStorage: resource.MustParse(kc.Spec.StorageSize),
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
					ServiceAccountName: kc.Spec.GetServiceAccount(),
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
							Image:           kc.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "kafka",
							Env: []corev1.EnvVar{
								{
									Name:  podNamespace,
									Value: kc.Namespace,
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

// labelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}
