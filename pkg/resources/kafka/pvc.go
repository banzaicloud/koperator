package kafka

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) pvc(id int32) runtime.Object {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerStorageTemplate+"-%d", r.KafkaCluster.Name, id), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
}
