package restproxy

import (
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) deployment() runtime.Object {

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
					Containers: []corev1.Container{
						{
							Name:            "rest-proxy",
							Image:           "mailgun/kafka-pixy:0.16.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{"kafka-pixy",
								"-kafkaPeers",
								fmt.Sprintf("%s:29092", fmt.Sprintf(kafka.HeadlessServiceTemplate, r.KafkaCluster.Name)),
								"-zookeeperPeers",
								r.KafkaCluster.Spec.ZKAddress,
								"-tcpAddr",
								"0.0.0.0:80"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}
