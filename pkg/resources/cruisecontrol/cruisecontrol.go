package cruisecontrol

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName          = "cruisecontrol"
	serviceName            = "cruisecontrol-svc"
	configAndVolumeName    = "cruisecontrol-config"
	modconfigAndVolumeName = "cruisecontrol-modconfig"
	deploymentName         = "cruisecontrol"
	keystoreVolume         = "ks-files"
	keystoreVolumePath     = "/var/run/secrets/java.io/keystores"
	pemFilesVolume         = "pem-files"
	jmxVolumePath          = "/opt/jmx-exporter/"
	jmxVolumeName          = "jmx-jar-data"
)

var labelSelector = map[string]string{
	"app": "cruisecontrol",
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
		r.service,
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
