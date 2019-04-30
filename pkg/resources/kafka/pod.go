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

func (r *Reconciler) pod(broker banzaicloudv1alpha1.BrokerConfig, log logr.Logger) runtime.Object {

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

	return &corev1.Pod{
		ObjectMeta: templates.ObjectMetaWithGeneratedName(r.KafkaCluster.Name, util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", broker.Id)}), r.KafkaCluster),
		Spec: corev1.PodSpec{
			Hostname:  fmt.Sprintf("%s-%d", r.KafkaCluster.Name, broker.Id),
			Subdomain: fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name),
			InitContainers: []corev1.Container{
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
			},
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
					Command: []string{"sh", "-c", "/opt/kafka/bin/kafka-server-start.sh /config/broker-config"},
					Ports:   kafkaBrokerContainerPorts,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      brokerConfigVolumeMount,
							MountPath: "/config",
						},
						{
							Name:      kafkaDataVolumeMount,
							MountPath: "/kafka-logs",
						},
						{
							Name:      "extensions",
							MountPath: "/opt/kafka/libs/extensions",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: brokerConfigVolumeMount,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, broker.Id)},
						},
					},
				},
				{
					Name: kafkaDataVolumeMount,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: fmt.Sprintf(brokerStorageTemplate+"-%d", r.KafkaCluster.Name, broker.Id),
						},
					},
				},
				{
					Name: "extensions",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}
