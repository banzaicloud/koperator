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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	koperator_v1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

// requireApplyingKoperatorSampleResource deploys the specified sample resource (config/samples).
// The full path of the manifest also can be specified.
// It supports different versions that can be specified with the koperatorVersion parameter.
func requireApplyingKoperatorSampleResource(kubectlOptions *k8s.KubectlOptions, koperatorVersion Version, sampleFile string) {
	It("Applying Koperator sample resource", func() {
		By(fmt.Sprintf("Retrieving Koperator sample resource: '%s' with version: '%s' ", sampleFile, koperatorVersion))

		sampleFileSplit := strings.Split(sampleFile, "/")
		Expect(sampleFileSplit).ShouldNot(HaveLen(0))

		var err error
		var rawKoperatorSampleResource []byte

		switch koperatorVersion {
		case LocalVersion:
			if len(sampleFileSplit) == 1 {
				sampleFile = fmt.Sprintf("../../config/samples/%s", sampleFile)
			}

			rawKoperatorSampleResource, err = os.ReadFile(sampleFile)
			Expect(err).NotTo(HaveOccurred())
		default:
			httpClient := new(http.Client)
			httpClient.Timeout = 5 * time.Second

			Expect(sampleFileSplit).Should(HaveLen(1))

			response, err := httpClient.Get("https://raw.githubusercontent.com/banzaicloud/koperator/" + koperatorVersion + "/config/samples/" + sampleFile)
			if response != nil {
				defer func() { _ = response.Body.Close() }()
			}

			Expect(err).NotTo(HaveOccurred())

			rawKoperatorSampleResource, err = io.ReadAll(response.Body)

			Expect(err).NotTo(HaveOccurred())
		}

		By(fmt.Sprintf("Applying K8s manifest %s", sampleFile))
		k8s.KubectlApplyFromString(GinkgoT(), kubectlOptions, string(rawKoperatorSampleResource))

	})
}

// requireRemoveKoperatorCRDs deletes the koperator CRDs
func requireRemoveKoperatorCRDs(kubectlOptions *k8s.KubectlOptions) {
	It("Removing koperator CRDs", func() {
		for _, crd := range koperatorCRDs() {
			deleteK8sResourceGlobalNoErrNotFound(kubectlOptions, "", "crds", crd)
		}
	})
}

// requireApplyingKoperatorCRDs deploys the koperator CRDs and checks their
// existence afterwards.
func requireApplyingKoperatorCRDs(kubectlOptions *k8s.KubectlOptions, koperatorVersion Version) {
	It("Applying koperator CRDs", func() {
		By(fmt.Sprintf("Retrieving Koperator CRDs (to work around too long CRD) with version %s ", koperatorVersion))
		// Note: had to go lower because none of apply, create, replace works
		// properly with the KafkaCluster CRD due to the size (apply cannot
		// handle the size, create is not idempotent, replace is only idempotent
		// if the resources are existing already). Tried dropping the
		// descriptions in the CRDs, but still too large
		var rawKoperatorCRDsManifest []byte
		var err error

		switch koperatorVersion {
		case LocalVersion:
			rawKoperatorCRDsManifest = []byte(helm.RenderTemplate(
				GinkgoT(),
				&helm.Options{
					SetValues: map[string]string{
						"crd.enabled": "true",
					},
				},
				"../../charts/kafka-operator",
				"dummy",
				[]string{"templates/crds.yaml"},
			))
		default:
			httpClient := new(http.Client)
			httpClient.Timeout = 5 * time.Second

			response, err := httpClient.Get("https://github.com/banzaicloud/koperator/releases/download/" + koperatorVersion + "/kafka-operator.crds.yaml")
			if response != nil {
				defer func() { _ = response.Body.Close() }()
			}

			Expect(err).NotTo(HaveOccurred())

			rawKoperatorCRDsManifest, err = io.ReadAll(response.Body)

			Expect(err).NotTo(HaveOccurred())
		}

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
				By("Applying koperator CRDs with version " + koperatorVersion)
				applyK8sResourceManifest(kubectlOptions, tempFile)
			}
		}

		By("Verifying koperator CRDs")
		requireExistingCRDs(
			kubectlOptions,
			koperatorCRDs()...,
		)
	})
}

