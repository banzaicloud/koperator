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

package tests

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

func expectCruiseControl(kafkaCluster *v1beta1.KafkaCluster) {
	expectCruiseControlTopic(kafkaCluster)
	expectCruiseControlService(kafkaCluster)
	expectCruiseControlConfigMap(kafkaCluster)
	expectCruiseControlDeployment(kafkaCluster)
}

func expectCruiseControlTopic(kafkaCluster *v1beta1.KafkaCluster) {
	createdKafkaCluster := &v1beta1.KafkaCluster{}
	err := k8sClient.Get(context.TODO(), types.NamespacedName{
		Name:      kafkaCluster.Name,
		Namespace: kafkaCluster.Namespace,
	}, createdKafkaCluster)
	Expect(err).NotTo(HaveOccurred())
	Expect(createdKafkaCluster.Status.CruiseControlTopicStatus).To(Equal(v1beta1.CruiseControlTopicReady))

	topic := &v1alpha1.KafkaTopic{}
	Eventually(func() error {
		topicObjName := fmt.Sprintf("%s-cruise-control-topic", kafkaCluster.Name)
		return k8sClient.Get(context.Background(), types.NamespacedName{Namespace: kafkaCluster.Namespace, Name: topicObjName}, topic)
	}).Should(Succeed())

	Expect(topic).NotTo(BeNil())
	Expect(topic.Labels).To(HaveKeyWithValue("app", "kafka"))
	Expect(topic.Labels).To(HaveKeyWithValue("clusterName", kafkaCluster.Name))
	Expect(topic.Labels).To(HaveKeyWithValue("clusterNamespace", kafkaCluster.Namespace))

	Expect(topic.Spec).To(Equal(v1alpha1.KafkaTopicSpec{
		Name:              "__CruiseControlMetrics",
		Partitions:        7,
		ReplicationFactor: 2,
		ClusterRef: v1alpha1.ClusterReference{
			Name:      kafkaCluster.Name,
			Namespace: kafkaCluster.Namespace,
		},
	}))
}

func expectCruiseControlService(kafkaCluster *v1beta1.KafkaCluster) {
	service := &corev1.Service{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-cruisecontrol-svc", kafkaCluster.Name),
		}, service)
	}).Should(Succeed())

	Expect(service.Labels).To(HaveKeyWithValue("app", "cruisecontrol"))
	Expect(service.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
	Expect(service.Spec.Ports).To(ConsistOf(
		corev1.ServicePort{
			Name:       "cc",
			Protocol:   "TCP",
			Port:       8090,
			TargetPort: intstr.FromInt(8090),
		},
		corev1.ServicePort{
			Name:       "metrics",
			Protocol:   "TCP",
			Port:       9020,
			TargetPort: intstr.FromInt(9020),
		},
	))
	Expect(service.Spec.Selector).To(HaveKeyWithValue("kafka_cr", "kafkacluster-1"))
	Expect(service.Spec.Selector).To(HaveKeyWithValue("app", "cruisecontrol"))
}

