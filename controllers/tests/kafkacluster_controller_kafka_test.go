// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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
	"strconv"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/kafkamonitoring"
	"github.com/banzaicloud/koperator/pkg/util"
)

func expectKafka(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, randomGenTestNumber uint64) {
	expectKafkaAllBrokerService(ctx, kafkaCluster)
	expectKafkaPDB(ctx, kafkaCluster)
	expectKafkaPVC(ctx, kafkaCluster)

	for _, broker := range kafkaCluster.Spec.Brokers {
		expectKafkaBrokerConfigmap(ctx, kafkaCluster, broker, randomGenTestNumber)
		expectKafkaBrokerService(ctx, kafkaCluster, broker)
		expectKafkaBrokerPod(ctx, kafkaCluster, broker)
	}

	expectKafkaCRStatus(ctx, kafkaCluster)

	// TODO test reconcile PKI?
	// TODO test reconcileKafkaPodDelete
}

func expectKafkaAllBrokerService(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) {
	service := &corev1.Service{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-all-broker", kafkaCluster.Name),
		}, service)
	}).Should(Succeed())

	Expect(service.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(service.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))

	Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
	Expect(service.Spec.SessionAffinity).To(Equal(corev1.ServiceAffinityNone))
	Expect(service.Spec.Selector).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(service.Spec.Selector).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
	Expect(service.Spec.Ports).To(ConsistOf(
		corev1.ServicePort{
			Name:       "tcp-internal",
			Protocol:   corev1.ProtocolTCP,
			Port:       29092,
			TargetPort: intstr.FromInt(29092),
		},
		corev1.ServicePort{
			Name:       "tcp-controller",
			Protocol:   corev1.ProtocolTCP,
			Port:       29093,
			TargetPort: intstr.FromInt(29093),
		},
		corev1.ServicePort{
			Name:       "tcp-test",
			Protocol:   corev1.ProtocolTCP,
			Port:       9094,
			TargetPort: intstr.FromInt(9094),
		}))
}

func expectKafkaPDB(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) {
	// get current CR
	err := k8sClient.Get(ctx, types.NamespacedName{Name: kafkaCluster.Name, Namespace: kafkaCluster.Namespace}, kafkaCluster)
	Expect(err).NotTo(HaveOccurred())

	// set PDB and reset status
	kafkaCluster.Spec.DisruptionBudget = v1beta1.DisruptionBudget{
		Create: true,
		Budget: "20%",
	}
	kafkaCluster.Status = v1beta1.KafkaClusterStatus{}

	// update CR
	err = k8sClient.Update(ctx, kafkaCluster)
	Expect(err).NotTo(HaveOccurred())

	// wait until reconcile finishes
	waitForClusterRunningState(ctx, kafkaCluster, kafkaCluster.Namespace)

	// get created PDB
	pdb := policyv1.PodDisruptionBudget{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-pdb", kafkaCluster.Name),
		}, &pdb)
	}).Should(Succeed())

	// make assertions
	Expect(pdb.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(pdb.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
	Expect(pdb.Spec.MinAvailable).To(Equal(util.IntstrPointer(3)))
	Expect(pdb.Spec.Selector).NotTo(BeNil())
	Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
}

func expectKafkaPVC(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) {
	// get PVCs
	pvcs := corev1.PersistentVolumeClaimList{}
	Eventually(ctx, func() error {
		return k8sClient.List(ctx, &pvcs,
			client.ListOption(client.InNamespace(kafkaCluster.Namespace)),
			client.ListOption(client.MatchingLabels(map[string]string{v1beta1.AppLabelKey: "kafka", v1beta1.KafkaCRLabelKey: kafkaCluster.Name})))
	}).Should(Succeed())

	Expect(pvcs.Items).To(HaveLen(3))
	for i, pvc := range pvcs.Items {
		Expect(pvc.GenerateName).To(Equal(fmt.Sprintf("%s-%d-storage-0-", kafkaCluster.Name, i)))
		Expect(pvc.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
		Expect(pvc.Labels).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(i)))
		Expect(pvc.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
		Expect(pvc.Annotations).To(HaveKeyWithValue("mountPath", "/kafka-logs"))
		Expect(pvc.Spec.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))
		Expect(pvc.Spec.Resources).To(Equal(corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"storage": resource.MustParse("10Gi"),
			},
		}))
	}
}

