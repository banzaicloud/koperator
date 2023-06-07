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
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// requireInstallingKoperatorCRDs deploys the Koperator CRDs and checks their
// existence afterwards.
func requireInstallingKoperatorCRDs(kubectlOptions k8s.KubectlOptions, koperatorVersion Version) {
	It("Installing Koperator CRDs", func() {
		koperatorCRDsPath := ""
		var err error

		switch koperatorVersion {
		case LocalVersion:
			localKoperatorCRDs := []byte(helm.RenderTemplate(
				GinkgoT(),
				&helm.Options{
					SetValues: map[string]string{
						"crd.enabled": "true",
					},
				},
				"../../charts/kafka-operator",
				"kafka-operator",
				[]string{"templates/crds.yaml"},
			))

			koperatorCRDsPath, err = createTempFileFromBytes(localKoperatorCRDs, "", "", 0)

			Expect(err).NotTo(HaveOccurred())
		default:
			koperatorCRDsPath = "https://github.com/banzaicloud/koperator/releases/download/" + koperatorVersion + "/kafka-operator.crds.yaml"
		}

		requireInstallingCRDs(kubectlOptions, koperatorCRDsPath)
	})
}

// requireInstallingKoperator deploys koperator CRDs and Helm chart and checks
// the success of those operations.
func requireInstallingKoperator(kubectlOptions k8s.KubectlOptions, koperatorVersion Version) {
	When("Installing Koperator", func() {
		requireInstallingKoperatorCRDs(kubectlOptions, koperatorVersion)
		requireInstallingKoperatorHelmChart(kubectlOptions, koperatorVersion)
	})
}

// requireDeployingKoperatorHelmChart checks the existence of the Koperator Helm
// release and installs it if it's not present.
func requireInstallingKoperatorHelmChart(kubectlOptions k8s.KubectlOptions, koperatorVersion Version) {
	It("Installing Koperator Helm chart", func() {
		switch koperatorVersion {
		case LocalVersion:
			installHelmChart(kubectlOptions, "", "../../charts/kafka-operator", "", "kafka-operator", nil)
		default:
			installHelmChart(
				kubectlOptions,
				"https://kubernetes-charts.banzaicloud.com",
				"kafka-operator",
				koperatorVersion,
				"kafka-operator",
				nil,
			)
		}
	})
}
