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
	"strings"
	"text/template"
	"time"

	"emperror.dev/errors"
	"github.com/Masterminds/sprig"
	"github.com/cisco-open/k8s-objectmatcher/patch"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	// allowedCRDByteCount is the limitation of the number of bytes a CRD is
	// allowed to have when being applied by K8s API server/kubectl.
	allowedCRDByteCount = 262144 //nolint:unused // Note: this const is currently only used in helper functions which are not yet called on so this linter transitively fails for this const

	// crdNamePrefix is the prefix of the CRD names when listed through kubectl.
	crdNamePrefix = "customresourcedefinition.apiextensions.k8s.io/" //nolint:unused // Note: this const is currently only used in helper functions which are not yet called on so this linter transitively fails for this const
)

// applyK8sResourceManifests applies the specified manifest to the provided
// kubectl context and namespace.
func applyK8sResourceManifest(kubectlOptions k8s.KubectlOptions, manifestPath string) { //nolint:unused // Note: this might come in handy for manual K8s resource operations.
	By(fmt.Sprintf("Applying k8s manifest %s", manifestPath))
	k8s.KubectlApply(GinkgoT(), &kubectlOptions, manifestPath)
}

// isExistingK8SResource queries a Resource by it's kind, namespace and name and
// returns true if it's found, false otherwise
func isExistingK8SResource(
	kubectlOptions k8s.KubectlOptions,
	resourceKind string,
	resourceName string,
) bool {
	By(fmt.Sprintf("Checking the existence of resource %s in namespace %s (kind: %s)", resourceName, kubectlOptions.Namespace, resourceKind))
	err := k8s.RunKubectlE(GinkgoT(), &kubectlOptions, "get", resourceKind, resourceName)
	if err != nil {
		By(fmt.Sprintf("Received error when getting resource: %s", err))
		return false
	}
	return true
}

// createOrReplaceK8sResourcesFromManifest creates non-existent Kubernetes
// resources or replaces existing ones from the specified manifest to the
// provided kubectl context and namespace.
func createOrReplaceK8sResourcesFromManifest( //nolint:unused // Note: this might come in handy for manual K8s resource operations such as too long CRDs.
	kubectlOptions k8s.KubectlOptions,
	resourceKind string,
	resourceName string,
	resourceManifest string,
	shouldBeValidated bool,
) {
	By(fmt.Sprintf("Checking the existence of resource %s", resourceName))
	err := k8s.RunKubectlE(GinkgoT(), &kubectlOptions, "get", resourceKind, resourceName)

	if err == nil {
		By(fmt.Sprintf("Replacing k8s resources from manifest %s", resourceManifest))
		k8s.RunKubectl(
			GinkgoT(),
			&kubectlOptions,
			"replace",
			fmt.Sprintf("--validate=%t", shouldBeValidated),
			"--filename", resourceManifest,
		)
	} else {
		By(fmt.Sprintf("Creating k8s resources from manifest %s", resourceManifest))
		k8s.RunKubectl(
			GinkgoT(),
			&kubectlOptions,
			"create",
			fmt.Sprintf("--validate=%t", shouldBeValidated),
			"--filename", resourceManifest,
		)
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

// getK8sCRD queries and returns the CRD of the specified CRD name from the
// provided Kubernetes context.
func getK8sCRD(kubectlOptions k8s.KubectlOptions, crdName string) ([]byte, error) { //nolint:unused // Note: this might come in handy for manual CRD operations.
	if crdName == "" {
		return nil, errors.Errorf("invalid empty CRD name")
	}

	By(fmt.Sprintf("Getting CRD %s", crdName))
	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		[]string{"get", "crd", "--output", "json", crdName}...,
	)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "retrieving K8s CRD failed", "crdName", crdName)
	}

	return []byte(output), nil
}

