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
		Data: map[string]string{"broker-config": generateListenerSpecificConfig(&r.KafkaCluster.Spec.Listeners, log) +
			fmt.Sprintf("zookeeper.connect=%s\n", r.KafkaCluster.Spec.ZKAddress) +
			generateSSLConfig(&r.KafkaCluster.Spec.Listeners) +
			generateSASLConfig(&r.KafkaCluster.Spec.Listeners) +
			r.KafkaCluster.Spec.GenerateDefaultConfig()},
	}
	return configMap
}

func generateSSLConfig(l *banzaicloudv1alpha1.Listeners) (res string) {
	if l.TLSSecretName != "" {
		res = `ssl.keystore.location=/var/run/secrets/java.io/keystores/kafka.server.keystore.jks
ssl.truststore.location=/var/run/secrets/java.io/keystores/kafka.server.truststore.jks
ssl.client.auth=required
`
	}
	return
}

func generateSASLConfig(l *banzaicloudv1alpha1.Listeners) (res string) {
	if l.SASLSecret != "" {
		res = `sasl.enabled.mechanisms=SCRAM-SHA-256
sasl.mechanism.inter.broker.protocol=SCRAM-SHA-256
`
	}
	return
}

func generateListenerSpecificConfig(l *banzaicloudv1alpha1.Listeners, log logr.Logger) string {

	var interBrokerListenerType string
	var securityProtocolMapConfig []string
	var listenerConfig []string

	for _, iListener := range l.InternalListener {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListenerType == "" {
				interBrokerListenerType = strings.ToUpper(iListener.Type)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
		UpperedListenerType := strings.ToUpper(iListener.Type)
		UpperedListenerName := strings.ToUpper(iListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, iListener.ContainerPort))
	}
	for _, eListener := range l.ExternalListener {
		UpperedListenerType := strings.ToUpper(eListener.Type)
		UpperedListenerName := strings.ToUpper(eListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
	}
	return "listener.security.protocol.map=" + strings.Join(securityProtocolMapConfig, ",") + "\n" +
		"security.inter.broker.protocol=" + interBrokerListenerType + "\n" +
		"listeners=" + strings.Join(listenerConfig, ",") + "\n"
}
