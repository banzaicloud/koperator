// Copyright © 2020 Banzai Cloud
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
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/imdario/mergo"
	"strings"
	"text/template"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/go-logr/logr"
)

var dynamicConfig = `
{{ .ListenerDynamicConfig }}
{{ if .KafkaCluster.Spec.ListenersConfig.SSLSecrets }}
ssl.keystore.location={{ .ServerKeystorePath }}/{{ .KeyStoreFile }}
ssl.truststore.location={{ .ServerKeystorePath }}/{{ .TrustStoreFile }}
ssl.keystore.password={{ .ServerKeystorePassword }}
ssl.truststore.password={{ .ServerKeystorePassword }}
ssl.client.auth=required
{{ end }}
{{ .AdvertisedListenersConfig }}
`

func (r *Reconciler) generateDynamicConfigs(id int32, loadBalancerIPs []string, serverPass string, log logr.Logger) string {
	var out bytes.Buffer
	t := template.Must(template.New("dynamicConfig-config").Parse(dynamicConfig))
	if err := t.Execute(&out, map[string]interface{}{
		"KafkaCluster":              r.KafkaCluster,
		"ListenerDynamicConfig":     generateListenerSpecificDynamicConfig(&r.KafkaCluster.Spec.ListenersConfig),
		"ServerKeystorePath":        serverKeystorePath,
		"ClientKeystorePath":        clientKeystorePath,
		"KeyStoreFile":              v1alpha1.TLSJKSKeyStore,
		"TrustStoreFile":            v1alpha1.TLSJKSTrustStore,
		"ServerKeystorePassword":    serverPass,
		"AdvertisedListenersConfig": generateAdvertisedListenerConfig(id, r.KafkaCluster.Spec.ListenersConfig, loadBalancerIPs, r.KafkaCluster.Spec.GetKubernetesClusterDomain(), r.KafkaCluster.Namespace, r.KafkaCluster.Name, r.KafkaCluster.Spec.HeadlessServiceEnabled),
	}); err != nil {
		log.Error(err, "error occurred during parsing the dynamic config template")
	}
	return out.String()
}

func (r *Reconciler) getDynamicConfigs(brokerId int32, loadBalancerIPs []string, serverPass string, brokerConfig *v1beta1.BrokerConfig, log logr.Logger) map[string]string {
	overridablePerBrokerConfig := util.ParsePropertiesFormat(brokerConfig.Config)
	generatedPerBrokerConfig := util.ParsePropertiesFormat(r.generateDynamicConfigs(brokerId, loadBalancerIPs, serverPass, log))

	if err := mergo.Merge(&generatedPerBrokerConfig, overridablePerBrokerConfig); err != nil {
		log.Error(err, "error occurred during merging per-broker configs")
	}

	return generatedPerBrokerConfig
}

func generateListenerSpecificDynamicConfig(l *v1beta1.ListenersConfig) string {
	var securityProtocolMapConfig []string
	var listenerConfig []string

	for _, iListener := range l.InternalListeners {
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
		"listeners=" + strings.Join(listenerConfig, ",")
}

func generateAdvertisedListenerConfig(id int32, l v1beta1.ListenersConfig, loadBalancerIPs []string, domain, namespace, crName string, headlessServiceEnabled bool) string {
	advertisedListenerConfig := []string{}
	for _, eListener := range l.ExternalListeners {
		// use first element of loadBalancerIPs slice for external listener name
		advertisedListenerConfig = append(advertisedListenerConfig,
			fmt.Sprintf("%s://%s:%d", strings.ToUpper(eListener.Name), loadBalancerIPs[0], eListener.ExternalStartingPort+id))
	}
	for _, iListener := range l.InternalListeners {
		if headlessServiceEnabled {
			advertisedListenerConfig = append(advertisedListenerConfig,
				fmt.Sprintf("%s://%s-%d.%s-headless.%s.svc.%s:%d", strings.ToUpper(iListener.Name), crName, id, crName, namespace, domain, iListener.ContainerPort))
		} else {
			advertisedListenerConfig = append(advertisedListenerConfig,
				fmt.Sprintf("%s://%s-%d.%s.svc.%s:%d", strings.ToUpper(iListener.Name), crName, id, namespace, domain, iListener.ContainerPort))
		}
	}
	return fmt.Sprintf("advertised.listeners=%s\n", strings.Join(advertisedListenerConfig, ","))
}
