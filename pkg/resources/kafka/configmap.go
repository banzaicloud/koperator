package kafka

import (
	"errors"
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/go-logr/logr"
	"strings"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) configMap(log logr.Logger) runtime.Object {
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate, r.KafkaCluster.Name), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Data:       map[string]string{"broker-config": generateListenerSpecificConfig(&r.KafkaCluster.Spec.Listeners, log) + r.KafkaCluster.Spec.GenerateDefaultConfig()},
	}
	return configMap
}

func generateListenerSpecificConfig(l *banzaicloudv1alpha1.Listeners, log logr.Logger) string {

	var interBrokerListener string
	var securityProtocolMapConfig []string
	var listenerConfig []string

	for _, iListener := range l.InternalListener {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListener == "" {
				interBrokerListener = strings.ToUpper(iListener.Name)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
		UpperedListenerType := strings.ToUpper(iListener.Type)
		UpperedListenerName := strings.ToUpper(iListener.Name)
		switch UpperedListenerType {
		case "PLAINTEXT":
			securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		case "TLS":
		}
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, iListener.ContainerPort))
	}
	for _, eListener := range l.ExternalListener {
		UpperedListenerType := strings.ToUpper(eListener.Type)
		UpperedListenerName := strings.ToUpper(eListener.Name)
		switch UpperedListenerType {
		case "PLAINTEXT":
			securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		case "TLS":
		}
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
	}
	return "listener.security.protocol.map=" + strings.Join(securityProtocolMapConfig, ",") + "\n" +
		"inter.broker.listener.name=" + interBrokerListener + "\n" +
		"listeners=" + strings.Join(listenerConfig, ",") + "\n"
}
