package kafka

import (
	"fmt"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) pvc(broker banzaicloudv1alpha1.BrokerConfig, storage banzaicloudv1alpha1.StorageConfig, log logr.Logger) runtime.Object {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: templates.ObjectMetaWithGeneratedNameAndAnnotations(fmt.Sprintf(brokerStorageTemplate, r.KafkaCluster.Name), util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", broker.Id)}), map[string]string{"mountPath": storage.MountPath}, r.KafkaCluster),
		Spec:       *storage.PVCSpec,
	}
}
