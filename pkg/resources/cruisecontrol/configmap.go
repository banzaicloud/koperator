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

package cruisecontrol

import (
	"fmt"
	"strings"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) configMap(log logr.Logger, clientPass string) runtime.Object {
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name), labelSelector, r.KafkaCluster),
		Data: map[string]string{
			"cruisecontrol.properties": r.KafkaCluster.Spec.CruiseControlConfig.Config + fmt.Sprintf(`
    # The Kafka cluster to control.
    bootstrap.servers=%s:%d
    # The zookeeper connect of the Kafka cluster
    zookeeper.connect=%s
`, generateBootstrapServer(r.KafkaCluster.Spec.HeadlessServiceEnabled, r.KafkaCluster.Name), r.KafkaCluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort, prepareZookeeperAddress(r.KafkaCluster.Spec.ZKAddresses)) +
				generateSSLConfig(&r.KafkaCluster.Spec.ListenersConfig, clientPass),
			"capacity.json":       r.KafkaCluster.Spec.CruiseControlConfig.CapacityConfig,
			"clusterConfigs.json": r.KafkaCluster.Spec.CruiseControlConfig.ClusterConfig,
			"log4j.properties": `
log4j.rootLogger = INFO, FILE
    log4j.appender.FILE=org.apache.log4j.FileAppender
    log4j.appender.FILE.File=/dev/stdout
    log4j.appender.FILE.layout=org.apache.log4j.PatternLayout
    log4j.appender.FILE.layout.conversionPattern=%-6r [%15.15t] %-5p %30.30c %x - %m%n
`,
			"log4j2.xml": `
<?xml version="1.0" encoding="UTF-8"?>
    <Configuration status="INFO">
        <Appenders>
            <File name="Console" fileName="/dev/stdout">
                <PatternLayout pattern="%d{yyy-MM-dd HH:mm:ss.SSS} [%t] %-5level %logger{36} - %msg%n"/>
            </File>
        </Appenders>
        <Loggers>
            <Root level="info">
                <AppenderRef ref="Console" />
            </Root>
        </Loggers>
    </Configuration>
`,
		},
	}
	return configMap
}

func generateSSLConfig(l *v1beta1.ListenersConfig, clientPass string) (res string) {
	if l.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(l.InternalListeners) {
		res = fmt.Sprintf(`
security.protocol=SSL
ssl.truststore.location=/var/run/secrets/java.io/keystores/tls.jks
ssl.keystore.location=/var/run/secrets/java.io/keystores/tls.jks
ssl.keystore.password=%s
ssl.truststore.password=%s
`, clientPass, clientPass)
	}
	return
}

func generateBootstrapServer(headlessEnabled bool, clusterName string) string {
	if headlessEnabled {
		return fmt.Sprintf(kafkautils.HeadlessServiceTemplate, clusterName)
	}
	return fmt.Sprintf(kafkautils.AllBrokerServiceTemplate, clusterName)
}

func prepareZookeeperAddress(zkAddresses []string) string {
	preparedAddress := []string{}
	for _, addr := range zkAddresses {
		if strings.Contains(addr, "/") {
			preparedAddress = append(preparedAddress, addr)
		} else {
			preparedAddress = append(preparedAddress, addr+"/")
		}
	}
	return strings.Join(preparedAddress, ",")
}
