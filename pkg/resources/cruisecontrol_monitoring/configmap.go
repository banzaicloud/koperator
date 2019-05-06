package cruisecontrol_monitoring

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) configMap() runtime.Object {
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(CruiseControlJmxTemplate, r.KafkaCluster.Name), labelsForJmx(r.KafkaCluster.Name), r.KafkaCluster),
		Data: map[string]string{"config.yaml": `
    lowercaseOutputName: true
`},
	}
	return configMap
}
