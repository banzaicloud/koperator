package kafka

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName           = "kafka"
	HeadlessServiceTemplate = "%s-headless"
	brokerConfigTemplate    = "%s-config"
	brokerStorageTemplate   = "%s-storage"
	brokerConfigVolumeMount = "broker-config"
	kafkaDataVolumeMount    = "kafka-data"
	podNamespace            = "POD_NAMESPACE"
	keystoreVolume          = "ks-files"
	pemFilesVolume          = "pem-files"
	jaasConfig              = "jaas-config"
	scramSecret             = "scram-secret"
)

type Reconciler struct {
	resources.Reconciler
}

// labelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}

func New(client client.Client, cluster *banzaicloudv1alpha1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

//func getLoadBalancerIP(client client.Client, namespace string, log logr.Logger) (string, error) {
//	foundLBService := &corev1.Service{}
//	err := client.Get(context.TODO(), types.NamespacedName{Name: envoy.EnvoyServiceName, Namespace: namespace}, foundLBService)
//	if err != nil {
//		return "", err
//	}
//
//	if len(foundLBService.Status.LoadBalancer.Ingress) == 0 {
//		return "", fmt.Errorf("loadbalancer is not created waiting")
//	}
//
//	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" && foundLBService.Status.LoadBalancer.Ingress[0].IP == "" {
//		time.Sleep(20 * time.Second)
//		return "", fmt.Errorf("loadbalancer is not ready waiting")
//	}
//	var loadBalancerExternalAddress string
//	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" {
//		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].IP
//	} else {
//		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].Hostname
//	}
//	return loadBalancerExternalAddress, nil
//}

func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.Info("Reconciling")

	for _, res := range []resources.Resource{
		r.headlessServicePod,
	} {
		o := res()
		err := k8sutil.Reconcile(log, r.Client, o)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	for brokerId := int32(0); brokerId < r.KafkaCluster.Spec.Brokers; brokerId++ {
		for _, res := range []resources.ResourceWithId{
			r.configMapPod,
			r.pvc,
			r.pod,
		} {
			o := res(brokerId)
			err := k8sutil.Reconcile(log, r.Client, o)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
	}

	//for _, res := range []resources.ResourceWithLogs{
	//	r.configMap,
	//} {
	//	o := res(log)
	//	err := k8sutil.Reconcile(log, r.Client, o)
	//	if err != nil {
	//		return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
	//	}
	//}
	//
	//for _, res := range []resources.Resource{
	//	r.headlessService,
	//} {
	//	o := res()
	//	err := k8sutil.Reconcile(log, r.Client, o)
	//	if err != nil {
	//		return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
	//	}
	//}
	//
	//lBIp, err := getLoadBalancerIP(r.Client, r.KafkaCluster.Namespace, log)
	//if err != nil {
	//	return emperror.WrapWith(err, "failed to get loadbalancerIP maybe still creating...")
	//}
	//
	//for _, res := range []resources.ResourceVariation{
	//	r.statefulSet,
	//} {
	//	o := res(lBIp)
	//	err := k8sutil.Reconcile(log, r.Client, o)
	//	if err != nil {
	//		return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
	//	}
	//}

	log.Info("Reconciled")

	return nil
}
