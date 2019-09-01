package k8sutil

import (
	"context"
	"fmt"

	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func LookupKafkaCluster(client runtimeClient.Client, ref v1alpha1.ClusterReference) (cluster *v1alpha1.KafkaCluster, err error) {
	cluster = &v1alpha1.KafkaCluster{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, cluster)
	return
}

func LookupControllerSecret(client runtimeClient.Client, ref v1alpha1.ClusterReference, controllerTempl string) (secret *corev1.Secret, err error) {
	secret = &corev1.Secret{}
	secretName := fmt.Sprintf(controllerTempl, ref.Name)
	err = client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: ref.Namespace}, secret)
	return
}