func expectCruiseControlConfigMap(kafkaCluster *v1beta1.KafkaCluster) {
	configMap := &corev1.ConfigMap{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-cruisecontrol-config", kafkaCluster.Name),
		}, configMap)
	}).Should(Succeed())

	Expect(configMap.Labels).To(HaveKeyWithValue("app", "cruisecontrol"))
	Expect(configMap.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))

	Expect(configMap.Data).To(HaveKeyWithValue("cruisecontrol.properties", fmt.Sprintf(`bootstrap.servers=%s-all-broker.%s.%s:29092
some.config=value
zookeeper.connect=/
`, kafkaCluster.Name, kafkaCluster.Namespace, "svc.cluster.local")))
	Expect(configMap.Data).To(HaveKeyWithValue("capacity.json", `{
    "brokerCapacities": [
        {
            "brokerId": "0",
            "capacity": {
                "DISK": {
                    "/kafka-logs/kafka": "10737418240"
                },
                "CPU": "150",
                "NW_IN": "125000",
                "NW_OUT": "125000"
            },
            "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
        },
        {
            "brokerId": "1",
            "capacity": {
                "DISK": {
                    "/kafka-logs/kafka": "10737418240"
                },
                "CPU": "150",
                "NW_IN": "125000",
                "NW_OUT": "125000"
            },
            "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
        },
        {
            "brokerId": "2",
            "capacity": {
                "DISK": {
                    "/kafka-logs/kafka": "10737418240"
                },
                "CPU": "150",
                "NW_IN": "125000",
                "NW_OUT": "125000"
            },
            "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
        },
        {
            "brokerId": "-1",
            "capacity": {
                "DISK": {
                    "/kafka-logs/kafka": "10737418240"
                },
                "CPU": "100",
                "NW_IN": "125000",
                "NW_OUT": "125000"
            },
            "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
        }
    ]
}`))
	Expect(configMap.Data).To(HaveKeyWithValue("clusterConfigs.json", ""))
	Expect(configMap.Data).To(HaveKeyWithValue("log4j.properties", `rootLogger.level=INFO
appenders=console, kafkaCruiseControlAppender, operationAppender, requestAppender

property.filename=./logs

appender.console.type=Console
appender.console.name=STDOUT
appender.console.layout.type=PatternLayout
appender.console.layout.pattern=[%d] %p %m (%c)%n

appender.kafkaCruiseControlAppender.type=RollingFile
appender.kafkaCruiseControlAppender.name=kafkaCruiseControlFile
appender.kafkaCruiseControlAppender.fileName=${filename}/kafkacruisecontrol.log
appender.kafkaCruiseControlAppender.filePattern=${filename}/kafkacruisecontrol.log.%d{yyyy-MM-dd-HH}
appender.kafkaCruiseControlAppender.layout.type=PatternLayout
appender.kafkaCruiseControlAppender.layout.pattern=[%d] %p %m (%c)%n
appender.kafkaCruiseControlAppender.policies.type=Policies
appender.kafkaCruiseControlAppender.policies.time.type=TimeBasedTriggeringPolicy
appender.kafkaCruiseControlAppender.policies.time.interval=1

appender.operationAppender.type=RollingFile
appender.operationAppender.name=operationFile
appender.operationAppender.fileName=${filename}/kafkacruisecontrol-operation.log
appender.operationAppender.filePattern=${filename}/kafkacruisecontrol-operation.log.%d{yyyy-MM-dd}
appender.operationAppender.layout.type=PatternLayout
appender.operationAppender.layout.pattern=[%d] %p [%c] %m %n
appender.operationAppender.policies.type=Policies
appender.operationAppender.policies.time.type=TimeBasedTriggeringPolicy
appender.operationAppender.policies.time.interval=1

appender.requestAppender.type=RollingFile
appender.requestAppender.name=requestFile
appender.requestAppender.fileName=${filename}/kafkacruisecontrol-request.log
appender.requestAppender.filePattern=${filename}/kafkacruisecontrol-request.log.%d{yyyy-MM-dd-HH}
appender.requestAppender.layout.type=PatternLayout
appender.requestAppender.layout.pattern=[%d] %p %m (%c)%n
appender.requestAppender.policies.type=Policies
appender.requestAppender.policies.time.type=TimeBasedTriggeringPolicy
appender.requestAppender.policies.time.interval=1

# Loggers
logger.cruisecontrol.name=com.linkedin.kafka.cruisecontrol
logger.cruisecontrol.level=info
logger.cruisecontrol.appenderRef.kafkaCruiseControlAppender.ref=kafkaCruiseControlFile

logger.detector.name=com.linkedin.kafka.cruisecontrol.detector
logger.detector.level=info
logger.detector.appenderRef.kafkaCruiseControlAppender.ref=kafkaCruiseControlFile

logger.operationLogger.name=operationLogger
logger.operationLogger.level=info
logger.operationLogger.appenderRef.operationAppender.ref=operationFile

logger.CruiseControlPublicAccessLogger.name=CruiseControlPublicAccessLogger
logger.CruiseControlPublicAccessLogger.level=info
logger.CruiseControlPublicAccessLogger.appenderRef.requestAppender.ref=requestFile

rootLogger.appenderRefs=console, kafkaCruiseControlAppender
rootLogger.appenderRef.console.ref=STDOUT
rootLogger.appenderRef.kafkaCruiseControlAppender.ref=kafkaCruiseControlFile
`))
}