// installK8sCRD installs a CRD from the specified path using the provided
// Kubernetes context. If the CRD is too long, create or replace will be used,
// otherwise apply is used.
//
// It is inefficient to transform path to representation and then back to temp
// file to use as path, but the separation from a CRD group manifest to
// individual manifests is required to not apply existing matching ones and
// after that we cannot use the original path so I don't see a better solution
// at the moment.
//
// We had to go to a lower level because none of apply, create, replace works
// properly with the KafkaCluster CRD due to the size (apply cannot handle the
// size, create is not idempotent, replace is only idempotent if the resources
// are existing already). Tried dropping the descriptions in the CRDs, but still
// too large.
func installK8sCRD(kubectlOptions k8s.KubectlOptions, crd []byte, shouldBeValidated bool) error { //nolint:unused // Note: this might come in handy for manual CRD installations.
	tempPath, err := createTempFileFromBytes(crd, "", "", 0)
	if err != nil {
		return errors.WrapIf(err, "creating temporary file for CRD failed")
	}

	switch {
	case len(crd) > allowedCRDByteCount: // Note: too long CRDs cannot be applied, only created or replaced.
		object, err := k8sObjectFromResourceManifest(crd)
		if err != nil {
			return errors.WrapIfWithDetails(err, "parsing CRD as K8s object failed", "crd", string(crd))
		}

		createOrReplaceK8sResourcesFromManifest(kubectlOptions, "crd", object.GetName(), tempPath, shouldBeValidated)
	default: // Note: regular CRD.
		applyK8sResourceManifest(kubectlOptions, tempPath)
	}

	return nil
}

// installK8sCRDs checks whether the CRDs specified with their manifest paths exist
// with the same content, installs them if they are missing and errors if it
// finds mismatching existing CRDs.
func installK8sCRDs(kubectlOptions k8s.KubectlOptions, crdManifestPaths ...string) error { //nolint:unused // Note: this might come in handy for manual CRD installations.
	crds, err := k8sResourcesFromManifestPaths(crdManifestPaths...)
	if err != nil {
		return errors.WrapIfWithDetails(
			err,
			"retrieving CRDs from manifest paths failed",
			"crdManifestPaths", crdManifestPaths,
		)
	}

	crdNames := make([]string, 0, len(crds))

	for _, crd := range crds {
		object, err := k8sObjectFromResourceManifest(crd)
		if err != nil {
			return errors.WrapIfWithDetails(
				err,
				"retrieving CRD object from resource manifest failed",
				"crd", string(crd),
			)
		}

		crdNames = append(crdNames, object.GetName())
	}

	clusterCRDNames, err := listK8sCRDs(kubectlOptions)
	if err != nil {
		return errors.WrapIf(err, "listing K8s CRDs failed")
	}

	for crdIndex, crd := range crds {
		crdName := crdNames[crdIndex]

		isFound := false
		for _, clusterCRDName := range clusterCRDNames {
			if clusterCRDName == crdNamePrefix+crdName {
				isFound = true
				break
			}
		}

		if isFound {
			clusterCRD, err := getK8sCRD(kubectlOptions, crdName)
			if err != nil {
				return errors.WrapIfWithDetails(err, "retrieving K8s CRD failed", "crdName", crdName)
			}

			By(fmt.Sprintf("Comparing existing and desired CRD %s", crdName))

			typedClusterCRD := new(apiextensionsv1.CustomResourceDefinition)
			err = yaml.Unmarshal(clusterCRD, typedClusterCRD)
			if err != nil {
				return errors.WrapIfWithDetails(err, "parsing K8s CRD failed", "clusterCRD", string(clusterCRD))
			}

			typedCRD := new(apiextensionsv1.CustomResourceDefinition)
			err = yaml.Unmarshal(crd, typedCRD)
			if err != nil {
				return errors.WrapIfWithDetails(err, "parsing CRD failed", "crd", string(crd))
			}

			patchResult, err := patch.DefaultPatchMaker.Calculate(typedClusterCRD, typedCRD)
			if err != nil {
				return errors.WrapIfWithDetails(
					err,
					"calculating actual and desired CRD diff failed",
					"clusterCRD", clusterCRD,
					"crd", crd,
				)
			}

			if !patchResult.IsEmpty() {
				return errors.NewWithDetails("actual and desired CRDs mismatch", "patch", patchResult.String())
			}
		} else {
			By(fmt.Sprintf("Installing CRD %s", crdName))
			err := installK8sCRD(kubectlOptions, crd, false)
			if err != nil {
				return errors.WrapIfWithDetails(err, "installing CRD failed", "crd", crd)
			}
		}
	}

	return nil
}

