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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

// HelmReleaseStatus describes the possible states of a Helm release.
type HelmReleaseStatus string

const (
	// HelmReleaseDeployed is the Helm release state where the deployment is
	// successfully applied to the cluster.
	HelmReleaseDeployed HelmReleaseStatus = "deployed"

	// HelmReleaseFailed is the Helm release state where the deployment
	// encountered an error and couldn't be applied successfully to the cluster.
	HelmReleaseFailed HelmReleaseStatus = "failed"
)

// HelmRelease describes a Helm
type HelmRelease struct {
	ReleaseName string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace" yaml:"namespace"`
	Revision    string            `json:"revision" yaml:"revision"`
	UpdatedTime string            `json:"updated" yaml:"updated"` // Note: not parsable implicitly.
	Status      HelmReleaseStatus `json:"status" yaml:"status"`
	Chart       string            `json:"chart" yaml:"chart"`
	AppVersion  string            `json:"app_version" yaml:"app_version"`
}

// installHelmChart deploys a Helm chart to the specified kubectl context and
// namespace using the specified infos, extra arguments can be any of the helm
// CLI install flag arguments, flag keys and values must be provided separately.
func installHelmChart(
	kubectlOptions *k8s.KubectlOptions,
	helmRepository string,
	helmChartName string,
	helmChartVersion string,
	helmReleaseName string,
	setValues map[string]string,
) {
	By(
		fmt.Sprintf(
			"Installing Helm chart %s from %s with version %s by name %s",
			helmChartName, helmRepository, helmChartVersion, helmReleaseName,
		),
	)

	fixedArguments := []string{
		"--repo", helmRepository,
		"--create-namespace",
		"--atomic",
		"--debug",
	}

	helm.Install(
		GinkgoT(),
		&helm.Options{
			SetValues:      setValues,
			KubectlOptions: kubectlOptions,
			Version:        helmChartVersion,
			ExtraArgs: map[string][]string{
				"install": fixedArguments,
			},
		},
		helmChartName,
		helmReleaseName,
	)
}

// installHelmChart checks whether the specified named Helm release exists in
// the provided kubectl context and namespace, logs it if it does and returns or
// alternatively deploys the Helm chart to the specified kubectl context and
// namespace using the specified infos, extra arguments can be any of the helm
// CLI install flag arguments, flag keys and values must be provided separately.
func installHelmChartIfDoesNotExist(
	kubectlOptions *k8s.KubectlOptions,
	helmRepository string,
	helmChartName string,
	helmChartVersion string,
	helmReleaseName string,
	setValues map[string]string,
) {
	By(fmt.Sprintf("Checking for existing Helm release named %s", helmReleaseName))
	helmRelease, isInstalled := lookUpInstalledHelmReleaseByName(kubectlOptions, helmReleaseName)

	if isInstalled {
		By(fmt.Sprintf(
			"Skipping the installation of %s, because the Helm release is already present: %+v",
			helmReleaseName, helmRelease,
		))

		return
	}

	installHelmChart(kubectlOptions, helmRepository, helmChartName, helmChartVersion, helmReleaseName, setValues)
}

// listHelmReleases returns a slice of Helm releases retrieved from the cluster
// using the specified kubectl context and namespace.
func listHelmReleases(kubectlOptions *k8s.KubectlOptions) []*HelmRelease {
	By("Listing Helm releases")
	output, err := helm.RunHelmCommandAndGetOutputE(
		GinkgoT(),
		&helm.Options{
			KubectlOptions: kubectlOptions,
		},
		"list",
		"--output", "yaml",
	)

	Expect(err).NotTo(HaveOccurred())

	var releases []*HelmRelease
	err = yaml.Unmarshal([]byte(output), &releases)

	Expect(err).NotTo(HaveOccurred())

	return releases
}

// lookUpInstalledHelmReleaseByName returns a Helm release and an indicator
// whether the Helm release is installed to the specified kubectl context
// and namespace by the provided Helm release name.
func lookUpInstalledHelmReleaseByName(kubectlOptions *k8s.KubectlOptions, helmReleaseName string) (*HelmRelease, bool) {
	releases := listHelmReleases(kubectlOptions)

	for _, release := range releases {
		if release.ReleaseName == helmReleaseName {
			Expect(release.Status).To(BeEquivalentTo(HelmReleaseDeployed))

			return release, true
		}
	}

	return nil, false
}
