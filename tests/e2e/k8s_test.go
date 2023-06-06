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
	"os"
	"path"
	"strings"

	"emperror.dev/errors"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// applyK8sResourceManifests applies the specified manifest to the provided
// kubectl context and namespace.
func applyK8sResourceManifest(kubectlOptions k8s.KubectlOptions, manifestPath string) {
	By(fmt.Sprintf("Applying k8s manifest %s", manifestPath))
	k8s.KubectlApply(GinkgoT(), &kubectlOptions, manifestPath)
}

// createK8sResourcesFromManifest creates Kubernetes resources from the
// specified manifest to the provided kubectl context and namespace.
func createK8sResourcesFromManifest(kubectlOptions k8s.KubectlOptions, manifestPath string, shouldBeValidated bool) {
	By(fmt.Sprintf("Creating k8s resources from manifest %s", manifestPath))
	k8s.RunKubectl(
		GinkgoT(),
		&kubectlOptions,
		"create",
		fmt.Sprintf("--validate=%t", shouldBeValidated),
		"--filename", manifestPath,
	)
}

// createOrReplaceK8sResourcesFromManifest creates non-existent Kubernetes
// resources or replaces existing ones from the specified manifest to the
// provided kubectl context and namespace.
func createOrReplaceK8sResourcesFromManifest(
	kubectlOptions k8s.KubectlOptions,
	resourceKind string,
	resourceName string,
	resourceManifest string,
	shouldBeValidated bool,
) {
	By(fmt.Sprintf("Checking the existence of resource %s", resourceName))
	err := k8s.RunKubectlE(GinkgoT(), &kubectlOptions, "get", resourceKind, resourceName)

	if err == nil {
		replaceK8sResourcesFromManifest(kubectlOptions, resourceManifest, shouldBeValidated)
	} else {
		createK8sResourcesFromManifest(kubectlOptions, resourceManifest, shouldBeValidated)
	}
}

// currentKubernetesContext returns the currently set Kubernetes context based
// on the the environment variables and the KUBECONFIG file.
func currentEnvK8sContext() (kubeconfigPath string, kubecontextName string, err error) {
	kubeconfigPath, isExisting := os.LookupEnv("KUBECONFIG")
	if !isExisting {
		homePath, err := os.UserHomeDir()
		if err != nil {
			return "", "", errors.WrapIf(err, "retrieving user home directory failed")
		}

		kubeconfigPath = path.Join(homePath, ".kube", "config")
	}

	kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return "", "", errors.WrapIfWithDetails(err, "reading KUBECONFIG file failed", "path", kubeconfigPath)
	}

	structuredKubeconfig := make(map[string]interface{})
	err = yaml.Unmarshal(kubeconfigBytes, &structuredKubeconfig)
	if err != nil {
		return "", "", errors.WrapIfWithDetails(
			err,
			"parsing kubeconfig failed",
			"kubeconfig", string(kubeconfigBytes),
		)
	}

	kubecontext, isOk := structuredKubeconfig["current-context"].(string)
	if !isOk {
		return "", "", errors.WrapIfWithDetails(
			err,
			"kubeconfig current-context is not string",
			"current-context", structuredKubeconfig["current-context"],
		)
	}

	return kubeconfigPath, kubecontext, nil
}

// kubectlOptions instantiates a KubectlOptions from the specified Kubernetes
// context name, provided KUBECONFIG path and given namespace.
func kubectlOptions(kubecontextName, kubeconfigPath, namespace string) k8s.KubectlOptions {
	return *k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, namespace)
}

// listK8sCRDs lists the available CRDs from the specified kubectl context and
// namespace optionally filtering for the specified CRD names.
func listK8sCRDs(kubectlOptions k8s.KubectlOptions, crdNames ...string) []string {
	if len(crdNames) == 0 {
		By("Listing CRDs")
	} else {
		By(fmt.Sprintf("Listing CRDs filtered for CRD names %+v", crdNames))
	}

	args := append([]string{"get", "crd", "-o", "name"}, crdNames...)
	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		args...,
	)

	Expect(err).NotTo(HaveOccurred())

	return strings.Split(output, "\n")
}

// replaceK8sResourcesFromManifest replaces existing Kubernetes resources from
// the specified manifest to the provided kubectl context and namespace.
func replaceK8sResourcesFromManifest(kubectlOptions k8s.KubectlOptions, manifestPath string, shouldBeValidated bool) {
	By(fmt.Sprintf("Replacing k8s resources from manifest %s", manifestPath))
	k8s.RunKubectl(
		GinkgoT(),
		&kubectlOptions,
		"replace",
		fmt.Sprintf("--validate=%t", shouldBeValidated),
		"--filename", manifestPath,
	)
}

// requireExistingCRDs checks whether the specified CRDs are existing on
// the provided kubectl context.
func requireExistingCRDs(kubectlOptions k8s.KubectlOptions, crdNames ...string) {
	crds := listK8sCRDs(kubectlOptions, crdNames...)

	crdFullNames := make([]string, 0, len(crds))
	for _, crdName := range crdNames {
		crdFullNames = append(crdFullNames, "customresourcedefinition.apiextensions.k8s.io/"+crdName)
	}

	Expect(crds).To(ContainElements(crdFullNames))
}

// requireRunningPods checks whether the specified pod names are existing in the
// namespace and have a running status.
func requireRunningPods(kubectlOptions k8s.KubectlOptions, matchingLabelKey string, podNames ...string) {
	By(fmt.Sprintf("Verifying running pods for pod names %+v", podNames))
	pods := k8s.ListPods(GinkgoT(), &kubectlOptions, v1.ListOptions{})

	podNamesAsInterfaces := make([]interface{}, 0, len(podNames))
	for _, podName := range podNames {
		podNamesAsInterfaces = append(podNamesAsInterfaces, podName)
	}

	Expect(pods).To(HaveLen(len(podNames)))
	for _, pod := range pods {
		Expect(pod.GetLabels()[matchingLabelKey]).To(BeElementOf(podNamesAsInterfaces...))
		Expect(pod.Status.Phase).To(BeEquivalentTo("Running"))
	}
}
