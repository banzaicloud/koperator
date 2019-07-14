// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafka

import (
	"errors"
	"fmt"
	"strings"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *Reconciler) configMapPod(broker banzaicloudv1alpha1.BrokerConfig, loadBalancerIP string, log logr.Logger) runtime.Object {
	return &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, broker.Id), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster),
		Data: map[string]string{"broker-config": generateListenerSpecificConfig(&r.KafkaCluster.Spec.ListenersConfig, log) +
			fmt.Sprintf("zookeeper.connect=%s\n", strings.Join(r.KafkaCluster.Spec.ZKAddresses, ",")) +
			generateSSLConfig(&r.KafkaCluster.Spec.ListenersConfig) +
			"metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter\n" +
			fmt.Sprintf("cruise.control.metrics.reporter.bootstrap.servers=%s\n", strings.Join(getInternalListeners(r.KafkaCluster.Spec.ListenersConfig.InternalListeners, broker, r.KafkaCluster.Namespace, r.KafkaCluster.Name), ",")) +
			fmt.Sprintf("broker.id=%d\n", broker.Id) +
			generateStorageConfig(broker.StorageConfigs) +
			generateAdvertisedListenerConfig(broker, r.KafkaCluster.Spec.ListenersConfig, loadBalancerIP, r.KafkaCluster.Namespace, r.KafkaCluster.Name) +
			broker.Config},
	}
}

func generateAdvertisedListenerConfig(broker banzaicloudv1alpha1.BrokerConfig, l banzaicloudv1alpha1.ListenersConfig, loadBalancerIP, namespace, crName string) string {
	advertisedListenerConfig := []string{}
	for _, eListener := range l.ExternalListeners {
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s:%d", strings.ToUpper(eListener.Name), loadBalancerIP, eListener.ExternalStartingPort+broker.Id))
	}
	for _, iListener := range l.InternalListeners {
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s-%d.%s.svc.cluster.local:%d", strings.ToUpper(iListener.Name), crName, broker.Id, namespace, iListener.ContainerPort))
	}
	return fmt.Sprintf("advertised.listeners=%s\n", strings.Join(advertisedListenerConfig, ","))
}

func generateStorageConfig(sConfig []banzaicloudv1alpha1.StorageConfig) string {
	mountPaths := []string{}
	for _, storage := range sConfig {
		mountPaths = append(mountPaths, storage.MountPath+`/kafka`)
	}
	return fmt.Sprintf("log.dirs=%s\n", strings.Join(mountPaths, ","))
}

func generateSSLConfig(l *banzaicloudv1alpha1.ListenersConfig) (res string) {
	if l.SSLSecrets != nil {
		res = `ssl.keystore.location=/var/run/secrets/java.io/keystores/kafka.server.keystore.jks
ssl.truststore.location=/var/run/secrets/java.io/keystores/kafka.server.truststore.jks
ssl.client.auth=required
`
	}
	if l.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(l.InternalListeners) {
		res = res + `cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.truststore.location=/var/run/secrets/java.io/keystores/client.truststore.jks
cruise.control.metrics.reporter.ssl.keystore.location=/var/run/secrets/java.io/keystores/client.keystore.jks
`
	}
	return
}

func generateListenerSpecificConfig(l *banzaicloudv1alpha1.ListenersConfig, log logr.Logger) string {

	var interBrokerListenerType string
	var securityProtocolMapConfig []string
	var listenerConfig []string

	for _, iListener := range l.InternalListeners {
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
	for _, eListener := range l.ExternalListeners {
		UpperedListenerType := strings.ToUpper(eListener.Type)
		UpperedListenerName := strings.ToUpper(eListener.Name)
		securityProtocolMapConfig = append(securityProtocolMapConfig, fmt.Sprintf("%s:%s", UpperedListenerName, UpperedListenerType))
		listenerConfig = append(listenerConfig, fmt.Sprintf("%s://:%d", UpperedListenerName, eListener.ContainerPort))
	}
	return "listener.security.protocol.map=" + strings.Join(securityProtocolMapConfig, ",") + "\n" +
		"security.inter.broker.protocol=" + interBrokerListenerType + "\n" +
		"listeners=" + strings.Join(listenerConfig, ",") + "\n"
}

func getInternalListeners(iListeners []banzaicloudv1alpha1.InternalListenerConfig, broker banzaicloudv1alpha1.BrokerConfig, namespace, crName string) []string {

	listenerConfig := []string{}

	for _, iListener := range iListeners {
		listenerConfig = append(listenerConfig,
			fmt.Sprintf("%s://%s-%d.%s.svc.cluster.local:%d", strings.ToUpper(iListener.Name), crName, broker.Id, namespace, iListener.ContainerPort))
	}

	return listenerConfig
}
