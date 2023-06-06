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

// requireApplyingCertManagerCRDs deploys the cert-manager CRDs and checks
// their existence afterwards.
func requireApplyingCertManagerCRDs(kubectlOptions *k8s.KubectlOptions, certManagerVersion Version) {
	It("Applying cert-manager CRDs", func() {
		By("Applying cert-manager CRDs with version " + certManagerVersion)
		applyK8sResourceManifest(
			kubectlOptions,
			"https://github.com/jetstack/cert-manager/releases/download/"+certManagerVersion+"/cert-manager.crds.yaml",
		)

		By("Verifying cert-manager CRDs")
		requireExistingCRDs(
			kubectlOptions,
			"certificaterequests.cert-manager.io",
			"certificates.cert-manager.io",
			"challenges.acme.cert-manager.io",
			"clusterissuers.cert-manager.io",
			"issuers.cert-manager.io",
			"orders.acme.cert-manager.io",
		)
	})
}

// requireInstallingCertManager deploys cert-manager CRDs and Helm chart and
// checks the success of those operations.
func requireInstallingCertManager(kubectlOptions *k8s.KubectlOptions, certManagerVersion Version) {
	When("Installing cert-manager", func() {
		requireApplyingCertManagerCRDs(kubectlOptions, certManagerVersion)
		requireInstallingCertManagerHelmChart(kubectlOptions, certManagerVersion)
	})
}

// requireDeployingCertManagerHelmChart checks the existence of the cert-manager
// Helm release and installs it if it's not present.
func requireInstallingCertManagerHelmChart(
	kubectlOptions *k8s.KubectlOptions,
	certManagerVersion string,
) {
	It("Installing cert-manager Helm chart", func() {
		installHelmChart(
			kubectlOptions,
			"https://charts.jetstack.io",
			"cert-manager",
			certManagerVersion,
			"cert-manager",
			nil,
		)

		By("Verifying cert-manager pods")
		requireRunningPods(kubectlOptions, "app.kubernetes.io/name", "cert-manager", "cainjector", "webhook")
	})
}
