package cruisecontrol_monitoring

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CruiseControlJmxTemplate = "%s-cc-jmx-exporter"
	componentName            = "cruisecontrol_monitoring"
)

type Reconciler struct {
	resources.Reconciler
}

func New(client client.Client, cluster *banzaicloudv1alpha1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.Info("Reconciling")

	for _, res := range []resources.Resource{
		r.configMap,
	} {
		o := res()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster.Name)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	log.Info("Reconciled")

	return nil
}

func labelsForJmx(name string) map[string]string {
	return map[string]string{"app": "cruisecontrol-jmx", "kafka_cr": name}
}
