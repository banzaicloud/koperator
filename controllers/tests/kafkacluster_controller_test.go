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
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

var _ = Describe("KafkaCluster", func() {
	var (
		count                    uint64 = 0
		namespace                string
		namespaceObj             *corev1.Namespace
		kafkaCluster             *v1beta1.KafkaCluster
		loadBalancerServiceName  string
		externalListenerHostName string
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaCluster = createMinimalKafkaClusterCR(fmt.Sprintf("kafkacluster-%d", count), namespace)
		kafkaCluster.Spec.ListenersConfig.ExternalListeners[0].HostnameOverride = ""
		kafkaCluster.Spec.CruiseControlConfig = v1beta1.CruiseControlConfig{
			TopicConfig: &v1beta1.TopicConfig{
				Partitions:        7,
				ReplicationFactor: 2,
			},
			Config:                   "some.config=value",
			CruiseControlAnnotations: map[string]string{"test-cc-ann": "test-cc-ann-val"},
		}
		kafkaCluster.Spec.ReadOnlyConfig = ""
		// Set some Kafka pod and container related SecurityContext values
		defaultGroup := kafkaCluster.Spec.BrokerConfigGroups[defaultBrokerConfigGroup]
		defaultGroup.PodSecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: util.BoolPointer(false),
		}
		defaultGroup.SecurityContext = &corev1.SecurityContext{
			Privileged: util.BoolPointer(true),
		}
		defaultGroup.InitContainers = []corev1.Container{
			{
				Name:  "test-initcontainer",
				Image: "busybox:latest",
			},
			{
				Name:  "a-test-initcontainer",
				Image: "test/image:latest",
			},
		}
		defaultGroup.Containers = []corev1.Container{
			{
				Name:  "test-container",
				Image: "busybox:latest",
			},
		}
		defaultGroup.Volumes = []corev1.Volume{
			{
				Name: "test-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "a-test-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
		defaultGroup.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "test-volume",
				MountPath: "/test/path",
			},
			{
				Name:      "a-test-volume",
				MountPath: "/a/test/path",
			},
		}
		defaultGroup.Envs = []corev1.EnvVar{
			{
				Name:  "ENVVAR1+",
				Value: " VALUE2",
			},
			{
				Name:  "ENVVAR2",
				Value: "VALUE2",
			},
			{
				Name:  "CLASSPATH+",
				Value: ":/test/class/path",
			},
		}
		kafkaCluster.Spec.BrokerConfigGroups[defaultBrokerConfigGroup] = defaultGroup
		// Set some CruiseControl pod and container related SecurityContext values
		kafkaCluster.Spec.CruiseControlConfig.PodSecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: util.BoolPointer(false),
		}
		kafkaCluster.Spec.CruiseControlConfig.SecurityContext = &corev1.SecurityContext{
			Privileged: util.BoolPointer(true),
		}
		kafkaCluster.Spec.EnvoyConfig.Annotations = map[string]string{
			"envoy-annotation-key": "envoy-annotation-value",
		}
		kafkaCluster.Spec.Envs = []corev1.EnvVar{
			{
				Name:  "ENVVAR1",
				Value: "VALUE1",
			},
			{
				Name:  "ENVVAR2+",
				Value: "VALUE1",
			},
		}
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		// assign host to envoy LB
		envoyLBService := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      loadBalancerServiceName,
				Namespace: namespace,
			}, envoyLBService)
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		envoyLBService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{
			Hostname: externalListenerHostName,
		}}

		err = k8sClient.Status().Update(context.TODO(), envoyLBService)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		// in the tests the CC topic might not get deleted

		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		kafkaCluster = nil
	})
	When("using default configuration", func() {
		BeforeEach(func() {
			loadBalancerServiceName = fmt.Sprintf("envoy-loadbalancer-test-%s", kafkaCluster.Name)
			externalListenerHostName = "test.host.com"
		})
		It("should reconciles objects properly", func() {
			expectEnvoy(kafkaCluster, []string{"test"})
			expectKafkaMonitoring(kafkaCluster)
			expectCruiseControlMonitoring(kafkaCluster)
			expectKafka(kafkaCluster, count)
			expectCruiseControl(kafkaCluster)
		})
	})
	When("configuring one ingress envoy controller config inside the external listener without bindings", func() {
		BeforeEach(func() {
			testExternalListener := kafkaCluster.Spec.ListenersConfig.ExternalListeners[0]
			testExternalListener.Config = &v1beta1.Config{
				DefaultIngressConfig: "az1",
				IngressConfig: map[string]v1beta1.IngressConfig{
					"az1": {EnvoyConfig: &v1beta1.EnvoyConfig{
						Annotations: map[string]string{"zone": "az1"},
					}},
				},
			}
			kafkaCluster.Spec.ListenersConfig.ExternalListeners[0] = testExternalListener
			loadBalancerServiceName = fmt.Sprintf("envoy-loadbalancer-test-az1-%s", kafkaCluster.Name)
			externalListenerHostName = "external.az1.host.com"
		})
		It("should reconcile object properly", func() {
			expectDefaultBrokerSettingsForExternalListenerBinding(kafkaCluster, count)
			expectEnvoy(kafkaCluster, []string{"test-az1"})
		})
	})
	When("configuring two ingress envoy controller config inside the external listener without bindings", func() {
		BeforeEach(func() {
			testExternalListener := kafkaCluster.Spec.ListenersConfig.ExternalListeners[0]
			testExternalListener.Config = &v1beta1.Config{
				DefaultIngressConfig: "az1",
				IngressConfig: map[string]v1beta1.IngressConfig{
					"az1": {EnvoyConfig: &v1beta1.EnvoyConfig{
						Annotations: map[string]string{"zone": "az1"},
					},
					},
					"az2": {EnvoyConfig: &v1beta1.EnvoyConfig{
						Annotations: map[string]string{"zone": "az2"},
					},
					},
				},
			}
			kafkaCluster.Spec.ListenersConfig.ExternalListeners[0] = testExternalListener
			loadBalancerServiceName = fmt.Sprintf("envoy-loadbalancer-test-az1-%s", kafkaCluster.Name)
			externalListenerHostName = "external.az1.host.com"
		})
		It("should reconcile object properly", func() {
			expectDefaultBrokerSettingsForExternalListenerBinding(kafkaCluster, count)
			expectEnvoy(kafkaCluster, []string{"test-az1"})
		})
	})
	When("configuring two ingress envoy controller config inside the external listener without using the default", func() {
		BeforeEach(func() {
			testExternalListener := kafkaCluster.Spec.ListenersConfig.ExternalListeners[0]
			testExternalListener.Config = &v1beta1.Config{
				DefaultIngressConfig: "az1",
				IngressConfig: map[string]v1beta1.IngressConfig{
					"az1": {EnvoyConfig: &v1beta1.EnvoyConfig{
						Annotations: map[string]string{"zone": "az1"},
					},
					},
					"az2": {EnvoyConfig: &v1beta1.EnvoyConfig{
						Annotations: map[string]string{"zone": "az2"},
					},
					},
				},
			}
			kafkaCluster.Spec.ListenersConfig.ExternalListeners[0] = testExternalListener
			defaultBrokerConfig := kafkaCluster.Spec.BrokerConfigGroups["default"]
			defaultBrokerConfig.BrokerIngressMapping = []string{"az2"}
			kafkaCluster.Spec.BrokerConfigGroups["default"] = defaultBrokerConfig
			loadBalancerServiceName = fmt.Sprintf("envoy-loadbalancer-test-az2-%s", kafkaCluster.Name)
			externalListenerHostName = "external.az2.host.com"
		})
		It("should reconcile object properly", func() {
			expectEnvoy(kafkaCluster, []string{"test-az2"})
		})
	})
})

