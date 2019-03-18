package restproxy

import (
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) service() runtime.Object {
	return &corev1.Service{
		ObjectMeta: templates.ObjectMeta(serviceName, labelSelector, r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector: labelSelector,
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
}