func expectCruiseControlDeployment(kafkaCluster *v1beta1.KafkaCluster) {
	deployment := &appsv1.Deployment{}
	deploymentName := fmt.Sprintf("%s-cruisecontrol", kafkaCluster.Name)
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      deploymentName,
		}, deployment)
	}).Should(Succeed())

	Expect(deployment.Labels).To(HaveKeyWithValue("app", "cruisecontrol"))
	Expect(deployment.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))

	Expect(deployment.Spec.Selector).NotTo(BeNil())
	Expect(deployment.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", "cruisecontrol"))
	Expect(deployment.Spec.Selector.MatchLabels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))

	Expect(deployment.Spec.Template.Annotations).To(HaveKey("cruiseControlCapacity.json"))
	Expect(deployment.Spec.Template.Annotations).To(HaveKey("cruiseControlClusterConfig.json"))
	Expect(deployment.Spec.Template.Annotations).To(HaveKey("cruiseControlConfig.json"))
	Expect(deployment.Spec.Template.Annotations).To(HaveKey("cruiseControlLogConfig.json"))

	// init container
	Expect(deployment.Spec.Template.Spec.InitContainers).To(HaveLen(1))
	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	Expect(initContainer.Name).To(Equal("jmx-exporter"))
	Expect(initContainer.Image).To(Equal("ghcr.io/banzaicloud/jmx-javaagent:0.14.0"))
	Expect(initContainer.Command).To(Equal([]string{"cp", "/opt/jmx_exporter/jmx_prometheus_javaagent-0.14.0.jar", "/opt/jmx-exporter/jmx_prometheus.jar"}))
	Expect(initContainer.VolumeMounts).To(ConsistOf(corev1.VolumeMount{
		Name:      "jmx-jar-data",
		MountPath: "/opt/jmx-exporter/",
	}))

	// container
	Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal(deploymentName))
	Expect(container.Env).To(ConsistOf(corev1.EnvVar{Name: "KAFKA_OPTS", Value: "-javaagent:/opt/jmx-exporter/jmx_prometheus.jar=9020:/etc/jmx-exporter/config.yaml"}))
	Expect(container.Lifecycle).NotTo(BeNil())
	Expect(container.Lifecycle.PreStop).NotTo(BeNil())
	Expect(container.Lifecycle.PreStop.Exec).NotTo(BeNil())
	Expect(container.Image).To(Equal("ghcr.io/banzaicloud/cruise-control:2.5.37"))
	Expect(container.Ports).To(ConsistOf(
		corev1.ContainerPort{
			ContainerPort: 8090,
			Protocol:      corev1.ProtocolTCP,
		},
		corev1.ContainerPort{
			ContainerPort: 9020,
			Protocol:      corev1.ProtocolTCP,
		},
	))
	Expect(container.Resources).To(Equal(corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("512Mi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("512Mi"),
		},
	}))
	Expect(container.ReadinessProbe).NotTo(BeNil())
	Expect(container.ReadinessProbe.Handler).To(Equal(
		corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(8090),
			},
		}))
	Expect(container.VolumeMounts).To(ConsistOf(
		corev1.VolumeMount{
			Name:      fmt.Sprintf("%s-cruisecontrol-config", kafkaCluster.Name),
			MountPath: "/opt/cruise-control/config",
		},
		corev1.VolumeMount{
			Name:      "jmx-jar-data",
			MountPath: "/opt/jmx-exporter/",
		},
		corev1.VolumeMount{
			Name:      fmt.Sprintf("%s-cc-jmx-exporter", kafkaCluster.Name),
			MountPath: "/etc/jmx-exporter/",
		}))

	Expect(deployment.Spec.Template.Spec.Volumes).To(ConsistOf(
		corev1.Volume{
			Name: fmt.Sprintf("%s-cruisecontrol-config", kafkaCluster.Name),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-cruisecontrol-config", kafkaCluster.Name)},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		},
		corev1.Volume{
			Name: fmt.Sprintf("%s-cc-jmx-exporter", kafkaCluster.Name),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-cc-jmx-exporter", kafkaCluster.Name)},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		},
		corev1.Volume{
			Name: "jmx-jar-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	))

	// Check if the securityContext values are propagated correctly
	Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(Equal(util.BoolPointer(false)))
	Expect(container.SecurityContext.Privileged).To(Equal(util.BoolPointer(true)))
}
