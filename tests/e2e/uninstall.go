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
	"fmt"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// requireUninstallingKoperator uninstall koperator Helm chart and removes Koperator's CRDs.
func requireUninstallingKoperator(kubectlOptions k8s.KubectlOptions) {
	When("Uninstalling Koperator", func() {
		requireUninstallingKoperatorHelmChart(kubectlOptions)
		requireRemoveKoperatorCRDs(kubectlOptions)
		requireRemoveNamespace(kubectlOptions, koperatorLocalHelmDescriptor.Namespace)
	})
}

// requireUninstallingKoperatorHelmChart uninstall Koperator Helm chart
// and checks the success of that operation.
func requireUninstallingKoperatorHelmChart(kubectlOptions k8s.KubectlOptions) {
	It("Uninstalling Koperator Helm chart", func() {
		err := koperatorLocalHelmDescriptor.uninstallHelmChart(kubectlOptions, true)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying Koperator helm chart resources cleanup")
		k8sResourceKinds, err := listK8sResourceKinds(kubectlOptions, "")
		Expect(err).ShouldNot(HaveOccurred())

		koperatorAvailableResourceKinds := stringSlicesInstersect(koperatorCRDs(), k8sResourceKinds)
		koperatorAvailableResourceKinds = append(koperatorAvailableResourceKinds, basicK8sResourceKinds()...)

		remainedResources, err := getK8sResources(kubectlOptions,
			koperatorAvailableResourceKinds,
			fmt.Sprintf(managedByHelmLabelTemplate, koperatorLocalHelmDescriptor.ReleaseName),
			"",
			kubectlArgGoTemplateKindNameNamespace,
			"--all-namespaces")

		Expect(err).ShouldNot(HaveOccurred())
		Expect(remainedResources).Should(BeEmpty())
	})
}