var _ = Describe("KafkaCluster with two config external listener", func() {
	var (
		count        uint64 = 0
		namespace    string
		namespaceObj *corev1.Namespace
		kafkaCluster *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafkaconfigtest-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaCluster = createMinimalKafkaClusterCR(fmt.Sprintf("kafkacluster-%d", count), namespace)
		kafkaCluster.Spec.ListenersConfig.ExternalListeners[0].HostnameOverride = ""
		kafkaCluster.Spec.EnvoyConfig.EnableHealthCheckHttp10 = true
		testExternalListener := kafkaCluster.Spec.ListenersConfig.ExternalListeners[0]
		testExternalListener.Config = &v1beta1.Config{
			DefaultIngressConfig: "az2",
			IngressConfig: map[string]v1beta1.IngressConfig{
				"az1": {EnvoyConfig: &v1beta1.EnvoyConfig{
					Annotations: map[string]string{"zone": "az1"},
				},
				},
				"az2": {EnvoyConfig: &v1beta1.EnvoyConfig{
					Annotations: map[string]string{"zone": "az2"},
				},
				},
			},
		}
		kafkaCluster.Spec.ListenersConfig.ExternalListeners[0] = testExternalListener
	})
	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		// assign host to envoy LB
		envoyLBService := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("envoy-loadbalancer-test-az1-%s", kafkaCluster.Name),
				Namespace: namespace,
			}, envoyLBService)
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		envoyLBService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{
			Hostname: "external.az1.host.com",
		}}

		//logf.Log.V(-1).Info("envoy service updated", "spec", envoyLBService)
		err = k8sClient.Status().Update(context.TODO(), envoyLBService)
		Expect(err).NotTo(HaveOccurred())

		envoyLBService = &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("envoy-loadbalancer-test-az2-%s", kafkaCluster.Name),
				Namespace: namespace,
			}, envoyLBService)
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		envoyLBService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{
			Hostname: "external.az2.host.com",
		}}

		//logf.Log.V(-1).Info("envoy service updated", "spec", envoyLBService)
		err = k8sClient.Status().Update(context.TODO(), envoyLBService)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	When("configuring two ingress envoy controller config inside the external listener using both as bindings", func() {
		BeforeEach(func() {
			kafkaCluster.Spec.Brokers[0].BrokerConfig = &v1beta1.BrokerConfig{BrokerIngressMapping: []string{"az1"}}
			kafkaCluster.Spec.Brokers[1].BrokerConfig = &v1beta1.BrokerConfig{BrokerIngressMapping: []string{"az2"}}
		})
		It("should reconcile object properly", func() {
			expectEnvoyWithConfigAz1(kafkaCluster)
			expectEnvoyWithConfigAz2(kafkaCluster)
			expectBrokerConfigmapForAz1ExternalListener(kafkaCluster, count)
			expectBrokerConfigmapForAz2ExternalListener(kafkaCluster, count)
		})
	})
})

