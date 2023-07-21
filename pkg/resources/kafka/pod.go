// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	apiutil "github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/resources/kafkamonitoring"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"
)

var (
	//go:embed wait-for-envoy-sidecar.sh
	envoySidecarScript string
)

func (r *Reconciler) pod(id int32, brokerConfig *v1beta1.BrokerConfig, pvcs []corev1.PersistentVolumeClaim, log logr.Logger) runtime.Object {
	var kafkaBrokerContainerPorts []corev1.ContainerPort
	const kafkaContainerName = "kafka"

	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		if _, ok := r.KafkaCluster.Status.ListenerStatuses.ExternalListeners[eListener.Name]; !ok {
			continue
		}
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(eListener.GetListenerServiceName(), "_", "-"),
			ContainerPort: eListener.ContainerPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	for _, iListener := range r.KafkaCluster.Spec.ListenersConfig.InternalListeners {
		kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
			Name:          strings.ReplaceAll(iListener.GetListenerServiceName(), "_", "-"),
			ContainerPort: iListener.ContainerPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, r.KafkaCluster.Spec.AdditionalPorts...)

	for _, envVar := range r.KafkaCluster.Spec.Envs {
		if envVar.Name == "JMX_PORT" {
			port, err := strconv.ParseInt(envVar.Value, 10, 32)
			if err != nil {
				log.Error(err, "can't parse JMX_PORT environment variable")
			}

			kafkaBrokerContainerPorts = append(kafkaBrokerContainerPorts, corev1.ContainerPort{
				Name:          "jmx",
				ContainerPort: int32(port),
				Protocol:      corev1.ProtocolTCP,
			})

			break
		}
	}

	dataVolume, dataVolumeMount := generateDataVolumeAndVolumeMount(pvcs, brokerConfig.StorageConfigs)

	// TODO remove this bash envoy sidecar checker script once sidecar precedence becomes available to Kubernetes(baluchicken)
	command := []string{"bash", "-c", envoySidecarScript}

	pod := &corev1.Pod{
		ObjectMeta: templates.ObjectMetaWithGeneratedNameAndAnnotations(
			fmt.Sprintf("%s-%d-", r.KafkaCluster.Name, id),
			brokerConfig.GetBrokerLabels(r.KafkaCluster.Name, id),
			brokerConfig.GetBrokerAnnotations(),
			r.KafkaCluster,
		),
		Spec: corev1.PodSpec{
			SecurityContext: brokerConfig.PodSecurityContext,
			InitContainers:  getInitContainers(brokerConfig, r.KafkaCluster.Spec),
			Affinity:        getAffinity(brokerConfig, r.KafkaCluster),
			Containers: append([]corev1.Container{
				{
					Name:  kafkaContainerName,
					Image: util.GetBrokerImage(brokerConfig, r.KafkaCluster.Spec.GetClusterImage()),
					Lifecycle: &corev1.Lifecycle{
						PreStop: &corev1.LifecycleHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"bash", "-c", `
if [[ -n "$ENVOY_SIDECAR_STATUS" ]]; then
  HEALTHYSTATUSCODE="200"
  SC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:15000/ready)
  if [[ "$SC" == "$HEALTHYSTATUSCODE" ]]; then
    kill -s TERM $(pidof java)
  else
    kill -s KILL $(pidof java)
  fi
else
  kill -s TERM $(pidof java)
fi`},
							},
						},
					},
					SecurityContext: brokerConfig.SecurityContext,
					Env: generateEnvConfig(brokerConfig, []corev1.EnvVar{
						{
							Name:  "CLASSPATH",
							Value: "/opt/kafka/libs/extensions/*",
						},
						{
							Name:  "KAFKA_OPTS",
							Value: "-javaagent:/opt/jmx-exporter/jmx_prometheus.jar=9020:/etc/jmx-exporter/config.yaml",
						},
						{
							Name: "ENVOY_SIDECAR_STATUS",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: `metadata.annotations['sidecar.istio.io/status']`,
								},
							},
						},
					}),

					Command: command,
					Ports: append(kafkaBrokerContainerPorts, []corev1.ContainerPort{
						{
							ContainerPort: 9020,
							Protocol:      corev1.ProtocolTCP,
							Name:          "metrics",
						},
					}...),
					VolumeMounts: getVolumeMounts(brokerConfig.VolumeMounts, dataVolumeMount, r.KafkaCluster.Spec, r.KafkaCluster.Name),
					Resources:    *brokerConfig.GetResources(),
				},
			}, brokerConfig.Containers...),
			Volumes:                       getVolumes(brokerConfig.Volumes, dataVolume, r.KafkaCluster.Spec, r.KafkaCluster.Name, id),
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64Pointer(brokerConfig.GetTerminationGracePeriod()),
			ImagePullSecrets:              brokerConfig.GetImagePullSecrets(),
			ServiceAccountName:            brokerConfig.GetServiceAccount(),
			Tolerations:                   brokerConfig.GetTolerations(),
			NodeSelector:                  brokerConfig.GetNodeSelector(),
			PriorityClassName:             brokerConfig.GetPriorityClassName(),
		},
	}
	if r.KafkaCluster.Spec.HeadlessServiceEnabled {
		pod.Spec.Hostname = fmt.Sprintf("%s-%d", r.KafkaCluster.Name, id)
		pod.Spec.Subdomain = fmt.Sprintf(kafkautils.HeadlessServiceTemplate, r.KafkaCluster.Name)
	}

	if r.KafkaCluster.Spec.KraftMode() {
		for i, container := range pod.Spec.Containers {
			if container.Name == kafkaContainerName {
				// in KRaft mode, all broker nodes within the same Kafka cluster need to use the same cluster ID to format the storage
				pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env,
					corev1.EnvVar{
						Name:  "CLUSTER_ID",
						Value: r.KafkaCluster.Status.ClusterID,
					},
				)
				// see how this env var is used in wait-for-envoy-sidecars.sh
				storageMountPaths := brokerConfig.GetStorageMountPaths()
				if storageMountPaths != "" {
					pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env,
						corev1.EnvVar{
							Name:  "LOG_DIRS",
							Value: brokerConfig.GetStorageMountPaths(),
						},
					)
				}
				break
			}
		}
	}

	return pod
}

