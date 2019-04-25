package kafka

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) configMapPod(id int32) runtime.Object {
	return &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Data: map[string]string{"broker-config": fmt.Sprintf(`
broker.id=%d
log.dirs=/kafka-logs/kafka
advertised.listeners=PLAINTEXT://%s-%d.%s.%s.svc.cluster.local:29092
listeners=PLAINTEXT://:29092
cruise.control.metrics.reporter.bootstrap.servers=PLAINTEXT://%s-%d.%s.%s.svc.cluster.local:29092
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
`, id, r.KafkaCluster.Name, id, fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name), r.KafkaCluster.Namespace, r.KafkaCluster.Name, id, fmt.Sprintf(HeadlessServiceTemplate, r.KafkaCluster.Name), r.KafkaCluster.Namespace) + fmt.Sprintf("zookeeper.connect=%s\n", r.KafkaCluster.Spec.ZKAddress)},
	}
}
