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
	"os"
	"path"
	"strings"
	"text/template"

	"emperror.dev/errors"
	"github.com/Masterminds/sprig/v3"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	kubectlArgGoTemplateName              = `-o=go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'`
	kubectlArgGoTemplateKindNameNamespace = `-o=go-template='{{range .items}}{{.kind}}{{"/"}}{{.metadata.name}}{{if .metadata.namespace}}{{"."}}{{.metadata.namespace}}{{end}}{{"\n"}}{{end}}'`
)

func deleteK8sResourceOpts(
	kubectlOptions *k8s.KubectlOptions,
	globalResource,
	noErr bool,
	timeout string,
	kind string,
	selector string,
	name string,
	extraArgs ...string) {

	args := extraArgs
	args = append(args, "delete", kind)

	if timeout == "" {
		timeout = defaultDeletionTimeout
	}
	args = append(args, fmt.Sprintf("--timeout=%s", timeout))
	kubectlNamespace := kubectlOptions.Namespace

	if globalResource {
		kubectlOptions.Namespace = ""
	}

	logMsg := fmt.Sprintf("Deleting k8s resource: kind: '%s' ", kind)
	logMsg, args = _kubectlArgExtender(args, logMsg, selector, name, kubectlOptions.Namespace, extraArgs)
	By(logMsg)

	_, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		kubectlOptions,
		args...,
	)

	kubectlOptions.Namespace = kubectlNamespace

	if !noErr {
		Expect(err).NotTo(HaveOccurred())
	}
}

func deleteK8sResourceGlobalNoErr(kubectlOptions *k8s.KubectlOptions, timeout, kind string, name string, extraArgs ...string) {
	deleteK8sResourceOpts(kubectlOptions, true, true, timeout, kind, "", name, extraArgs...)
}

func deleteK8sResourceGlobal(kubectlOptions *k8s.KubectlOptions, timeout, kind string, name string, extraArgs ...string) {
	deleteK8sResourceOpts(kubectlOptions, true, false, timeout, kind, "", name, extraArgs...)
}

func deleteK8sResource(kubectlOptions *k8s.KubectlOptions, timeout, kind string, name string, extraArgs ...string) {
	deleteK8sResourceOpts(kubectlOptions, false, false, timeout, kind, "", name, extraArgs...)
}

func deleteK8sResourceNoErr(kubectlOptions *k8s.KubectlOptions, timeout, kind string, name string, extraArgs ...string) {
	deleteK8sResourceOpts(kubectlOptions, false, true, timeout, kind, "", name, extraArgs...)
}

// applyK8sResourceManifests applies the specified manifest to the provided
// kubectl context and namespace.
func applyK8sResourceManifest(kubectlOptions *k8s.KubectlOptions, manifestPath string) {
	By(fmt.Sprintf("Applying k8s manifest %s", manifestPath))
	k8s.KubectlApply(GinkgoT(), kubectlOptions, manifestPath)
}

// applyK8sResourceManifestFromString applies the specified manifest in string format to the provided
// kubectl context and namespace.
func applyK8sResourceManifestFromString(kubectlOptions *k8s.KubectlOptions, manifest string) {
	By(fmt.Sprintf("Applying k8s manifest\n%s", manifest))
	k8s.KubectlApplyFromString(GinkgoT(), kubectlOptions, manifest)
}

// applyK8sResourceFromTemplate generates manifest from the specified go-template based on values
// and applies the specified manifest to the provided kubectl context and namespace.
func applyK8sResourceFromTemplate(kubectlOptions *k8s.KubectlOptions, templateFile string, values map[string]interface{}) {
	By(fmt.Sprintf("Generating K8s manifest from template %s", templateFile))
	var manifest bytes.Buffer
	rawTemplate, err := os.ReadFile(templateFile)
	Expect(err).NotTo(HaveOccurred())
	t := template.Must(template.New("template").Funcs(sprig.TxtFuncMap()).Parse(string(rawTemplate)))
	err = t.Execute(&manifest, values)
	Expect(err).NotTo(HaveOccurred())
	applyK8sResourceManifestFromString(kubectlOptions, manifest.String())
}

// checkExistenceOfK8sResource queries a Resource by it's kind, namespace and name and
// returns the output of stderr
func checkExistenceOfK8sResource(
	kubectlOptions *k8s.KubectlOptions,
	resourceKind string,
	resourceName string,
) error {
	By(fmt.Sprintf("Checking the existence of resource %s", resourceName))
	return k8s.RunKubectlE(GinkgoT(), kubectlOptions, "get", resourceKind, resourceName)
}

// createK8sResourcesFromManifest creates Kubernetes resources from the
// specified manifest to the provided kubectl context and namespace.
func createK8sResourcesFromManifest(kubectlOptions *k8s.KubectlOptions, manifestPath string, shouldBeValidated bool) {
	By(fmt.Sprintf("Creating k8s resources from manifest %s", manifestPath))
	k8s.RunKubectl(
		GinkgoT(),
		kubectlOptions,
		"create",
		fmt.Sprintf("--validate=%t", shouldBeValidated),
		"--filename", manifestPath,
	)
}

