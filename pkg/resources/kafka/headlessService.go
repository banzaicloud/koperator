package kafka

import (
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

// HeadlessServiceForKafka return a HeadLess service for Kafka
func (r *Reconciler) headlessService() runtime.Object {

	var usedPorts []corev1.ServicePort

	for _, iListeners := range r.KafkaCluster.Spec.Listeners.InternalListener {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name: strings.ReplaceAll(iListeners.Name, "_", ""),
			Port: iListeners.ContainerPort,
		})
	}
	service := &corev1.Service{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector:  labelsForKafka(r.KafkaCluster.Name),
			ClusterIP: corev1.ClusterIPNone,
			Ports:     usedPorts,
		},
	}
	return service
}
