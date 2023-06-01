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

type Version = string

const (

	// LocalVersion means using the files in the local repository snapshot.
	LocalVersion Version = "local"

	kubectlArgGoTemplateName              = `-o=go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'`
	kubectlArgGoTemplateKindNameNamespace = `-o=go-template='{{range .items}}{{.kind}}{{"/"}}{{.metadata.name}}{{if .metadata.namespace}}{{"."}}{{.metadata.namespace}}{{end}}{{"\n"}}{{end}}'`

	kafkaKind        = "kafkaclusters.kafka.banzaicloud.io"
	kafkaClusterName = "kafka"

	defaultDeletionTimeout                 = "10s"
	kafkaClusterResourceCleanupTimeout     = 30 * time.Second
	zookeeperClusterResourceCleanupTimeout = 60 * time.Second

	zookeeperKind        = "zookeeperclusters.zookeeper.pravega.io"
	zookeeperClusterName = "zookeeper-server"
)

func basicK8sCRDs() []string {
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
