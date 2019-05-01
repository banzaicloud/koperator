package resources

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Reconciler struct {
	client.Client
	KafkaCluster *banzaicloudv1alpha1.KafkaCluster
}

type ComponentReconciler interface {
	Reconcile(log logr.Logger) error
}

type Resource func() runtime.Object

type ResourceWithLogs func(log logr.Logger) runtime.Object

type ResourceWithBrokerAndVolume func(broker banzaicloudv1alpha1.BrokerConfig, pvcs []corev1.PersistentVolumeClaim, log logr.Logger) runtime.Object

type ResourceWithBrokerAndString func(broker banzaicloudv1alpha1.BrokerConfig, t string, log logr.Logger) runtime.Object

type ResourceWithBrokerAndStorage func(broker banzaicloudv1alpha1.BrokerConfig, storage banzaicloudv1alpha1.StorageConfig, log logr.Logger) runtime.Object