// k8sObjectFromResourceManifest returns the K8s object meta from the
// specified resource manifest.
func k8sObjectFromResourceManifest(resourceManifest []byte) (*unstructured.Unstructured, error) { //nolint:unused // Note: this might come in handy for manual K8s resource operations.
	unstructured := new(unstructured.Unstructured)
	err := yaml.Unmarshal(resourceManifest, &unstructured)
	if err != nil {
		return nil, errors.WrapIfWithDetails(
			err,
			"parsing K8s object failed",
			"resourceManifest", string(resourceManifest),
		)
	}

	return unstructured, nil
}

// k8sResourcesFromManifest splits the specified YAML manifest to separate resource
// manifests based on the --- YAML node delimiter and also trims the results for
// the delimiter and the leading or trailing whitespaces.
func k8sResourcesFromManifest(manifest []byte) [][]byte { //nolint:unused // Note: this might come in handy for manual K8s resource operations.
	resources := bytes.Split(manifest, []byte("\n---\n"))
	for resourceIndex, resource := range resources {
		resources[resourceIndex] = bytes.Trim(resource, "\n-")
	}

	return resources
}

// k8sResourcesFromManifestPaths returns the YAML resource manifests as raw data
// found in the specified manifest paths.
func k8sResourcesFromManifestPaths(manifestPaths ...string) ([][]byte, error) { //nolint:unused // Note: this might come in handy for manual K8s resource operations.
	resources := make([][]byte, 0, len(manifestPaths))
	for _, manifestPath := range manifestPaths {
		var manifest []byte
		var err error

		switch {
		case strings.HasPrefix(manifestPath, "https://"): // Note: remote URL.
			httpClient := new(http.Client)
			httpClient.Timeout = 5 * time.Second

			response, err := httpClient.Get(manifestPath)
			if err != nil {
				return nil, errors.WrapIfWithDetails(
					err,
					"retrieving remote resource from manifest path failed",
					"manifestPath", manifestPath,
				)
			}

			manifest, err = io.ReadAll(response.Body)
			if err != nil {
				return nil, errors.WrapIfWithDetails(
					err,
					"reading remote resource manifest response failed",
					"manifestPath", manifestPath,
				)
			}

			if response != nil {
				err := response.Body.Close()
				if err != nil {
					return nil, errors.WrapIfWithDetails(
						err,
						"closing remote manifest query response body failed",
						"manifestPath", manifestPath,
					)
				}
			}
		default: // Note: local file.
			manifest, err = os.ReadFile(manifestPath)
			if err != nil {
				return nil, errors.WrapIfWithDetails(
					err,
					"reading local resource manifest file failed",
					"manifestPath", manifestPath,
				)
			}
		}

		resources = append(resources, k8sResourcesFromManifest(manifest)...)
	}

	return resources, nil
}

// kubectlOptions instantiates a KubectlOptions from the specified Kubernetes
// context name, provided KUBECONFIG path and given namespace.
func kubectlOptions(kubecontextName, kubeconfigPath, namespace string) k8s.KubectlOptions {
	return *k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, namespace)
}

// kubectlOptionsForCurrentContext returns a kubectlOptions object for the
// current Kubernetes context or alternatively an error.
func kubectlOptionsForCurrentContext() (k8s.KubectlOptions, error) {
	kubeconfigPath, kubecontextName, err := currentEnvK8sContext()
	if err != nil {
		return k8s.KubectlOptions{}, errors.WrapIf(err, "retrieving current environment Kubernetes context failed")
	}

	return kubectlOptions(kubecontextName, kubeconfigPath, ""), nil
}

