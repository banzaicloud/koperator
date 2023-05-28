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

import (
	"github.com/gruntwork-io/terratest/modules/k8s"

	. "github.com/onsi/ginkgo/v2"
)

// requireInstallingPrometheusOperator deploys prometheus-operator Helm chart
// and checks the success of that operation.
func requireInstallingPrometheusOperator(kubectlOptions *k8s.KubectlOptions, prometheusOperatorVersion Version) {
	When("Installing prometheus-operator", func() {
		requireInstallingPrometheusOperatorHelmChartIfDoesNotExist(kubectlOptions, prometheusOperatorVersion)
	})
}

// requireDeployingCertManagerHelmChart checks the existence of the cert-manager
// Helm release and installs it if it's not present.
func requireInstallingPrometheusOperatorHelmChartIfDoesNotExist(
	kubectlOptions *k8s.KubectlOptions,
	prometheusOperatorVersion Version,
) {
	It("Installing prometheus-operator Helm chart", func() {
		installHelmChartIfDoesNotExist(
			kubectlOptions,
			"https://prometheus-community.github.io/helm-charts",
			"kube-prometheus-stack",
			prometheusOperatorVersion,
			"prometheus-operator",
			map[string]string{
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
		)

		By("Verifying prometheus-operator pods")
		requireRunningPods(kubectlOptions, "app", "kube-prometheus-stack-operator")
	})
}

func requireUninstallingPrometheusOperator(kubectlOptions *k8s.KubectlOptions) {
	When("Uninstalling prometheus-operator", Ordered, func() {
		requireUninstallingPrometheusOperatorHelmChart(kubectlOptions)
		requireRemovePrometheusOperatorCRDs(kubectlOptions)
	})
}

func requireUninstallingPrometheusOperatorHelmChart(kubectlOptions *k8s.KubectlOptions) {
	It("Uninstalling prometheus-operator Helm chart", func() {
		uninstallHelmChartIfExist(kubectlOptions, "prometheus-operator", true)
	})
}

// requireRemoveKoperatorCRDs deletes the prometheus-operator CRDs
func requireRemovePrometheusOperatorCRDs(kubectlOptions *k8s.KubectlOptions) {
	It("Removing prometheus-operator CRDs", func() {
		crds := []string{
			"alertmanagerconfigs.monitoring.coreos.com",
			"alertmanagers.monitoring.coreos.com",
			"probes.monitoring.coreos.com",
			"prometheuses.monitoring.coreos.com",
			"prometheusrules.monitoring.coreos.com",
			"servicemonitors.monitoring.coreos.com",
			"thanosrulers.monitoring.coreos.com",
			"podmonitors.monitoring.coreos.com",
		}

		for _, crd := range crds {
			deleteK8sResourceGlobalNoErr(kubectlOptions, []string{"--timeout=" + defaultDeletionTimeout}, "crds", crd)
		}
	})
}