func getInitContainers(brokerConfig *v1beta1.BrokerConfig, kafkaClusterSpec v1beta1.KafkaClusterSpec) []corev1.Container {
	initContainers := make([]corev1.Container, 0, len(brokerConfig.InitContainers))
	initContainers = append(initContainers, brokerConfig.InitContainers...)

	initContainers = append(initContainers, []corev1.Container{
		{
			Name:    "cruise-control-reporter",
			Image:   util.GetBrokerMetricsReporterImage(brokerConfig, kafkaClusterSpec),
			Command: []string{"/bin/sh", "-cex", "cp -v /opt/cruise-control/cruise-control/build/dependant-libs/cruise-control-metrics-reporter.jar /opt/kafka/libs/extensions/cruise-control-metrics-reporter.jar"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "extensions",
				MountPath: "/opt/kafka/libs/extensions",
			}},
			Resources: k8sutil.GetDefaultInitContainerResourceRequirements(),
		},
		{
			Name:    "jmx-exporter",
			Image:   kafkaClusterSpec.MonitoringConfig.GetImage(),
			Command: []string{"cp", kafkaClusterSpec.MonitoringConfig.GetPathToJar(), "/opt/jmx-exporter/jmx_prometheus.jar"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      jmxVolumeName,
					MountPath: jmxVolumePath,
				},
			},
			Resources: k8sutil.GetDefaultInitContainerResourceRequirements(),
		},
	}...)

	sort.Slice(initContainers, func(i, j int) bool {
		return initContainers[i].Name < initContainers[j].Name
	})

	return initContainers
}

func getVolumeMounts(brokerConfigVolumeMounts, dataVolumeMount []corev1.VolumeMount,
	kafkaClusterSpec v1beta1.KafkaClusterSpec, kafkaClusterName string) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0, len(brokerConfigVolumeMounts))
	volumeMounts = append(volumeMounts, brokerConfigVolumeMounts...)

	volumeMounts = append(volumeMounts, dataVolumeMount...)

	if kafkaClusterSpec.IsClientSSLSecretPresent() {
		volumeMounts = append(volumeMounts, generateVolumeMountForClientSSLCerts())
	}

	volumeMounts = append(volumeMounts, generateVolumeMountForListenerCerts(kafkaClusterSpec.ListenersConfig)...)
	volumeMounts = append(volumeMounts, []corev1.VolumeMount{
		{
			Name:      brokerConfigMapVolumeMount,
			MountPath: "/config",
		},
		{
			Name:      "extensions",
			MountPath: "/opt/kafka/libs/extensions",
		},
		{
			Name:      jmxVolumeName,
			MountPath: jmxVolumePath,
		},
		{
			Name:      fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, kafkaClusterName),
			MountPath: "/etc/jmx-exporter/",
		},
		{
			Name:      "exitfile",
			MountPath: "/var/run/wait",
		},
	}...)

	sort.Slice(volumeMounts, func(i, j int) bool {
		return volumeMounts[i].Name < volumeMounts[j].Name
	})

	return volumeMounts
}

