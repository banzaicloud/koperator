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
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var kafkaConfigTemplate = `
{{ .ListenerConfig }}

zookeeper.connect={{ .ZookeeperConnectString }}

{{ if .KafkaCluster.Spec.ListenersConfig.SSLSecrets }}

ssl.keystore.location={{ .ServerKeystorePath }}/{{ .KeystoreFile }}
ssl.truststore.location={{ .ServerKeystorePath }}/{{ .KeystoreFile }}
ssl.keystore.password={{ .ServerKeystorePassword }}
ssl.truststore.password={{ .ServerKeystorePassword }}
ssl.client.auth=required

{{ if .SSLEnabledForInternalCommunication }}

cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.truststore.location={{ .ClientKeystorePath }}/{{ .KeystoreFile }}
cruise.control.metrics.reporter.ssl.keystore.location={{ .ClientKeystorePath }}/{{ .KeystoreFile }}
cruise.control.metrics.reporter.ssl.keystore.password={{ .ClientKeystorePassword }}

{{ end }}
{{ end }}

metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
cruise.control.metrics.reporter.bootstrap.servers={{ .CruiseControlBootstrapServers }}
broker.id={{ .Id }}

{{ if .StorageConfig }}
log.dirs={{ .StorageConfig }}
{{ end }}

{{ .AdvertisedListenersConfig }}

super.users={{ .SuperUsers }}
`

func (r *Reconciler) getConfigString(bConfig *v1beta1.BrokerConfig, id int32, loadBalancerIP, serverPass, clientPass string, superUsers []string, log logr.Logger) string {
	var out bytes.Buffer
	t := template.Must(template.New("bConfig-config").Parse(kafkaConfigTemplate))
	if err := t.Execute(&out, map[string]interface{}{
		"KafkaCluster":                       r.KafkaCluster,
		"Id":                                 id,
		"ListenerConfig":                     generateListenerSpecificConfig(&r.KafkaCluster.Spec.ListenersConfig, log),
		"SSLEnabledForInternalCommunication": r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners),
		"ZookeeperConnectString":             strings.Join(r.KafkaCluster.Spec.ZKAddresses, ","),
		"CruiseControlBootstrapServers":      strings.Join(getInternalListeners(r.KafkaCluster.Spec.ListenersConfig.InternalListeners, id, r.KafkaCluster.Namespace, r.KafkaCluster.Name, r.KafkaCluster.Spec.HeadlessServiceEnabled), ","),
		"StorageConfig":                      generateStorageConfig(bConfig.StorageConfigs),
		"AdvertisedListenersConfig":          generateAdvertisedListenerConfig(id, r.KafkaCluster.Spec.ListenersConfig, loadBalancerIP, r.KafkaCluster.Namespace, r.KafkaCluster.Name, r.KafkaCluster.Spec.HeadlessServiceEnabled),
		"SuperUsers":                         strings.Join(generateSuperUsers(superUsers), ";"),
		"ServerKeystorePath":                 serverKeystorePath,
		"ClientKeystorePath":                 clientKeystorePath,
		"KeystoreFile":                       v1alpha1.TLSJKSKey,
		"ServerKeystorePassword":             serverPass,
		"ClientKeystorePassword":             clientPass,
	}); err != nil {
		log.Error(err, "error occurred during parsing the config template")
	}
	return out.String()
}

func generateSuperUsers(users []string) (suStrings []string) {
	suStrings = make([]string, 0)
	for _, x := range users {
		suStrings = append(suStrings, fmt.Sprintf("User:%s", x))
	}
	return
}

func (r *Reconciler) configMap(id int32, brokerConfig *v1beta1.BrokerConfig, loadBalancerIP, serverPass, clientPass string, superUsers []string, log logr.Logger) runtime.Object {
	return &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id), util.MergeLabels(labelsForKafka(r.KafkaCluster.Name), map[string]string{"brokerId": fmt.Sprintf("%d", id)}), r.KafkaCluster),
		Data:       map[string]string{"broker-config": r.generateBrokerConfig(id, brokerConfig, loadBalancerIP, serverPass, clientPass, superUsers, log)},
	}
}