// createOrReplaceK8sResourcesFromManifest creates non-existent Kubernetes
// resources or replaces existing ones from the specified manifest to the
// provided kubectl context and namespace.
func createOrReplaceK8sResourcesFromManifest(
	kubectlOptions *k8s.KubectlOptions,
	resourceKind string,
	resourceName string,
	resourceManifest string,
	shouldBeValidated bool,
) {
	By(fmt.Sprintf("Checking the existence of resource %s", resourceName))
	err := k8s.RunKubectlE(GinkgoT(), kubectlOptions, "get", resourceKind, resourceName)

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

// listK8sCRDs lists the available CRDs from the specified kubectl context and
// namespace optionally filtering for the specified CRD names.
func listK8sCRDs(kubectlOptions *k8s.KubectlOptions, crdNames ...string) []string {
	if len(crdNames) == 0 {
		By("Listing CRDs")
	} else {
		By(fmt.Sprintf("Listing CRDs filtered for CRD names %+v", crdNames))
	}

	args := append([]string{"get", "crd", "-o", "name"}, crdNames...)
	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		kubectlOptions,
		args...,
	)

	Expect(err).NotTo(HaveOccurred())

	return strings.Split(output, "\n")
}

func listK8sAllResourceType(kubectlOptions *k8s.KubectlOptions) []string {
	By("Listing available K8s resource types")

	args := []string{"api-resources", "--verbs=list", "-o", "name"}
	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		kubectlOptions,
		args...,
	)

	Expect(err).NotTo(HaveOccurred())

	return strings.Split(output, "\n")
}

// replaceK8sResourcesFromManifest replaces existing Kubernetes resources from
// the specified manifest to the provided kubectl context and namespace.
func replaceK8sResourcesFromManifest(kubectlOptions *k8s.KubectlOptions, manifestPath string, shouldBeValidated bool) {
	By(fmt.Sprintf("Replacing k8s resources from manifest %s", manifestPath))
	k8s.RunKubectl(
		GinkgoT(),
		kubectlOptions,
		"replace",
		fmt.Sprintf("--validate=%t", shouldBeValidated),
		"--filename", manifestPath,
	)
}

// requireExistingCRDs checks whether the specified CRDs are existing on
// the provided kubectl context.
func requireExistingCRDs(kubectlOptions *k8s.KubectlOptions, crdNames ...string) {
	crds := listK8sCRDs(kubectlOptions, crdNames...)

	crdFullNames := make([]string, 0, len(crds))
	for _, crdName := range crdNames {
		crdFullNames = append(crdFullNames, "customresourcedefinition.apiextensions.k8s.io/"+crdName)
	}

	Expect(crds).To(ContainElements(crdFullNames))
}

// requireRunningPods checks whether the specified pod names are existing in the
// namespace and have a running status.
func requireRunningPods(kubectlOptions *k8s.KubectlOptions, matchingLabelKey string, podNames ...string) {
	By(fmt.Sprintf("Verifying running pods for pod names %+v", podNames))
	pods := k8s.ListPods(GinkgoT(), kubectlOptions, v1.ListOptions{})

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

// getK8sResources gets the specified K8S resources from the specified kubectl context and
// namespace optionally. Extra arguments can be any of the kubectl get flag arguments.
// Returns a slice of the returned elements. Separator between elements must be newline.
func getK8sResources(kubectlOptions *k8s.KubectlOptions, resourceKind []string, selector string, names string, extraArgs ...string) []string {
	logMsg := fmt.Sprintf("Get K8S resources: '%s'", resourceKind)

	args := []string{"get", strings.Join(resourceKind, ",")}
	logMsg, args = _kubectlArgExtender(args, logMsg, selector, names, kubectlOptions.Namespace, extraArgs)

	By(logMsg)

	args = append(args, extraArgs...)

	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		kubectlOptions,
		args...,
	)

	Expect(err).NotTo(HaveOccurred())

	output = strings.Trim(output, "'")
	if output == "" {
		return nil
	}

	output = strings.TrimRight(output, "\n")
	outputSlice := strings.Split(output, "\n")
	var resources []string

	for i := range outputSlice {
		if !strings.Contains(outputSlice[i], "Warning:") {
			resources = append(resources, outputSlice[i])
		}
	}

	return resources
}

func waitK8sResourceCondition(kubectlOptions *k8s.KubectlOptions, resourceKind, waitFor, timeout string, selector string, names string, extraArgs ...string) {
	logMsg := fmt.Sprintf("Waiting K8s resource(s)' condition: '%s' to fulfil", waitFor)
	args := []string{
		"wait",
		resourceKind,
		fmt.Sprintf("--for=%s", waitFor),
		fmt.Sprintf("--timeout=%s", timeout),
	}

	logMsg, args = _kubectlArgExtender(args, logMsg, selector, names, kubectlOptions.Namespace, extraArgs)
	By(logMsg)

	_, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		kubectlOptions,
		args...,
	)

	Expect(err).NotTo(HaveOccurred())
}