func getVolumes(brokerConfigVolumes, dataVolume []corev1.Volume, kafkaClusterSpec v1beta1.KafkaClusterSpec, kafkaClusterName string, id int32) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(brokerConfigVolumes))
	// clone the brokerConfig volumes
	volumes = append(volumes, brokerConfigVolumes...)
	volumes = append(volumes, dataVolume...)

	if kafkaClusterSpec.IsClientSSLSecretPresent() {
		volumes = append(volumes, generateVolumeForClientSSLCert(kafkaClusterSpec, kafkaClusterName))
	}

	volumes = append(volumes, generateVolumesForListenerCerts(kafkaClusterSpec.ListenersConfig, kafkaClusterName)...)
	volumes = append(volumes, []corev1.Volume{
		{
			Name: "exitfile",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: brokerConfigMapVolumeMount,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(brokerConfigTemplate+"-%d", kafkaClusterName, id)},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		},
		{
			Name: "extensions",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, kafkaClusterName),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf(kafkamonitoring.BrokerJmxTemplate, kafkaClusterName)},
					DefaultMode:          util.Int32Pointer(0644),
				},
			},
		},
		{
			Name: jmxVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}...)

	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})

	return volumes
}

// getAffinity returns a default `v1.Affinity` which is generated regarding the `OneBrokerPerNode` value
// or if there is any user Affinity definition provided by the user the latter will be used ignoring the value of `OneBrokerPerNode`
func getAffinity(bc *v1beta1.BrokerConfig, cluster *v1beta1.KafkaCluster) *corev1.Affinity {
	if bc.Affinity == nil {
		return &corev1.Affinity{PodAntiAffinity: generatePodAntiAffinity(cluster.Name, cluster.Spec.OneBrokerPerNode)}
	}
	return bc.Affinity
}

func generatePodAntiAffinity(clusterName string, hardRuleEnabled bool) *corev1.PodAntiAffinity {
	podAntiAffinity := corev1.PodAntiAffinity{}
	if hardRuleEnabled {
		podAntiAffinity = corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: apiutil.LabelsForKafka(clusterName),
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		}
	} else {
		podAntiAffinity = corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: int32(100),
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: apiutil.LabelsForKafka(clusterName),
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
	}
	return &podAntiAffinity
}

func generateDataVolumeAndVolumeMount(pvcs []corev1.PersistentVolumeClaim, storageConfigs []v1beta1.StorageConfig) ([]corev1.Volume, []corev1.VolumeMount) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// PVC volume mount (pvcs are already sorted by pvc name in 'getCreatedPvcForBroker' function
	for i, pvc := range pvcs {
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf(kafkaDataVolumeMount+"-%d", i),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf(kafkaDataVolumeMount+"-%d", i),
			MountPath: pvc.Annotations["mountPath"],
		})
	}

	// emptyDir volume mounts
	var emptyDirs []v1beta1.StorageConfig
	for i := range storageConfigs {
		if storageConfigs[i].PvcSpec == nil && storageConfigs[i].EmptyDir != nil {
			// pvcSpec has priority over emptyDir and pvcs are handled already above
			emptyDirs = append(emptyDirs, storageConfigs[i])
		}
	}
	sort.Slice(emptyDirs, func(i, j int) bool {
		return emptyDirs[i].MountPath < emptyDirs[j].MountPath
	})

	offset := len(volumes)
	for i := range emptyDirs {
		j := i + offset
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf(kafkaDataVolumeMount+"-%d", j),
			VolumeSource: corev1.VolumeSource{
				EmptyDir: emptyDirs[i].EmptyDir,
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf(kafkaDataVolumeMount+"-%d", j),
			MountPath: emptyDirs[i].MountPath,
		})
	}

	return volumes, volumeMounts
}

func generateVolumeForListenersCertsFromCommonSpec(commonSpec v1beta1.CommonListenerSpec, clusterName string) corev1.Volume {
	// Use default one if custom has not specified
	secretName := fmt.Sprintf(pkicommon.BrokerServerCertTemplate, clusterName)
	if commonSpec.GetServerSSLCertSecretName() != "" {
		secretName = commonSpec.GetServerSSLCertSecretName()
	}
	return corev1.Volume{
		Name: fmt.Sprintf(listenerSSLCertVolumeNameTemplate, commonSpec.Name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: util.Int32Pointer(0644),
			},
		},
	}
}

func generateVolumesForListenerCerts(listenerConfig v1beta1.ListenersConfig, clusterName string) (ret []corev1.Volume) {
	for _, iListener := range listenerConfig.InternalListeners {
		if iListener.CommonListenerSpec.Type != v1beta1.SecurityProtocolSSL {
			continue
		}
		ret = append(ret, generateVolumeForListenersCertsFromCommonSpec(iListener.CommonListenerSpec, clusterName))
	}
	for _, eListener := range listenerConfig.ExternalListeners {
		if eListener.CommonListenerSpec.Type != v1beta1.SecurityProtocolSSL {
			continue
		}
		ret = append(ret, generateVolumeForListenersCertsFromCommonSpec(eListener.CommonListenerSpec, clusterName))
	}
	return ret
}

