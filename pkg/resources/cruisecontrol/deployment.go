package cruisecontrol

import (
	"fmt"

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
					InitContainers: []corev1.Container{
						{
							Name:            "create-topic",
							Image:           r.KafkaCluster.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/bash", "-c", fmt.Sprintf("until /opt/kafka/bin/kafka-topics.sh --zookeeper %s --create --if-not-exists --topic __CruiseControlMetrics --partitions 12 --replication-factor 3; do echo waiting for kafka; sleep 3; done ;", r.KafkaCluster.Spec.ZKAddress)},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           "solsson/kafka-cruise-control@sha256:d5e05c95d6e8fddc3e607ec3cdfa2a113b76eabca4aefe6c382f5b3d7d990505",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8090,
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
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configAndVolumeName,
									MountPath: "/opt/cruise-control/config",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: configAndVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configAndVolumeName},
								},
							},
						},
					},
				},
			},
		},
	}
}
