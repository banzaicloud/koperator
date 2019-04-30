package kafka

import (
	"fmt"
	"strings"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) headlessServicePod() runtime.Object {

	var usedPorts []corev1.ServicePort

	for _, iListeners := range r.KafkaCluster.Spec.ListenersConfig.InternalListeners {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name: strings.ReplaceAll(iListeners.Name, "_", ""),
			Port: iListeners.ContainerPort,
		})
	}

	return &corev1.Service{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector:  labelsForKafka(r.KafkaCluster.Name),
			ClusterIP: corev1.ClusterIPNone,
			Ports:     usedPorts,
		},
	}
}