func generateVolumeForClientSSLCert(kafkaClusterSpec v1beta1.KafkaClusterSpec, clusterName string) (ret corev1.Volume) {
	// Use default one if custom has not specified
	clientSecretName := fmt.Sprintf(pkicommon.BrokerControllerTemplate, clusterName)
	if kafkaClusterSpec.GetClientSSLCertSecretName() != "" {
		clientSecretName = kafkaClusterSpec.GetClientSSLCertSecretName()
	}
	return corev1.Volume{
		Name: clientKeystoreVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  clientSecretName,
				DefaultMode: util.Int32Pointer(0644),
			},
		},
	}
}

func generateVolumeMountForListenerCerts(listenerConfig v1beta1.ListenersConfig) (ret []corev1.VolumeMount) {
	for _, iListener := range listenerConfig.InternalListeners {
		if iListener.CommonListenerSpec.Type != v1beta1.SecurityProtocolSSL {
			continue
		}
		vm := corev1.VolumeMount{
			Name:      fmt.Sprintf(listenerSSLCertVolumeNameTemplate, iListener.Name),
			MountPath: fmt.Sprintf(listenerServerKeyStorePathTemplate, serverKeystorePath, iListener.CommonListenerSpec.Name),
		}
		ret = append(ret, vm)
	}
	for _, eListener := range listenerConfig.ExternalListeners {
		if eListener.CommonListenerSpec.Type != v1beta1.SecurityProtocolSSL {
			continue
		}
		vm := corev1.VolumeMount{
			Name:      fmt.Sprintf(listenerSSLCertVolumeNameTemplate, eListener.Name),
			MountPath: fmt.Sprintf(listenerServerKeyStorePathTemplate, serverKeystorePath, eListener.CommonListenerSpec.Name),
		}
		ret = append(ret, vm)
	}
	return ret
}

func generateVolumeMountForClientSSLCerts() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      clientKeystoreVolume,
		MountPath: clientKeystorePath,
	}
}

func generateEnvConfig(brokerConfig *v1beta1.BrokerConfig, defaultEnvVars []corev1.EnvVar) []corev1.EnvVar {
	envs := map[string]corev1.EnvVar{}

	for _, v := range defaultEnvVars {
		envs[v.Name] = v
	}

	// merge the env variables
	for _, envVar := range brokerConfig.Envs {
		envVarName := strings.TrimSpace(envVar.Name)
		switch {
		default:
			fallthrough
		case envVar.ValueFrom != nil:
			// append/prepend based on sources is not supported
			envVar.Name = envVarName
			envs[envVarName] = envVar
		case strings.HasPrefix(envVarName, "+"):
			// prepend mode
			envVarName = envVarName[1:]
			if envVarFromMap, ok := envs[envVarName]; ok {
				envVarFromMap.Value = envVar.Value + envVarFromMap.Value
				envs[envVarName] = envVarFromMap
			} else {
				envVar.Name = envVarName
				envs[envVarName] = envVar
			}
		case strings.HasSuffix(envVarName, "+"):
			// append mode
			envVarName = envVarName[:len(envVarName)-1]
			if envVarFromMap, ok := envs[envVarName]; ok {
				envVarFromMap.Value += envVar.Value
				envs[envVarName] = envVarFromMap
			} else {
				envVar.Name = envVarName
				envs[envVarName] = envVar
			}
		}
	}

	if brokerConfig.Log4jConfig != "" {
		envs["KAFKA_LOG4J_OPTS"] = corev1.EnvVar{
			Name:  "KAFKA_LOG4J_OPTS",
			Value: "-Dlog4j.configuration=file:/config/log4j.properties",
		}
	}

	if _, ok := envs["KAFKA_HEAP_OPTS"]; !ok || brokerConfig.KafkaHeapOpts != "" {
		envs["KAFKA_HEAP_OPTS"] = corev1.EnvVar{
			Name:  "KAFKA_HEAP_OPTS",
			Value: brokerConfig.GetKafkaHeapOpts(),
		}
	}

	if _, ok := envs["KAFKA_JVM_PERFORMANCE_OPTS"]; !ok || brokerConfig.KafkaJVMPerfOpts != "" {
		envs["KAFKA_JVM_PERFORMANCE_OPTS"] = corev1.EnvVar{
			Name:  "KAFKA_JVM_PERFORMANCE_OPTS",
			Value: brokerConfig.GetKafkaPerfJmvOpts(),
		}
	}
	// Sort map values by key to avoid diff in sequence
	keys := make([]string, 0, len(envs))

	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var mergedEnv []corev1.EnvVar
	for _, k := range keys {
		mergedEnv = append(mergedEnv, envs[k])
	}

	return mergedEnv
}
