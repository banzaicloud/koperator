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
	zookeeperutils "github.com/banzaicloud/kafka-operator/pkg/util/zookeeper"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var kafkaConfigTemplate = `
{{ .ListenerReadOnlyConfig }}
{{ if .ControlPlaneListener }}
control.plane.listener.name={{ .ControlPlaneListener }}
{{ end }}

zookeeper.connect={{ .ZookeeperConnectString }}

{{ if .KafkaCluster.Spec.ListenersConfig.SSLSecrets }}
{{ if .SSLEnabledForInternalCommunication }}

cruise.control.metrics.reporter.security.protocol=SSL
cruise.control.metrics.reporter.ssl.truststore.location={{ .ClientKeystorePath }}/{{ .TrustStoreFile }}
cruise.control.metrics.reporter.ssl.keystore.location={{ .ClientKeystorePath }}/{{ .KeyStoreFile }}
cruise.control.metrics.reporter.ssl.keystore.password={{ .ClientKeystorePassword }}

{{ end }}
{{ end }}

metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
cruise.control.metrics.reporter.bootstrap.servers={{ .CruiseControlBootstrapServers }}
cruise.control.metrics.reporter.kubernetes.mode=true
broker.id={{ .Id }}

{{ if .StorageConfig }}
log.dirs={{ .StorageConfig }}
{{ end }}

{{ if .SuperUsers }}
super.users={{ .SuperUsers }}
{{ end }}
`

func (r *Reconciler) getConfigString(bConfig *v1beta1.BrokerConfig, id int32, clientPass string, superUsers []string, log logr.Logger) string {
	var out bytes.Buffer
	t := template.Must(template.New("bConfig-config").Parse(kafkaConfigTemplate))
	if err := t.Execute(&out, map[string]interface{}{
		"KafkaCluster":                       r.KafkaCluster,
		"Id":                                 id,
		"ListenerReadOnlyConfig":             generateListenerSpecificReadOnlyConfig(&r.KafkaCluster.Spec.ListenersConfig, log),
		"SSLEnabledForInternalCommunication": r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(r.KafkaCluster.Spec.ListenersConfig.InternalListeners),
		"ZookeeperConnectString":             zookeeperutils.PrepareConnectionAddress(r.KafkaCluster.Spec.ZKAddresses, r.KafkaCluster.Spec.GetZkPath()),
		"CruiseControlBootstrapServers":      getInternalListener(r.KafkaCluster.Spec.ListenersConfig.InternalListeners, id, r.KafkaCluster.Spec.GetKubernetesClusterDomain(), r.KafkaCluster.Namespace, r.KafkaCluster.Name, r.KafkaCluster.Spec.HeadlessServiceEnabled),
		"StorageConfig":                      generateStorageConfig(bConfig.StorageConfigs),
		"SuperUsers":                         strings.Join(generateSuperUsers(superUsers), ";"),
		"ClientKeystorePath":                 clientKeystorePath,
		"KeyStoreFile":                       v1alpha1.TLSJKSKeyStore,
		"TrustStoreFile":                     v1alpha1.TLSJKSTrustStore,
		"ClientKeystorePassword":             clientPass,
		"ControlPlaneListener":               generateControlPlaneListener(r.KafkaCluster.Spec.ListenersConfig.InternalListeners),
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

func (r *Reconciler) configMap(id int32, brokerConfig *v1beta1.BrokerConfig, clientPass string, superUsers []string, log logr.Logger) runtime.Object {
	brokerConf := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(brokerConfigTemplate+"-%d", r.KafkaCluster.Name, id),
			util.MergeLabels(
				LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{"brokerId": fmt.Sprintf("%d", id)},
			),
			r.KafkaCluster,
		),
		Data: map[string]string{"broker-config": r.generateBrokerConfig(id, brokerConfig, clientPass, superUsers, log)},
	}
	if brokerConfig.Log4jConfig != "" {
		brokerConf.Data["log4j.properties"] = brokerConfig.Log4jConfig
	}
	return brokerConf
}

func generateStorageConfig(sConfig []v1beta1.StorageConfig) string {
	mountPaths := []string{}
	for _, storage := range sConfig {
		mountPaths = append(mountPaths, storage.MountPath+`/kafka`)
	}
	return strings.Join(mountPaths, ",")
}

func generateControlPlaneListener(iListeners []v1beta1.InternalListenerConfig) string {
	controlPlaneListener := ""

	for _, iListener := range iListeners {
		if iListener.UsedForControllerCommunication {
			controlPlaneListener = strings.ToUpper(iListener.Name)
		}
	}

	return controlPlaneListener
}

func generateListenerSpecificReadOnlyConfig(l *v1beta1.ListenersConfig, log logr.Logger) string {
	var interBrokerListenerName string

	for _, iListener := range l.InternalListeners {
		if iListener.UsedForInnerBrokerCommunication {
			if interBrokerListenerName == "" {
				interBrokerListenerName = strings.ToUpper(iListener.Name)
			} else {
				log.Error(errors.New("inter broker listener name already set"), "config error")
			}
		}
	}
	return "inter.broker.listener.name=" + interBrokerListenerName + "\n"
}

func getInternalListener(iListeners []v1beta1.InternalListenerConfig, id int32, domain, namespace, crName string, headlessServiceEnabled bool) string {

	internalListener := ""

	for _, iListener := range iListeners {
		if iListener.UsedForInnerBrokerCommunication {
			if headlessServiceEnabled {
				internalListener = fmt.Sprintf("%s://%s-%d.%s-headless.%s.svc.%s:%d", strings.ToUpper(iListener.Name), crName, id, crName, namespace, domain, iListener.ContainerPort)
			} else {
				internalListener = fmt.Sprintf("%s://%s-%d.%s.svc.%s:%d", strings.ToUpper(iListener.Name), crName, id, namespace, domain, iListener.ContainerPort)
			}
		}
	}

	return internalListener
}

func (r Reconciler) generateBrokerConfig(id int32, brokerConfig *v1beta1.BrokerConfig, clientPass string, superUsers []string, log logr.Logger) string {
	parsedReadOnlyClusterConfig := util.ParsePropertiesFormat(r.KafkaCluster.Spec.ReadOnlyConfig)
	var parsedReadOnlyBrokerConfig = map[string]string{}

	for _, broker := range r.KafkaCluster.Spec.Brokers {
		if broker.Id == id {
			parsedReadOnlyBrokerConfig = util.ParsePropertiesFormat(broker.ReadOnlyConfig)
			break
		}
	}

	if err := mergo.Merge(&parsedReadOnlyBrokerConfig, parsedReadOnlyClusterConfig); err != nil {
		log.Error(err, "error occurred during merging readonly configs")
	}

	//Generate the Complete Configuration for the Broker
	completeConfigMap := map[string]string{}

	if err := mergo.Merge(&completeConfigMap, util.ParsePropertiesFormat(r.getConfigString(brokerConfig, id, clientPass, superUsers, log))); err != nil {
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
