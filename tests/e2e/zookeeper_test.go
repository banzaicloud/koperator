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
	"context"
	"fmt"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// requireInstallingZookeeperOperator deploys zookeeper-operator Helm chart and
// checks the success of that operation.
func requireInstallingZookeeperOperator(kubectlOptions *k8s.KubectlOptions, certManagerVersion string) {
	When("Installing zookeeper-operator", func() {
		requireInstallingZookeeperOperatorHelmChartIfDoesNotExist(kubectlOptions, certManagerVersion)
	})
}

// requireDeployingCertManagerHelmChart checks the existence of the cert-manager
// Helm release and installs it if it's not present.
func requireInstallingZookeeperOperatorHelmChartIfDoesNotExist(
	kubectlOptions *k8s.KubectlOptions,
	zookeeperOperatorVersion string,
) {
	It("Installing zookeeper-operator Helm chart", func() {
		installHelmChartIfDoesNotExist(
			kubectlOptions,
			"https://charts.pravega.io",
			"zookeeper-operator",
			zookeeperOperatorVersion,
			"zookeeper-operator",
			nil,
		)

		By("Verifying zookeeper-operator pods")
		requireRunningPods(kubectlOptions, "name", "zookeeper-operator")
	})
}

// requireUninstallingZookeeperOperator uninstall Zookeeper-operator Helm chart
// and remove CRDs.
func requireUninstallingZookeeperOperator(kubectlOptions *k8s.KubectlOptions) {
	When("Uninstalling zookeeper-operator", func() {
		requireUninstallingZookeeperOperatorHelmChart(kubectlOptions)
		requireRemoveZookeeperOperatorCRDs(kubectlOptions)
	})
}

// requireUninstallingZookeeperOperatorHelmChart uninstall Zookeeper-operator Helm chart
// and checks the success of that operation.
func requireUninstallingZookeeperOperatorHelmChart(kubectlOptions *k8s.KubectlOptions) {
	It("Uninstalling zookeeper-operator Helm chart", func() {
		uninstallHelmChartIfExist(kubectlOptions, "zookeeper-operator", true)
		By("Verifying Zookeeper-operator helm chart resources cleanup")
		k8sCRDs := listK8sAllResourceType(kubectlOptions)
		remainedRes := getK8sResources(kubectlOptions,
			k8sCRDs,
			fmt.Sprintf(managedByHelmLabelTemplate, "zookeeper-operator"),
			"",
			kubectlArgGoTemplateKindNameNamespace,
			"--all-namespaces")
		Expect(remainedRes).Should(BeEmpty())
	})
}

// requireRemoveZookeeperOperatorCRDs deletes the zookeeper-operator CRDs
func requireRemoveZookeeperOperatorCRDs(kubectlOptions *k8s.KubectlOptions) {
	It("Removing zookeeper-operator CRDs", func() {
		for _, crd := range zookeeperCRDs() {
			deleteK8sResourceGlobalNoErrNotFound(kubectlOptions, defaultDeletionTimeout, "crds", crd)
		}
	})
}

// requireUninstallZookeeperCluster uninstall the Zookeeper cluster
func requireUninstallZookeeperCluster(kubectlOptions *k8s.KubectlOptions, name string) {
	When("Uninstalling Zookeeper cluster", func() {
		requireDeleteZookeeperCluster(kubectlOptions, name)

	})
}

// requireDeleteZookeeperCluster deletes the ZookeeperCluster CR and verify the corresponding resources cleanup
func requireDeleteZookeeperCluster(kubectlOptions *k8s.KubectlOptions, name string) {
	It("Delete ZookeeperCluster custom resource", func() {
		deleteK8sResourceGlobalNoErrNotFound(kubectlOptions, defaultDeletionTimeout, zookeeperKind, name)
		Eventually(context.Background(), func() []string {
			By("Verifying the Zookeeper cluster resource cleanup")

			zookeeperK8sResources := basicK8sCRDs()
			zookeeperK8sResources = append(zookeeperK8sResources, zookeeperCRDs()...)

			return getK8sResources(kubectlOptions,
				zookeeperK8sResources,
				fmt.Sprintf("app=%s", name),
				"",
				"--all-namespaces", kubectlArgGoTemplateKindNameNamespace)
		}, zookeeperClusterResourceCleanupTimeout, 3*time.Millisecond).Should(BeEmpty())
	})
}
