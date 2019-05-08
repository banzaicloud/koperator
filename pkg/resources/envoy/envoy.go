package envoy

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName            = "envoy"
	EnvoyServiceName         = "envoy-loadbalancer"
	envoyVolumeAndConfigName = "envoy-config"
	envoyDeploymentName      = "envoy"
)

var labelSelector = map[string]string{
	"app": "envoy",
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
	log = log.WithValues("component", componentName)

	log.Info("Reconciling")

	for _, res := range []resources.ResourceWithLogs{
		r.loadBalancer,
		r.configMap,
		r.deployment,
	} {
		o := res(log)
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	log.Info("Reconciled")

	return nil
}
