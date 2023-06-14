// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package e2e

import "time"

// HelmDescriptors.
var (
	// certManagerHelmDescriptor describes the cert-manager Helm component.
	certManagerHelmDescriptor = helmDescriptor{
		Repository:   "https://charts.jetstack.io",
		ChartName:    "cert-manager",
		ChartVersion: "v1.11.0",
		ReleaseName:  "cert-manager",
		Namespace:    "cert-manager",
		SetValues: map[string]string{
			"installCRDs": "true",
		},
		RemoteCRDPathVersionTemplate: "https://github.com/jetstack/cert-manager/releases/download/v%s/cert-manager.crds.yaml",
	}

	// koperatorLocalHelmDescriptor describes the Koperator Helm component with
	// a local chart and version.
	koperatorLocalHelmDescriptor = helmDescriptor{
		Repository:   "../../charts/kafka-operator",
		ChartVersion: LocalVersion,
		ReleaseName:  "kafka-operator",
		Namespace:    "kafka",
		SetValues: map[string]string{
			"crd.enabled": "true",
		},
		LocalCRDSubpaths: []string{"templates/crds.yaml"},
		LocalCRDTemplateRenderValues: map[string]string{
			"crd.enabled": "true",
		},
	}

	// koperatorLocalHelmDescriptor describes the Koperator Helm component with
	// a remote latest chart and version.
	koperatorRemoteLatestHelmDescriptor = helmDescriptor{ //nolint:unused // Note: intentional possibly needed in the future for upgrade test.
		Repository:   "https://kubernetes-charts.banzaicloud.com",
		ChartName:    "kafka-operator",
		ChartVersion: "", // Note: empty string translates to latest final version.
		ReleaseName:  "kafka-operator",
		Namespace:    "kafka",
		SetValues: map[string]string{
			"crd.enabled": "true",
		},
		RemoteCRDPathVersionTemplate: "https://github.com/banzaicloud/koperator/releases/download/%s/kafka-operator.crds.yaml",
	}

	// prometheusOperatorHelmDescriptor describes the prometheus-operator Helm
	// component.
	prometheusOperatorHelmDescriptor = helmDescriptor{
		Repository:   "https://prometheus-community.github.io/helm-charts",
		ChartName:    "kube-prometheus-stack",
		ChartVersion: "42.0.1",
		ReleaseName:  "prometheus-operator",
		Namespace:    "prometheus",
		SetValues: map[string]string{
			"prometheusOperator.createCustomResource": "true",
			"defaultRules.enabled":                    "false",
			"alertmanager.enabled":                    "false",
			"grafana.enabled":                         "false",
			"kubeApiServer.enabled":                   "false",
			"kubelet.enabled":                         "false",
			"kubeControllerManager.enabled":           "false",
			"coreDNS.enabled":                         "false",
			"kubeEtcd.enabled":                        "false",
			"kubeScheduler.enabled":                   "false",
			"kubeProxy.enabled":                       "false",
			"kubeStateMetrics.enabled":                "false",
			"nodeExporter.enabled":                    "false",
			"prometheus.enabled":                      "false",
		},
	}

	// zookeeperOperatorHelmDescriptor describes the zookeeper-operator Helm
	// component.
	zookeeperOperatorHelmDescriptor = helmDescriptor{
		Repository:   "https://charts.pravega.io",
		ChartName:    "zookeeper-operator",
		ChartVersion: "0.2.14",
		ReleaseName:  "zookeeper-operator",
		Namespace:    "zookeeper",
		SetValues: map[string]string{
			"crd.create": "true",
		},
	}
)

// Versions.
type Version = string

