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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// requireApplyingKoperatorCRDs deploys the koperator CRDs and checks their
// existence afterwards.
func requireApplyingKoperatorCRDs(kubectlOptions *k8s.KubectlOptions, koperatorVersion string) {
	It("Applying koperator CRDs", func() {
		By(fmt.Sprintf("Retrieving Koperator CRDs (to work around too long CRD) with version %s ", koperatorVersion))
		// Note: had to go lower because none of apply, create, replace works
		// properly with the KafkaCluster CRD due to the size (apply cannot
		// handle the size, create is not idempotent, replace is only idempotent
		// if the resources are existing already). Tried dropping the
		// descriptions in the CRDs, but still too large
		httpClient := new(http.Client)
		httpClient.Timeout = 5 * time.Second

		response, err := httpClient.Get("https://github.com/banzaicloud/koperator/releases/download/" + koperatorVersion + "/kafka-operator.crds.yaml")
		if response != nil {
			defer func() { _ = response.Body.Close() }()
		}

		Expect(err).NotTo(HaveOccurred())

		rawKoperatorCRDsManifest, err := io.ReadAll(response.Body)

		Expect(err).NotTo(HaveOccurred())

		rawKoperatorCRDs := bytes.Split(rawKoperatorCRDsManifest, []byte("\n---\n"))
		for rawKoperatorCRDIndex, rawKoperatorCRD := range rawKoperatorCRDs {
			rawKoperatorCRDs[rawKoperatorCRDIndex] = bytes.Trim(rawKoperatorCRD, "\n-")
		}

		allowedCRDByteCount := 262144
		tempDir := os.TempDir()
		tempFile := path.Join(tempDir, "manifest.yaml")
		for _, rawKoperatorCRD := range rawKoperatorCRDs {
			err = os.WriteFile(tempFile, rawKoperatorCRD, 0o777)

			Expect(err).NotTo(HaveOccurred())

			if len(rawKoperatorCRD) > allowedCRDByteCount { // Note: too long CRDs cannot be applied, only created or replaced.
				var koperatorCRD apiextensionsv1.CustomResourceDefinition
				err := yaml.Unmarshal(rawKoperatorCRD, &koperatorCRD)

				Expect(err).NotTo(HaveOccurred())

				createOrReplaceK8sResourcesFromManifest(kubectlOptions, "crd", koperatorCRD.GetName(), tempFile, false)
			} else {
				By("Applying cert-manager CRDs with version " + koperatorVersion)
				applyK8sResourceManifest(kubectlOptions, tempFile)
			}
		}

		By("Verifying koperator CRDs")
		requireExistingCRDs(
			kubectlOptions,
			"cruisecontroloperations.kafka.banzaicloud.io",
			"kafkaclusters.kafka.banzaicloud.io",
			"kafkatopics.kafka.banzaicloud.io",
			"kafkausers.kafka.banzaicloud.io",
		)
	})
}

// requireInstallingKoperator deploys koperator CRDs and Helm chart and checks
// the success of those operations.
func requireInstallingKoperator(kubectlOptions *k8s.KubectlOptions, koperatorVersion string) {
	When("Installing koperator", func() {
		requireApplyingKoperatorCRDs(kubectlOptions, koperatorVersion)
		requireInstallingKoperatorHelmChartIfDoesNotExist(kubectlOptions, koperatorVersion)
	})
}

// requireDeployingKoperatorHelmChart checks the existence of the koperator Helm
// release and installs it if it's not present.
func requireInstallingKoperatorHelmChartIfDoesNotExist(
	kubectlOptions *k8s.KubectlOptions,
	koperatorVersion string,
) {
	It("Installing koperator Helm chart", func() {
		installHelmChartIfDoesNotExist(
			kubectlOptions,
			"https://kubernetes-charts.banzaicloud.com",
			"kafka-operator",
			koperatorVersion,
			"kafka-operator",
			nil,
		)

		By("Verifying koperator pods")
		requireRunningPods(kubectlOptions, "app.kubernetes.io/name", "kafka-operator")
	})
}