// listK8sCRDs lists the available CRDs from the specified kubectl context and
// namespace optionally filtering for the specified CRD names.
func listK8sCRDs(kubectlOptions k8s.KubectlOptions, crdNames ...string) ([]string, error) { //nolint:unused // Note: this might come in handy for manual CRD operations.
	if len(crdNames) == 0 {
		By("Listing CRDs")
	} else {
		By(fmt.Sprintf("Listing CRDs filtered for CRD names %+v", crdNames))
	}

	args := append([]string{"get", "crd", "--output", "name"}, crdNames...)
	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		args...,
	)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "listing K8s CRDs failed failed", "crdNames", crdNames)
	}

	return strings.Split(output, "\n"), nil
}

// deleteK8sResourceOpts deletes K8s resources based on the kind and name or kind and selector.
// It returns error in case when the resource is not found.
// timeout parameter specifies the timeout for the deletion.
// extraArgs can be any of the kubectl arguments.
func deleteK8sResource(
	kubectlOptions k8s.KubectlOptions,
	timeout time.Duration,
	kind string,
	selector string, //nolint:unparam // Note: library function with selector argument currently always receiving zero value ("").
	name string,
	extraArgs ...string,
) error {
	args := []string{"delete", kind}

	args = append(args, fmt.Sprintf("--timeout=%s", timeout))

	logMsg := fmt.Sprintf("Deleting k8s resource: kind: '%s' ", kind)
	logMsg, args = kubectlArgExtender(args, logMsg, selector, name, kubectlOptions.Namespace, extraArgs)
	By(logMsg)

	_, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		args...,
	)

	return err
}

// deleteK8sResourceNoErrNotFound deletes K8s resources based on the kind and name or kind and selector.
// Deletion is passed in case the resource is not found.
// timeout parameter specifies the timeout for the deletion.
// extraArgs can be any of the kubectl arguments.
func deleteK8sResourceNoErrNotFound(kubectlOptions k8s.KubectlOptions, timeout time.Duration, kind string, name string, extraArgs ...string) error { //nolint:unparam // Note: library function with timeout argument currently always using the same const.
	err := deleteK8sResource(kubectlOptions, timeout, kind, "", name, extraArgs...)
	if isKubectlNotFoundError(err) {
		By(fmt.Sprintf("K8s resource %s not found", name))
		return nil
	}
	return err
}

// applyK8sResourceManifestFromString applies the specified manifest in string format to the provided
// kubectl context and namespace.
func applyK8sResourceManifestFromString(kubectlOptions k8s.KubectlOptions, manifest string) error {
	By(fmt.Sprintf("Applying k8s manifest\n%s", manifest))
	return k8s.KubectlApplyFromStringE(GinkgoT(), &kubectlOptions, manifest)
}

// applyK8sResourceFromTemplate generates manifest from the specified go-template based on values
// and applies the specified manifest to the provided kubectl context and namespace.
func applyK8sResourceFromTemplate(kubectlOptions k8s.KubectlOptions, templateFile string, values map[string]interface{}) error {
	By(fmt.Sprintf("Generating K8s manifest from template %s", templateFile))
	var manifest bytes.Buffer
	rawTemplate, err := os.ReadFile(templateFile)
	if err != nil {
		return err
	}
	t := template.Must(template.New("template").Funcs(sprig.TxtFuncMap()).Parse(string(rawTemplate)))
	err = t.Execute(&manifest, values)
	if err != nil {
		return err
	}
	return applyK8sResourceManifestFromString(kubectlOptions, manifest.String())
}

