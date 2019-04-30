package kafka

import (
	//"fmt"
	//"time"

	"context"
	"errors"
	"fmt"
	"time"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"k8s.io/apimachinery/pkg/types"

	//"github.com/banzaicloud/kafka-operator/pkg/resources/envoy"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/types"
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

func getLoadBalancerIP(client client.Client, namespace string, log logr.Logger) (string, error) {
	foundLBService := &corev1.Service{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: envoy.EnvoyServiceName, Namespace: namespace}, foundLBService)
	if err != nil {
		return "", err
	}

	if len(foundLBService.Status.LoadBalancer.Ingress) == 0 {
		return "", errors.New("loadbalancer is not created waiting")
	}

	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" && foundLBService.Status.LoadBalancer.Ingress[0].IP == "" {
		time.Sleep(20 * time.Second)
		return "", errors.New("loadbalancer is not ready waiting")
	}
	var loadBalancerExternalAddress string
	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" {
		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].IP
	} else {
		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].Hostname
	}
	return loadBalancerExternalAddress, nil
}

func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.Info("Reconciling")

	for _, res := range []resources.Resource{
		r.headlessServicePod,
	} {
		o := res()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster.Name)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}
	podList := &corev1.PodList{}
	matchingLabels := map[string]string{
		"kafka_cr": r.KafkaCluster.Name,
	}
	err := r.Client.List(context.TODO(), client.InNamespace(r.KafkaCluster.Namespace).MatchingLabels(matchingLabels), podList)
	if err != nil {
		return emperror.Wrap(err, "failed to reconcile resource")
	}
	if len(podList.Items) > len(r.KafkaCluster.Spec.BrokerConfigs) {
		deletedBrokers := make([]corev1.Pod, 0)
	OUTERLOOP:
		for _, pod := range podList.Items {
			for _, broker := range r.KafkaCluster.Spec.BrokerConfigs {
				if pod.Labels["brokerId"] == fmt.Sprintf("%d", broker.Id) {
					continue OUTERLOOP
				}
			}
			deletedBrokers = append(deletedBrokers, pod)
		}
		for _, broker := range deletedBrokers {
			err := r.Client.Delete(context.TODO(), &broker)
			if err != nil {
				return emperror.WrapWith(err, "could not delete broker", "id", broker.Labels["brokerId"])
			}
			err = r.Client.Delete(context.TODO(), &corev1.ConfigMap{ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate+"-%s", r.KafkaCluster.Name, broker.Labels["brokerId"]), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
			if err != nil {
				return emperror.WrapWith(err, "could not delete configmap for broker", "id", broker.Labels["brokerId"])
			}
			err = r.Client.Delete(context.TODO(), &corev1.PersistentVolumeClaim{ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerStorageTemplate+"-%s", r.KafkaCluster.Name, broker.Labels["brokerId"]), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
			if err != nil {
				return emperror.WrapWith(err, "could not delete pvc for broker", "id", broker.Labels["brokerId"])
			}
		}
	}

	lBIp, err := getLoadBalancerIP(r.Client, r.KafkaCluster.Namespace, log)
	if err != nil {
		return emperror.WrapWith(err, "failed to get loadbalancerIP maybe still creating...")
	}

	for _, broker := range r.KafkaCluster.Spec.BrokerConfigs {
		for _, storage := range broker.StorageConfigs {
			for _, res := range []resources.ResourceWithBrokerAndStorage{
				r.pvc,
			} {
				o := res(broker, storage, log)
				err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster.Name)
				if err != nil {
					return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
				}
			}
		}
		for _, res := range []resources.ResourceWithBroker{
			r.configMapPod,
		} {
			o := res(broker, log)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster.Name)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}

		for _, res := range []resources.ResourceWithBrokerAndString{
			r.pod,
		} {
			o := res(broker, lBIp, log)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster.Name)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
	}

	log.Info("Reconciled")

	return nil
}
