package envoy

import (
	"errors"
	"fmt"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("envoy-components-builder")

// LoadBalancerForEnvoy return a Loadbalancer service for Envoy
func LoadBalancerForEnvoy(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.Service {

	var exposedPorts []corev1.ServicePort

	for _, eListener := range kc.Spec.Listeners.ExternalListener {
		switch eListener.Type {
		case "plaintext":
			for brokerSize := int32(0); brokerSize < kc.Spec.Brokers; brokerSize++ {
				exposedPorts = append(exposedPorts, corev1.ServicePort{
					Name: fmt.Sprintf("broker-%d", brokerSize),
					Port: eListener.ExternalStartingPort + brokerSize,
					//TargetPort: intstr.FromInt(int(eListener.ContainerPort)),
				})
			}
		case "tls":
			log.Error(errors.New("TLS listener type is not supported yet"), "not supported")
		case "both":
			log.Error(errors.New("both listener type is not supported yet"), "not supported")
		}
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envoy-loadbalancer",
			Namespace: kc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "envoy"},
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports:    exposedPorts,
		},
	}
	return service
}

func DeploymentForEnvoy(kc *banzaicloudv1alpha1.KafkaCluster) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envoy",
			Namespace: kc.Namespace,
			Labels:    map[string]string{"app": "envoy"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "envoy"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "envoy"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "envoy",
							Image:           "banzaicloud/envoy:01",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "envoy-admin",
									ContainerPort: 9901,
								},
								{
									Name:          "broker-0",
									ContainerPort: 19090,
								},
								{
									Name:          "broker-1",
									ContainerPort: 19091,
								},
								{
									Name:          "broker-2",
									ContainerPort: 19092,
								},
							},
						},
					},
				},
			},
		},
	}
}
