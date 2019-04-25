package resources

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
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

type ResourceVariation func(t string) runtime.Object

type ResourceWithId func(id int32) runtime.Object
