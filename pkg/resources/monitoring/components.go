package monitoring

import (
	"fmt"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("monitoring")

func ConfigMapForMonitoring(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(BrokerJmxTemplate, kc.Name),
			Namespace: kc.Namespace,
			Labels:    labelsForJmx(kc.Name),
		},
		Data: map[string]string{"config.yaml": `
    lowercaseOutputName: true
    rules:
    - pattern : kafka.cluster<type=(.+), name=(.+), topic=(.+), partition=(.+)><>Value
      name: kafka_cluster_$1_$2
      labels:
        topic: "$3"
        partition: "$4"
    - pattern : kafka.log<type=Log, name=(.+), topic=(.+), partition=(.+)><>Value
      name: kafka_log_$1
      labels:
        topic: "$2"
        partition: "$3"
    - pattern : kafka.controller<type=(.+), name=(.+)><>(Count|Value)
      name: kafka_controller_$1_$2
    - pattern : kafka.network<type=(.+), name=(.+)><>Value
      name: kafka_network_$1_$2
    - pattern : kafka.network<type=(.+), name=(.+)PerSec, request=(.+)><>Count
      name: kafka_network_$1_$2_total
      labels:
        request: "$3"
    - pattern : kafka.network<type=(.+), name=(\w+), networkProcessor=(.+)><>Count
      name: kafka_network_$1_$2
      labels:
        request: "$3"
      type: COUNTER
    - pattern : kafka.network<type=(.+), name=(\w+), request=(\w+)><>Count
      name: kafka_network_$1_$2
      labels:
        request: "$3"
      type: COUNTER
    - pattern : kafka.network<type=(.+), name=(\w+)><>Count
      name: kafka_network_$1_$2
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+)PerSec\w*, topic=(.+)><>Count
      name: kafka_server_$1_$2_total
      labels:
        topic: "$3"
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+)PerSec\w*><>Count
      name: kafka_server_$1_$2_total
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+), clientId=(.+), topic=(.+), partition=(.*)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        clientId: "$3"
        topic: "$4"
        partition: "$5"
    - pattern : kafka.server<type=(.+), name=(.+), topic=(.+), partition=(.*)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        topic: "$3"
        partition: "$4"
    - pattern : kafka.server<type=(.+), name=(.+), topic=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        topic: "$3"
      type: COUNTER
    - pattern : kafka.server<type=(.+), name=(.+), clientId=(.+), brokerHost=(.+), brokerPort=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        clientId: "$3"
        broker: "$4:$5"
    - pattern : kafka.server<type=(.+), name=(.+), clientId=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
      labels:
        clientId: "$3"
    - pattern : kafka.server<type=(.+), name=(.+)><>(Count|Value)
      name: kafka_server_$1_$2
    - pattern : kafka.(\w+)<type=(.+), name=(.+)PerSec\w*><>Count
      name: kafka_$1_$2_$3_total
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, topic=(.+)><>Count
      name: kafka_$1_$2_$3_total
      labels:
        topic: "$4"
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+)PerSec\w*, topic=(.+), partition=(.+)><>Count
      name: kafka_$1_$2_$3_total
      labels:
        topic: "$4"
        partition: "$5"
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+)><>(Count|Value)
      name: kafka_$1_$2_$3_$4
      type: COUNTER
    - pattern : kafka.(\w+)<type=(.+), name=(.+), (\w+)=(.+)><>(Count|Value)
      name: kafka_$1_$2_$3_$6
      labels:
        "$4": "$5"
`},
	}
	return configMap
}

func labelsForJmx(name string) map[string]string {
	return map[string]string{"app": "kafka-jmx", "kafka_cr": name}
}
