package kafka

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) headlessServicePod() runtime.Object {
	return &corev1.Service{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector:  labelsForKafka(r.KafkaCluster.Name),
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Port: int32(29092),
				},
			},
		},
	}
}