// listK8sResourceKinds lists all of the available resource kinds on the K8s cluster
// with the apiGroupSelector parameter the result can be narrowed by the resource group.
// extraArgs can be any kubectl api-resources parameter.
func listK8sResourceKinds(kubectlOptions k8s.KubectlOptions, apiGroupSelector string, extraArgs ...string) ([]string, error) {
	args := []string{"api-resources", "--verbs", "list", "--output", "name", "--sort-by", "name"}

	if apiGroupSelector != "" {
		args = append(args, "--api-group", apiGroupSelector)
	}

	args = append(args, extraArgs...)

	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		args...,
	)

	if err != nil {
		return nil, err
	}

	output = strings.Trim(output, "'")
	// Empty output
	if output == "" {
		return nil, nil
	}

	return kubectlRemoveWarnings(strings.Split(output, "\n")), nil
}

// getK8sResources gets the specified K8S resources from the specified kubectl context and
// namespace optionally. Extra arguments can be any of the kubectl get flag arguments.
// Returns a slice of the returned elements. Separator between elements must be newline.
func getK8sResources(kubectlOptions k8s.KubectlOptions, resourceKind []string, selector string, names string, extraArgs ...string) ([]string, error) {
	logMsg := fmt.Sprintf("Get K8S resources: '%s'", resourceKind)

	args := []string{"get", strings.Join(resourceKind, ",")}
	logMsg, args = kubectlArgExtender(args, logMsg, selector, names, kubectlOptions.Namespace, extraArgs)
	By(logMsg)

	output, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		args...,
	)

	if err != nil {
		return nil, err
	}

	output = strings.Trim(output, "'")
	// Empty output
	if output == "" {
		return nil, nil
	}

	output = strings.TrimRight(output, "\n")
	outputSlice := strings.Split(output, "\n")

	// Remove warning message pollution from the output

	return kubectlRemoveWarnings(outputSlice), nil
}

// waitK8sResourceCondition waits until the condition is met or the timeout is elapsed for the selected K8s resource(s)
// extraArgs can be any of the kubectl arguments
func waitK8sResourceCondition(kubectlOptions k8s.KubectlOptions, resourceKind, waitFor string, timeout time.Duration, selector string, names string, extraArgs ...string) error { //nolint:unparam // Note: library function with variadic argument currently always nil.
	logMsg := fmt.Sprintf("Waiting K8s resource(s)' condition: '%s' to fulfil", waitFor)

	args := []string{
		"wait",
		resourceKind,
		fmt.Sprintf("--for=%s", waitFor),
		fmt.Sprintf("--timeout=%s", timeout),
	}

	logMsg, args = kubectlArgExtender(args, logMsg, selector, names, kubectlOptions.Namespace, extraArgs)
	By(logMsg)

	_, err := k8s.RunKubectlAndGetOutputE(
		GinkgoT(),
		&kubectlOptions,
		args...,
	)

	return err
}

// kubectlArgExtender extends the kubectl arguments and log message based on the parameters
func kubectlArgExtender(args []string, logMsg, selector, names, namespace string, extraArgs []string) (string, []string) {
	if selector != "" {
		logMsg = fmt.Sprintf("%s selector: '%s'", logMsg, selector)
		args = append(args, fmt.Sprintf("--selector=%s", selector))
	} else if names != "" {
		logMsg = fmt.Sprintf("%s name(s): '%s'", logMsg, names)
		args = append(args, names)
	}
	if namespace != "" {
		logMsg = fmt.Sprintf("%s namespace: '%s'", logMsg, namespace)
	}
	if len(extraArgs) != 0 {
		logMsg = fmt.Sprintf("%s extraArgs: '%s'", logMsg, extraArgs)
		args = append(args, extraArgs...)
	}
	return logMsg, args
}

// kubectlRemoveWarning removes those elements from the outputSlice parameter which contains kubectl warning message.
func kubectlRemoveWarnings(outputSlice []string) []string {
	// Remove warning message pollution from the output
	result := make([]string, 0, len(outputSlice))
	for i := range outputSlice {
		if !strings.Contains(outputSlice[i], "Warning:") {
			result = append(result, outputSlice[i])
		}
	}
	return result
}

func isKubectlNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), kubectlNotFoundErrorMsg)
}