// requireInstallingKoperator deploys koperator CRDs and Helm chart and checks
// the success of those operations.
func requireInstallingKoperator(kubectlOptions *k8s.KubectlOptions, koperatorVersion Version) {
	When("Installing koperator", func() {
		requireApplyingKoperatorCRDs(kubectlOptions, koperatorVersion)
		requireInstallingKoperatorHelmChartIfDoesNotExist(kubectlOptions, koperatorVersion)
	})
}

// requireDeployingKoperatorHelmChart checks the existence of the koperator Helm
// release and installs it if it's not present.
func requireInstallingKoperatorHelmChartIfDoesNotExist(
	kubectlOptions *k8s.KubectlOptions,
	koperatorVersion Version,
) {
	It("Installing koperator Helm chart", func() {
		switch koperatorVersion {
		case LocalVersion:
			installHelmChartIfDoesNotExist(kubectlOptions, "", "../../charts/kafka-operator", "", "kafka-operator", nil)
		default:
			installHelmChartIfDoesNotExist(
				kubectlOptions,
				"https://kubernetes-charts.banzaicloud.com",
				"kafka-operator",
				koperatorVersion,
				"kafka-operator",
				nil,
			)
		}

		By("Verifying koperator pods")
		//requireRunningPods(kubectlOptions, "app.kubernetes.io/name", "kafka-operator")
	})
}

// requireUninstallingKoperator uninstall koperator Helm chart and removes Koperator's CRDs
// the success of those operations.
func requireUninstallingKoperator(kubectlOptions *k8s.KubectlOptions) {
	When("Uninstalling Koperator", Ordered, func() {
		requireUninstallingKoperatorHelmChart(kubectlOptions)
		requireRemoveKoperatorCRDs(kubectlOptions)
	})
}

// requireUninstallingKoperatorHelmChart uninstall Koperator Helm chart
// and checks the success of that operation.
func requireUninstallingKoperatorHelmChart(kubectlOptions *k8s.KubectlOptions) {
	It("Uninstalling Koperator Helm chart", func() {
		uninstallHelmChartIfExist(kubectlOptions, "kafka-operator", true)
		By("Verifying Koperator helm chart resources cleanup")
		k8sCRDs := listK8sAllResourceType(kubectlOptions)
		remainedRes := getK8sResources(kubectlOptions,
			k8sCRDs,
			"app.kubernetes.io/managed-by=Helm,app.kubernetes.io/instance=kafka-operator",
			"",
			kubectlArgGoTemplateKindNameNamespace,
			"--all-namespaces")
		Expect(remainedRes).Should(BeNil())
	})
}

// requireUninstallKafkaCluster uninstall the Kafka cluster
func requireUninstallKafkaCluster(kubectlOptions *k8s.KubectlOptions, name string) {
	When("Uninstalling Kafka cluster", Ordered, func() {
		requireDeleteKafkaCluster(kubectlOptions, name)

	})
}

// requireDeleteKafkaCluster deletes KafkaCluster resource and
// checks the removal of the Kafka cluster related resources
func requireDeleteKafkaCluster(kubectlOptions *k8s.KubectlOptions, name string) {
	It("Delete KafkaCluster custom resource", func() {
		deleteK8sResourceGlobalNoErrNotFound(kubectlOptions, "", "kafkacluster", "kafka")
		Eventually(context.Background(), func() []string {
			By("Verifying the Kafka cluster resource cleanup")

			k8sCRDs := listK8sAllResourceType(kubectlOptions)
			koperatorCRDsSelected := _stringSlicesUnion(getKoperatorRelatedResourceKinds(), k8sCRDs)
			koperatorCRDsSelected = append(koperatorCRDsSelected, basicK8sCRDs()...)

			return getK8sResources(kubectlOptions,
				koperatorCRDsSelected,
				fmt.Sprintf("%s=%s", koperator_v1beta1.KafkaCRLabelKey, name),
				"",
				"--all-namespaces", kubectlArgGoTemplateKindNameNamespace)
		}, kafkaClusterResourceCleanupTimeout, 3*time.Millisecond).Should(BeNil())
	})
}