const (

	// LocalVersion means using the files in the local repository snapshot.
	LocalVersion Version = "local"

	kubectlArgGoTemplateName                              = `-o=go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'`
	kubectlArgGoTemplateKindNameNamespace                 = `-o=go-template='{{range .items}}{{.kind}}{{"/"}}{{.metadata.name}}{{if .metadata.namespace}}{{"."}}{{.metadata.namespace}}{{end}}{{"\n"}}{{end}}'`
	kubectlArgGoTemplateInternalListenersName             = `-o=go-template='{{range $key,$value := .status.listenerStatuses.internalListeners}}{{$key}}{{"\n"}}{{end}}`
	kubectlArgGoTemplateInternalListenerAddressesTemplate = `-o=go-template='{{range .status.listenerStatuses.internalListeners.%s}}{{.address}}{{"\n"}}{{end}}`
	// kubectlArgGoTemplateExternalListenersName             = `-o=go-template='{{range $key,$value := .status.listenerStatuses.externallListeners}}{{$key}}{{"\n"}}{{end}}`
	// kubectlArgGoTemplateExternalListenerAddressesTemplate = `-o=go-template='{{range .status.listenerStatuses.externalListeners.%s}}{{.address}}{{"\n"}}{{end}}`

	crdKind                    = "customresourcedefinitions.apiextensions.k8s.io"
	kafkaKind                  = "kafkaclusters.kafka.banzaicloud.io"
	kafkaTopicKind             = "kafkatopics.kafka.banzaicloud.io"
	kafkaClusterName           = "kafka"
	testTopicName              = "topic-test"
	kcatPodName                = "kcat"
	zookeeperKind              = "zookeeperclusters.zookeeper.pravega.io"
	zookeeperClusterName       = "zookeeper-server"
	managedByHelmLabelTemplate = "app.kubernetes.io/managed-by=Helm,app.kubernetes.io/instance=%s"

	defaultDeletionTimeout                 = 20 * time.Second
	defaultPodReadinessWaitTime            = 10 * time.Second
	defaultTopicCreationWaitTime           = 10 * time.Second
	kafkaClusterResourceCleanupTimeout     = 30 * time.Second
	zookeeperClusterResourceCleanupTimeout = 60 * time.Second
	externalConsumerTimeout                = 5 * time.Second
	externalProducerTimeout                = 5 * time.Second

	kcatPodTemplate    = "templates/kcat.yaml.tmpl"
	kafkaTopicTemplate = "templates/topic.yaml.tmpl"
)

func basicK8sResourceKinds() []string {
	return []string{
		"pods",
		"services",
		"deployments.apps",
		"daemonset.apps",
		"replicasets.apps",
		"statefulsets.apps",
		"secrets",
		"serviceaccounts",
		"configmaps",
		"mutatingwebhookconfigurations.admissionregistration.k8s.io",
		"validatingwebhookconfigurations.admissionregistration.k8s.io",
		"jobs.batch",
		"cronjobs.batch",
		"poddisruptionbudgets.policy",
		"podsecuritypolicies.policy",
		"persistentvolumeclaims",
		"persistentvolumes",
	}
}

func certManagerCRDs() []string {
	return []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"challenges.acme.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"issuers.cert-manager.io",
		"orders.acme.cert-manager.io",
	}
}

func prometheusCRDs() []string {
	return []string{
		"alertmanagerconfigs.monitoring.coreos.com",
		"alertmanagers.monitoring.coreos.com",
		"probes.monitoring.coreos.com",
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"servicemonitors.monitoring.coreos.com",
		"thanosrulers.monitoring.coreos.com",
		"podmonitors.monitoring.coreos.com",
	}
}

func zookeeperCRDs() []string {
	return []string{
		"zookeeperclusters.zookeeper.pravega.io",
	}
}

func koperatorCRDs() []string {
	return []string{
		"kafkatopics.kafka.banzaicloud.io",
		"kafkaclusters.kafka.banzaicloud.io",
		"kafkausers.kafka.banzaicloud.io",
		"cruisecontroloperations.kafka.banzaicloud.io",
	}
}

func getKoperatorRelatedResourceKinds() []string {
	return []string{
		"nodepoollabelsets.labels.banzaicloud.io",
		"kafkatopics.kafka.banzaicloud.io",
		"kafkaclusters.kafka.banzaicloud.io",
		"kafkausers.kafka.banzaicloud.io",
		"cruisecontroloperations.kafka.banzaicloud.io",
		"istiomeshgateways.servicemesh.cisco.com",
		"virtualservices.networking.istio.io",
		"gateways.networking.istio.io",
		"clusterissuers.cert-manager.io",
		"servicemonitors.monitoring.coreos.com",
	}
}
