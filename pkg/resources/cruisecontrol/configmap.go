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

	"github.com/banzaicloud/kafka-operator/pkg/resources/kafka"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) configMap(log logr.Logger) runtime.Object {
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(configAndVolumeName, labelSelector, r.KafkaCluster),
		Data: map[string]string{
			"cruisecontrol.properties": r.KafkaCluster.Spec.CruiseControlConfig.Config + fmt.Sprintf(`
    # The Kafka cluster to control.
    bootstrap.servers=%s:%d
    # The zookeeper connect of the Kafka cluster
    zookeeper.connect=%s/
`, fmt.Sprintf(kafka.AllBrokerServiceTemplate, r.KafkaCluster.Name), r.KafkaCluster.Spec.ListenersConfig.InternalListeners[0].ContainerPort, strings.Join(r.KafkaCluster.Spec.ZKAddresses, ",")) +
				generateSSLConfig(&r.KafkaCluster.Spec.ListenersConfig),
			"capacity.json":       r.KafkaCluster.Spec.CruiseControlConfig.CapacityConfig,
			"clusterConfigs.json": r.KafkaCluster.Spec.CruiseControlConfig.ClusterConfigs,
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

func generateSSLConfig(l *banzaicloudv1alpha1.ListenersConfig) (res string) {
	if l.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(l.InternalListeners) {
		res = `
security.protocol=SSL
ssl.truststore.location=/var/run/secrets/java.io/keystores/client.truststore.jks
ssl.keystore.location=/var/run/secrets/java.io/keystores/client.keystore.jks
`
	}
	return
}