func expectKafkaBrokerConfigmap(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, broker v1beta1.Broker, randomGenTestNumber uint64) {
	configMap := corev1.ConfigMap{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-config-%d", kafkaCluster.Name, broker.Id),
		}, &configMap)
	}).Should(Succeed())

	Expect(configMap.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(configMap.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
	Expect(configMap.Labels).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(int(broker.Id))))

	Expect(configMap.Data).To(HaveKeyWithValue("broker-config", fmt.Sprintf(`advertised.listeners=TEST://test.host.com:%d,CONTROLLER://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29093,INTERNAL://kafkacluster-%d-%d.kafka-%d.svc.cluster.local:29092
broker.id=%d
control.plane.listener.name=CONTROLLER
cruise.control.metrics.reporter.bootstrap.servers=kafkacluster-1-all-broker.kafka-1.svc.cluster.local:29092
cruise.control.metrics.reporter.kubernetes.mode=true
inter.broker.listener.name=INTERNAL
listener.security.protocol.map=TEST:PLAINTEXT,INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
listeners=TEST://:9094,INTERNAL://:29092,CONTROLLER://:29093
log.dirs=/kafka-logs/kafka,/ephemeral-dir1/kafka
metric.reporters=com.linkedin.kafka.cruisecontrol.metricsreporter.CruiseControlMetricsReporter
zookeeper.connect=/
`, 19090+broker.Id, randomGenTestNumber, broker.Id, randomGenTestNumber, randomGenTestNumber, broker.Id, randomGenTestNumber, broker.Id)))

	// assert log4j?
}

func expectKafkaBrokerService(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, broker v1beta1.Broker) {
	service := corev1.Service{}
	Eventually(ctx, func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      fmt.Sprintf("%s-%d", kafkaCluster.Name, broker.Id),
		}, &service)
	}).Should(Succeed())

	Expect(service.Labels).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(service.Labels).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
	Expect(service.Labels).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(int(broker.Id))))

	Expect(service.Spec.Ports).To(ConsistOf(
		corev1.ServicePort{
			Name:       "tcp-internal",
			Protocol:   corev1.ProtocolTCP,
			Port:       29092,
			TargetPort: intstr.FromInt(29092),
		},
		corev1.ServicePort{
			Name:       "tcp-controller",
			Protocol:   corev1.ProtocolTCP,
			Port:       29093,
			TargetPort: intstr.FromInt(29093),
		},
		corev1.ServicePort{
			Name:       "tcp-test",
			Protocol:   corev1.ProtocolTCP,
			Port:       9094,
			TargetPort: intstr.FromInt(9094),
		},
		corev1.ServicePort{
			Name:       "metrics",
			Protocol:   corev1.ProtocolTCP,
			Port:       9020,
			TargetPort: intstr.FromInt(9020),
		}))

	Expect(service.Spec.Selector).To(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka"))
	Expect(service.Spec.Selector).To(HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name))
	Expect(service.Spec.Selector).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(int(broker.Id))))
	Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
}