func generateAdvertisedListenerConfig(id int32, l v1beta1.ListenersConfig, loadBalancerIP, namespace, crName string, headlessServiceEnabled bool) string {
	advertisedListenerConfig := []string{}
	for _, eListener := range l.ExternalListeners {
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s:%d", strings.ToUpper(eListener.Name), loadBalancerIP, eListener.ExternalStartingPort+id))
	}
	for _, iListener := range l.InternalListeners {
		if headlessServiceEnabled {
			advertisedListenerConfig = append(advertisedListenerConfig,
				fmt.Sprintf("%s://%s-%d.%s-headless.%s.svc.cluster.local:%d", strings.ToUpper(iListener.Name), crName, id, crName, namespace, iListener.ContainerPort))
		} else {
			advertisedListenerConfig = append(advertisedListenerConfig,
				fmt.Sprintf("%s://%s-%d.%s.svc.cluster.local:%d", strings.ToUpper(iListener.Name), crName, id, namespace, iListener.ContainerPort))
		}
	}
	return fmt.Sprintf("advertised.listeners=%s\n", strings.Join(advertisedListenerConfig, ","))
}

func generateStorageConfig(sConfig []v1beta1.StorageConfig) string {
	mountPaths := []string{}
	for _, storage := range sConfig {
		mountPaths = append(mountPaths, storage.MountPath+`/kafka`)
	}
	return strings.Join(mountPaths, ",")
}

func generateListenerSpecificConfig(l *v1beta1.ListenersConfig, log logr.Logger) string {

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

func getInternalListeners(iListeners []v1beta1.InternalListenerConfig, id int32, namespace, crName string, headlessServiceEnabled bool) []string {

	listenerConfig := []string{}

	if headlessServiceEnabled {
		for _, iListener := range iListeners {
			listenerConfig = append(listenerConfig,
				fmt.Sprintf("%s://%s-%d.%s-headless.%s.svc.cluster.local:%d", strings.ToUpper(iListener.Name), crName, id, crName, namespace, iListener.ContainerPort))
		}
	} else {
		for _, iListener := range iListeners {
			listenerConfig = append(listenerConfig,
				fmt.Sprintf("%s://%s-%d.%s.svc.cluster.local:%d", strings.ToUpper(iListener.Name), crName, id, namespace, iListener.ContainerPort))
		}
	}

	return listenerConfig
}

func (r Reconciler) generateBrokerConfig(id int32, brokerConfig *v1beta1.BrokerConfig, loadBalancerIP, serverPass, clientPass string, superUsers []string, log logr.Logger) string {
	parsedReadOnlyClusterConfig := util.ParsePropertiesFormat(r.KafkaCluster.Spec.ReadOnlyConfig)

	parsedReadOnlyBrokerConfig := util.ParsePropertiesFormat(r.KafkaCluster.Spec.Brokers[id].ReadOnlyConfig)

	if err := mergo.Merge(&parsedReadOnlyBrokerConfig, parsedReadOnlyClusterConfig); err != nil {
		log.Error(err, "error occurred during merging readonly configs")
	}

	//Generate the Complete Configuration for the Broker
	completeConfigMap := map[string]string{}

	if err := mergo.Merge(&completeConfigMap, util.ParsePropertiesFormat(r.getConfigString(brokerConfig, id, loadBalancerIP, serverPass, clientPass, superUsers, log))); err != nil {
		log.Error(err, "error occurred during merging operator generated configs")
	}

	if err := mergo.Merge(&completeConfigMap, parsedReadOnlyBrokerConfig); err != nil {
		log.Error(err, "error occurred during merging readOnly config to complete configs")
	}

	completeConfig := []string{}

	for key, value := range completeConfigMap {
		completeConfig = append(completeConfig, fmt.Sprintf("%s=%s", key, value))
	}

	// We need to sort the config every time to avoid diffs occurred because of ranging through map
	sort.Strings(completeConfig)

	return strings.Join(completeConfig, "\n")
}
