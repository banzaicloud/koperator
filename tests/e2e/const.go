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

// Versions.
type Version = string

const (

	// LocalVersion means using the files in the local repository snapshot.
	LocalVersion Version = "local"

	kubectlArgGoTemplateName                              = `-o=go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'`
	kubectlArgGoTemplateKindNameNamespace                 = `-o=go-template='{{range .items}}{{.kind}}{{"/"}}{{.metadata.name}}{{if .metadata.namespace}}{{"."}}{{.metadata.namespace}}{{end}}{{"\n"}}{{end}}'`
	kubectlArgGoTemplateInternalListenersName             = `-o=go-template='{{range $key,$value := .status.listenerStatuses.internalListeners}}{{$key}}{{"\n"}}{{end}}`
	kubectlArgGoTemplateInternalListenerAddressesTemplate = `-o=go-template='{{range .status.listenerStatuses.internalListeners.%s}}{{.address}}{{"\n"}}{{end}}`
	kubectlArgGoTemplateExternalListenersName             = `-o=go-template='{{range $key,$value := .status.listenerStatuses.externalListeners}}{{$key}}{{"\n"}}{{end}}`
	kubectlArgGoTemplateExternalListenerAddressesTemplate = `-o=go-template='{{range .status.listenerStatuses.externalListeners.%s}}{{.address}}{{"\n"}}{{end}}`

	crdKind               = "customresourcedefinitions.apiextensions.k8s.io"
	kafkaKind             = "kafkaclusters.kafka.banzaicloud.io"
	kafkaTopicKind        = "kafkatopics.kafka.banzaicloud.io"
	kafkaUserKind         = "kafkausers.kafka.banzaicloud.io"
	kafkaClusterName      = "kafka"
	kafkaUserName         = "test-user"
	testExternalTopicName = "topic-test-external"
	testInternalTopicName = "topic-test-internal"

	defaultTLSSecretName       = "test-secret"
	kcatName                   = "kcat"
	zookeeperKind              = "zookeeperclusters.zookeeper.pravega.io"
	zookeeperClusterName       = "zookeeper-server"
	managedByHelmLabelTemplate = "app.kubernetes.io/managed-by=Helm,app.kubernetes.io/instance=%s"

	cruiseControlPodReadinessTimeout       = 50 * time.Second
	kafkaClusterResourceReadinessTimeout   = 60 * time.Second
	defaultDeletionTimeout                 = 20 * time.Second
	defaultPodReadinessWaitTime            = 10 * time.Second
	defaultTopicCreationWaitTime           = 10 * time.Second
	defaultUserCreationWaitTime            = 10 * time.Second
	kafkaClusterCreateTimeout              = 600 * time.Second
	kafkaClusterResourceCleanupTimeout     = 120 * time.Second
	kcatDeleetionTimeout                   = 40 * time.Second
	zookeeperClusterCreateTimeout          = 4 * time.Minute
	zookeeperClusterResourceCleanupTimeout = 60 * time.Second
	externalConsumerTimeout                = 5 * time.Second
	externalProducerTimeout                = 5 * time.Second

	zookeeperClusterReplicaCount = 1

	kcatPodTemplate          = "templates/kcat.yaml.tmpl"
	kafkaTopicTemplate       = "templates/topic.yaml.tmpl"
	kafkaUserTemplate        = "templates/user.yaml.tmpl"
	zookeeperClusterTemplate = "templates/zookeeper_cluster.yaml.tmpl"

	kubectlNotFoundErrorMsg = "NotFound"
)

func apiGroupKoperatorDependencies() map[string]string {
	return map[string]string{
		"cert-manager": "cert-manager.io",
		"zookeeper":    "zookeeper.pravega.io",
		"prometheus":   "monitoring.coreos.com",
	}
}

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
		"persistentvolumeclaims",
		"persistentvolumes",
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

func koperatorRelatedResourceKinds() []string {
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
