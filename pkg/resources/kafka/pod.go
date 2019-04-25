package kafka

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) pod(id int32) runtime.Object {
	return &corev1.Pod{
		ObjectMeta: templates.ObjectMetaWithGeneratedName(r.KafkaCluster.Name, labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Spec: corev1.PodSpec{
			Hostname:  fmt.Sprintf("%s-%d", r.KafkaCluster.Name, id),
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
					Image:           r.KafkaCluster.Spec.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  "CLASSPATH",
							Value: "/opt/kafka/libs/extensions/*",
						},
					},
					Command: []string{"sh", "-c", "/opt/kafka/bin/kafka-server-start.sh /config/broker-config"},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: int32(29092),
						},
					},
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
							LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id)},
						},
					},
				},
				{
					Name: kafkaDataVolumeMount,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: fmt.Sprintf(brokerStorageTemplate+"-%d", r.KafkaCluster.Name, id),
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
