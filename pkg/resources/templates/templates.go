package templates

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ObjectMeta(name string, labels map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: cluster.Namespace,
		Labels:    labels,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         util.BoolPointer(true),
				BlockOwnerDeletion: util.BoolPointer(true),
			},
		},
	}
}

func ObjectMetaWithAnnotations(name string, labels map[string]string, annotations map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	o := ObjectMeta(name, labels, cluster)
	o.Annotations = annotations
	return o
}

func ObjectMetaClusterScope(name string, labels map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:   name,
		Labels: labels,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         util.BoolPointer(true),
				BlockOwnerDeletion: util.BoolPointer(true),
			},
		},
	}
}
