package envoy

import (
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// loadBalancer return a Loadbalancer service for Envoy
func (r *Reconciler) loadBalancer(log logr.Logger) runtime.Object {

	exposedPorts := getExposedServicePorts(r.KafkaCluster.Spec.ListenersConfig.ExternalListeners, r.KafkaCluster.Spec.BrokerConfigs)

	service := &corev1.Service{
		ObjectMeta: templates.ObjectMeta(EnvoyServiceName, map[string]string{}, r.KafkaCluster),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "envoy"},
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports:    exposedPorts,
		},
	}
	return service
}

func getExposedServicePorts(extListeners []banzaicloudv1alpha1.ExternalListenerConfig, brokers []banzaicloudv1alpha1.BrokerConfig) []corev1.ServicePort {
	var exposedPorts []corev1.ServicePort

	for _, eListener := range extListeners {
		for _, broker := range brokers {
			exposedPorts = append(exposedPorts, corev1.ServicePort{
				Name: fmt.Sprintf("broker-%d", broker.Id),
				Port: eListener.ExternalStartingPort + broker.Id,
				Protocol: corev1.ProtocolTCP,
			})
		}
	}
	return exposedPorts
}