func expectKafkaMonitoring(kafkaCluster *v1beta1.KafkaCluster) {
	configMap := corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-kafka-jmx-exporter", kafkaCluster.Name)
	Eventually(func() error {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: kafkaCluster.Namespace}, &configMap)
		return err
	}).Should(Succeed())

	Expect(configMap.Labels).To(And(HaveKeyWithValue(v1beta1.AppLabelKey, "kafka-jmx"), HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name)))
	Expect(configMap.Data).To(HaveKeyWithValue("config.yaml", Not(BeEmpty())))
}

func expectCruiseControlMonitoring(kafkaCluster *v1beta1.KafkaCluster) {
	configMap := corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-cc-jmx-exporter", kafkaCluster.Name)
	//logf.Log.Info("name", "name", configMapName)
	Eventually(func() error {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: kafkaCluster.Namespace}, &configMap)
		return err
	}).Should(Succeed())

	Expect(configMap.Labels).To(And(HaveKeyWithValue(v1beta1.AppLabelKey, "cruisecontrol-jmx"), HaveKeyWithValue(v1beta1.KafkaCRLabelKey, kafkaCluster.Name)))
	Expect(configMap.Data).To(HaveKeyWithValue("config.yaml", kafkaCluster.Spec.MonitoringConfig.CCJMXExporterConfig))
}