// requireRemoveKoperatorCRDs deletes the koperator CRDs
func requireRemoveKoperatorCRDs(kubectlOptions k8s.KubectlOptions) {
	It("Removing koperator CRDs", func() {
		for _, crd := range koperatorCRDs() {
			err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, crdKind, crd)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}

// requireUninstallingZookeeperOperator uninstall Zookeeper-operator Helm chart
// and remove CRDs.
func requireUninstallingZookeeperOperator(kubectlOptions k8s.KubectlOptions) {
	When("Uninstalling zookeeper-operator", func() {
		requireUninstallingZookeeperOperatorHelmChart(kubectlOptions)
		requireRemoveZookeeperOperatorCRDs(kubectlOptions)
		requireRemoveNamespace(kubectlOptions, zookeeperOperatorHelmDescriptor.Namespace)
	})
}

// requireUninstallingZookeeperOperatorHelmChart uninstall Zookeeper-operator Helm chart
// and checks the success of that operation.
func requireUninstallingZookeeperOperatorHelmChart(kubectlOptions k8s.KubectlOptions) {
	It("Uninstalling zookeeper-operator Helm chart", func() {
		err := zookeeperOperatorHelmDescriptor.uninstallHelmChart(kubectlOptions, true)
		Expect(err).NotTo(HaveOccurred())
		By("Verifying Zookeeper-operator helm chart resources cleanup")

		k8sResourceKinds, err := listK8sResourceKinds(kubectlOptions, "")
		Expect(err).ShouldNot(HaveOccurred())

		zookeeperAvailableResourceKinds := stringSlicesInstersect(dependencyCRDs.Zookeeper(), k8sResourceKinds)
		zookeeperAvailableResourceKinds = append(zookeeperAvailableResourceKinds, basicK8sResourceKinds()...)

		remainedResources, err := getK8sResources(kubectlOptions,
			zookeeperAvailableResourceKinds,
			fmt.Sprintf(managedByHelmLabelTemplate, zookeeperOperatorHelmDescriptor.ReleaseName),
			"",
			kubectlArgGoTemplateKindNameNamespace,
			"--all-namespaces")
		Expect(err).ShouldNot(HaveOccurred())

		Expect(remainedResources).Should(BeEmpty())
	})
}

// requireRemoveZookeeperOperatorCRDs deletes the zookeeper-operator CRDs
func requireRemoveZookeeperOperatorCRDs(kubectlOptions k8s.KubectlOptions) {
	It("Removing zookeeper-operator CRDs", func() {
		for _, crd := range dependencyCRDs.Zookeeper() {
			err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, crdKind, crd)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}

// requireUninstallingPrometheusOperator uninstall prometheus-operator Helm chart and
// remove CRDs.
func requireUninstallingPrometheusOperator(kubectlOptions k8s.KubectlOptions) {
	When("Uninstalling prometheus-operator", func() {
		requireUninstallingPrometheusOperatorHelmChart(kubectlOptions)
		requireRemovePrometheusOperatorCRDs(kubectlOptions)
		requireRemoveNamespace(kubectlOptions, prometheusOperatorHelmDescriptor.Namespace)
	})
}

// requireUninstallingPrometheusOperatorHelmChart uninstall prometheus-operator Helm chart
// and checks the success of that operation.
func requireUninstallingPrometheusOperatorHelmChart(kubectlOptions k8s.KubectlOptions) {
	It("Uninstalling Prometheus-operator Helm chart", func() {
		err := prometheusOperatorHelmDescriptor.uninstallHelmChart(kubectlOptions, true)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying Prometheus-operator helm chart resources cleanup")

		k8sResourceKinds, err := listK8sResourceKinds(kubectlOptions, "")
		Expect(err).ShouldNot(HaveOccurred())

		prometheusAvailableResourceKinds := stringSlicesInstersect(dependencyCRDs.Prometheus(), k8sResourceKinds)
		prometheusAvailableResourceKinds = append(prometheusAvailableResourceKinds, basicK8sResourceKinds()...)

		remainedResources, err := getK8sResources(kubectlOptions,
			prometheusAvailableResourceKinds,
			fmt.Sprintf(managedByHelmLabelTemplate, prometheusOperatorHelmDescriptor.ReleaseName),
			"",
			kubectlArgGoTemplateKindNameNamespace,
			"--all-namespaces")
		Expect(err).ShouldNot(HaveOccurred())

		Expect(remainedResources).Should(BeEmpty())
	})
}

// requireRemovePrometheusOperatorCRDs deletes the Prometheus-operator CRDs
func requireRemovePrometheusOperatorCRDs(kubectlOptions k8s.KubectlOptions) {
	It("Removing prometheus-operator CRDs", func() {
		for _, crd := range dependencyCRDs.Prometheus() {
			err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, crdKind, crd)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}

// requireUninstallingCertManager uninstall Cert-manager Helm chart and
// remove CRDs.
func requireUninstallingCertManager(kubectlOptions k8s.KubectlOptions) {
	When("Uninstalling zookeeper-operator", func() {
		requireUninstallingCertManagerHelmChart(kubectlOptions)
		requireRemoveCertManagerCRDs(kubectlOptions)
		requireRemoveNamespace(kubectlOptions, certManagerHelmDescriptor.Namespace)
	})
}

// requireUninstallingCertManagerHelmChart uninstalls cert-manager helm chart
// and checks the success of that operation.
func requireUninstallingCertManagerHelmChart(kubectlOptions k8s.KubectlOptions) {
	It("Uninstalling Cert-manager Helm chart", func() {
		err := certManagerHelmDescriptor.uninstallHelmChart(kubectlOptions, true)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying Cert-manager helm chart resources cleanup")

		k8sResourceKinds, err := listK8sResourceKinds(kubectlOptions, "")
		Expect(err).ShouldNot(HaveOccurred())

		certManagerAvailableResourceKinds := stringSlicesInstersect(dependencyCRDs.CertManager(), k8sResourceKinds)
		certManagerAvailableResourceKinds = append(certManagerAvailableResourceKinds, basicK8sResourceKinds()...)

		remainedResources, err := getK8sResources(kubectlOptions,
			certManagerAvailableResourceKinds,
			fmt.Sprintf(managedByHelmLabelTemplate, certManagerHelmDescriptor.ReleaseName),
			"",
			kubectlArgGoTemplateKindNameNamespace,
			"--all-namespaces")
		Expect(err).ShouldNot(HaveOccurred())

		Expect(remainedResources).Should(BeEmpty())
	})
}

// requireRemoveKoperatorCRDs deletes the cert-manager CRDs
func requireRemoveCertManagerCRDs(kubectlOptions k8s.KubectlOptions) {
	It("Removing cert-manager CRDs", func() {
		for _, crd := range dependencyCRDs.CertManager() {
			err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, crdKind, crd)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}

// requireRemoveNamespace deletes the indicated namespace object
func requireRemoveNamespace(kubectlOptions k8s.KubectlOptions, namespace string) {
	It(fmt.Sprintf("Removing namespace %s", namespace), func() {
		err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, "namespace", namespace, "--wait")
		Expect(err).ShouldNot(HaveOccurred())
	})
}
