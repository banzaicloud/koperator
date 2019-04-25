package envoy

//import (
//	"fmt"
//	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
//	"github.com/go-logr/logr"
//	"k8s.io/apimachinery/pkg/runtime"
//
//	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
//	appsv1 "k8s.io/api/apps/v1"
//	corev1 "k8s.io/api/core/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//)
//
//func (r *Reconciler) deployment(log logr.Logger) runtime.Object {
//
//	exposedPorts := getExposedContainerPorts(r.KafkaCluster.Spec.Listeners.ExternalListener, r.KafkaCluster.Spec.Brokers, log)
//	volumes := []corev1.Volume{
//		{
//			Name: envoyVolumeAndConfigName,
//			VolumeSource: corev1.VolumeSource{
//				ConfigMap: &corev1.ConfigMapVolumeSource{
//					LocalObjectReference: corev1.LocalObjectReference{Name: envoyVolumeAndConfigName},
//				},
//			},
//		},
//	}
//
//	volumeMounts := []corev1.VolumeMount{
//		{
//			Name:      envoyVolumeAndConfigName,
//			MountPath: "/etc/envoy",
//			ReadOnly:  true,
//		},
//	}
//
//	return &appsv1.Deployment{
//		ObjectMeta: templates.ObjectMetaWithName(envoyDeploymentName, labelSelector, r.KafkaCluster),
//		Spec: appsv1.DeploymentSpec{
//			Selector: &metav1.LabelSelector{
//				MatchLabels: labelSelector,
//			},
//			Template: corev1.PodTemplateSpec{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: labelSelector,
//				},
//				Spec: corev1.PodSpec{
//					Containers: []corev1.Container{
//						{
//							Name:            "envoy",
//							Image:           "banzaicloud/envoy:0.1.0",
//							ImagePullPolicy: corev1.PullIfNotPresent,
//							Ports: append(exposedPorts, []corev1.ContainerPort{
//								{Name: "envoy-admin", ContainerPort: 9901}}...),
//							VolumeMounts: volumeMounts,
//						},
//					},
//					Volumes: volumes,
//				},
//			},
//		},
//	}
//}
//
//func getExposedContainerPorts(extListeners []banzaicloudv1alpha1.ExternalListenerConfig, brokers int32, log logr.Logger) []corev1.ContainerPort {
//	var exposedPorts []corev1.ContainerPort
//
//	for _, eListener := range extListeners {
//		for brokerSize := int32(0); brokerSize < brokers; brokerSize++ {
//			exposedPorts = append(exposedPorts, corev1.ContainerPort{
//				Name:          fmt.Sprintf("broker-%d", brokerSize),
//				ContainerPort: eListener.ExternalStartingPort + brokerSize,
//			})
//		}
//	}
//	return exposedPorts
//}
