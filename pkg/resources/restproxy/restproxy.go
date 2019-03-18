package restproxy

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	deploymentName = "rest-proxy"
	serviceName    = "rest-proxy-svc"
	componentName  = "rest-proxy"
)

var labelSelector = map[string]string{
	"app": "rest-proxy",
}

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
	if r.KafkaCluster.Spec.RestProxyEnabled {
		log = log.WithValues("component", componentName)

		log.Info("Reconciling")

		for _, res := range []resources.Resource{
			r.service,
			r.deployment,
		} {
			o := res()
			err := k8sutil.Reconcile(log, r.Client, o)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}

		log.Info("Reconciled")
	}

	return nil
}