func expectKafkaBrokerPod(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster, broker v1beta1.Broker) {
	podList := corev1.PodList{}
	Eventually(ctx, func() ([]corev1.Pod, error) {
		err := k8sClient.List(ctx, &podList,
			client.ListOption(client.InNamespace(kafkaCluster.Namespace)),
			client.ListOption(client.MatchingLabels(map[string]string{v1beta1.AppLabelKey: "kafka", v1beta1.KafkaCRLabelKey: kafkaCluster.Name, v1beta1.BrokerIdLabelKey: strconv.Itoa(int(broker.Id))})))
		return podList.Items, err
	}).Should(HaveLen(1))

	pod := podList.Items[0]

	Expect(pod.GenerateName).To(Equal(fmt.Sprintf("%s-%d-", kafkaCluster.Name, broker.Id)))
	Expect(pod.Labels).To(HaveKeyWithValue(v1beta1.BrokerIdLabelKey, strconv.Itoa(int(broker.Id))))
	getContainerName := func(c corev1.Container) string { return c.Name }
	// test exact order, because if the slice reorders, it triggers another reconcile cycle
	Expect(pod.Spec.InitContainers).To(HaveLen(4))
	Expect(pod.Spec.InitContainers[0]).To(WithTransform(getContainerName, Equal("a-test-initcontainer")))
	Expect(pod.Spec.InitContainers[1]).To(WithTransform(getContainerName, Equal("cruise-control-reporter")))
	Expect(pod.Spec.InitContainers[2]).To(WithTransform(getContainerName, Equal("jmx-exporter")))
	Expect(pod.Spec.InitContainers[3]).To(WithTransform(getContainerName, Equal("test-initcontainer")))

	Expect(pod.Spec.Affinity).NotTo(BeNil())
	Expect(pod.Spec.Affinity.PodAntiAffinity).NotTo(BeNil())

	Expect(pod.Spec.Containers).To(HaveLen(2))
	Expect(pod.Spec.Containers[1]).To(WithTransform(getContainerName, Equal("test-container")))
	container := pod.Spec.Containers[0]
	Expect(container.Name).To(Equal("kafka"))
	Expect(container.Image).To(Equal("ghcr.io/banzaicloud/kafka:2.13-3.1.0"))
	Expect(container.Lifecycle).NotTo(BeNil())
	Expect(container.Lifecycle.PreStop).NotTo(BeNil())
	getEnvName := func(c corev1.EnvVar) string { return c.Name }
	Expect(container.Env).To(ConsistOf(
		// the exact value is not interesting
		WithTransform(getEnvName, Equal("KAFKA_OPTS")),
		WithTransform(getEnvName, Equal("KAFKA_JVM_PERFORMANCE_OPTS")),

		// the exact value should be checked
		corev1.EnvVar{
			Name: "ENVOY_SIDECAR_STATUS",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  `metadata.annotations['sidecar.istio.io/status']`,
				},
			},
		},
		corev1.EnvVar{
			Name:  "KAFKA_HEAP_OPTS",
			Value: "-Xmx2G -Xms2G",
		},
		corev1.EnvVar{
			Name:  "ENVVAR1",
			Value: "VALUE1 VALUE2",
		},
		corev1.EnvVar{
			Name:  "ENVVAR2",
			Value: "VALUE2",
		},
		corev1.EnvVar{
			Name:  "CLASSPATH",
			Value: "/opt/kafka/libs/extensions/*:/test/class/path",
		},
	))
	Expect(container.VolumeMounts).To(HaveLen(9))
	Expect(container.VolumeMounts[0]).To(Equal(corev1.VolumeMount{
		Name:      "a-test-volume",
		MountPath: "/a/test/path",
	}))
	Expect(container.VolumeMounts[1]).To(Equal(corev1.VolumeMount{
		Name:      "broker-config",
		MountPath: "/config",
	}))
	Expect(container.VolumeMounts[2]).To(Equal(corev1.VolumeMount{
		Name:      "exitfile",
		MountPath: "/var/run/wait",
	}))
	Expect(container.VolumeMounts[3]).To(Equal(corev1.VolumeMount{
		Name:      "extensions",
		MountPath: "/opt/kafka/libs/extensions",
	}))
	Expect(container.VolumeMounts[4]).To(Equal(corev1.VolumeMount{
		Name:      "jmx-jar-data",
		MountPath: "/opt/jmx-exporter/",
	}))
	Expect(container.VolumeMounts[5]).To(Equal(corev1.VolumeMount{
		Name:      "kafka-data-0",
		MountPath: "/kafka-logs",
	}))
	Expect(container.VolumeMounts[6]).To(Equal(corev1.VolumeMount{
		Name:      "kafka-data-1",
		MountPath: "/ephemeral-dir1",
	}))

	Expect(container.VolumeMounts[7]).To(Equal(corev1.VolumeMount{
		Name:      fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, kafkaCluster.Name),
		MountPath: "/etc/jmx-exporter/",
	}))
	Expect(container.VolumeMounts[8]).To(Equal(corev1.VolumeMount{
		Name:      "test-volume",
		MountPath: "/test/path",
	}))

	// test exact order, because if the slice reorders, it triggers another reconcile cycle
	Expect(pod.Spec.Volumes).To(HaveLen(9))
	Expect(pod.Spec.Volumes[0]).To(Equal(
		corev1.Volume{
			Name: "a-test-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}))
	Expect(pod.Spec.Volumes[1]).To(Equal(
		corev1.Volume{
			Name: "broker-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-config-%d", kafkaCluster.Name, broker.Id)},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		}))
	Expect(pod.Spec.Volumes[2]).To(Equal(
		corev1.Volume{
			Name: "exitfile",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}))
	Expect(pod.Spec.Volumes[3]).To(Equal(
		corev1.Volume{
			Name: "extensions",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}))
	Expect(pod.Spec.Volumes[4]).To(Equal(
		corev1.Volume{
			Name: "jmx-jar-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}))
	// the name of the PVC is dynamically created - no exact match
	Expect(pod.Spec.Volumes[5]).To(
		WithTransform(func(vol corev1.Volume) string { return vol.Name },
			Equal("kafka-data-0")))
	Expect(pod.Spec.Volumes[6]).To(Equal(
		corev1.Volume{
			Name: "kafka-data-1",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: util.QuantityPointer(resource.MustParse("100Mi")),
				},
			},
		}),
	)
	Expect(pod.Spec.Volumes[7]).To(Equal(
		corev1.Volume{
			Name: fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, kafkaCluster.Name),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, kafkaCluster.Name)},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		}))
	Expect(pod.Spec.Volumes[8]).To(Equal(
		corev1.Volume{
			Name: "test-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}))

	Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
	Expect(pod.Spec.TerminationGracePeriodSeconds).To(Equal(util.Int64Pointer(120)))

	// Check if the securityContext values are propagated correctly
	Expect(pod.Spec.SecurityContext.RunAsNonRoot).To(Equal(util.BoolPointer(false)))
	Expect(container.SecurityContext.Privileged).To(Equal(util.BoolPointer(true)))

	// expect some other fields
}

func expectKafkaCRStatus(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) {
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      kafkaCluster.Name,
		Namespace: kafkaCluster.Namespace,
	}, kafkaCluster)
	Expect(err).NotTo(HaveOccurred())

	Expect(kafkaCluster.Status.State).To(Equal(v1beta1.KafkaClusterRunning))
	Expect(kafkaCluster.Status.AlertCount).To(Equal(0))

	Expect(kafkaCluster.Status.ListenerStatuses).To(Equal(v1beta1.ListenerStatuses{
		InternalListeners: map[string]v1beta1.ListenerStatusList{
			"internal": {
				{
					Name:    "any-broker",
					Address: fmt.Sprintf("%s-all-broker.kafka-1.svc.cluster.local:29092", kafkaCluster.Name),
				},
				{
					Name:    "broker-0",
					Address: fmt.Sprintf("%s-0.kafka-1.svc.cluster.local:29092", kafkaCluster.Name),
				},
				{
					Name:    "broker-1",
					Address: fmt.Sprintf("%s-1.kafka-1.svc.cluster.local:29092", kafkaCluster.Name),
				},
				{
					Name:    "broker-2",
					Address: fmt.Sprintf("%s-2.kafka-1.svc.cluster.local:29092", kafkaCluster.Name),
				},
			},
		},
		ExternalListeners: map[string]v1beta1.ListenerStatusList{
			"test": {
				{
					Name:    "any-broker",
					Address: "test.host.com:29092",
				},
				{
					Name:    "broker-0",
					Address: "test.host.com:19090",
				},
				{
					Name:    "broker-1",
					Address: "test.host.com:19091",
				},
				{
					Name:    "broker-2",
					Address: "test.host.com:19092",
				},
			},
		},
	}))
	for _, brokerState := range kafkaCluster.Status.BrokersState {
		Expect(brokerState.Version).To(Equal("2.7.0"))
		Expect(brokerState.Image).To(Equal(kafkaCluster.Spec.GetClusterImage()))
	}
}
