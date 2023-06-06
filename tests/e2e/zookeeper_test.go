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

// requireInstallingZookeeperOperator deploys zookeeper-operator Helm chart and
// checks the success of that operation.
func requireInstallingZookeeperOperator(kubectlOptions k8s.KubectlOptions, certManagerVersion string) {
	When("Installing zookeeper-operator", Ordered, func() {
		requireInstallingZookeeperOperatorHelmChart(kubectlOptions, certManagerVersion)
	})
}

// requireInstallingZookeeperOperatorHelmChart checks the existence of the cert-manager
// Helm release and installs it if it's not present.
func requireInstallingZookeeperOperatorHelmChart(kubectlOptions k8s.KubectlOptions, zookeeperOperatorVersion string) {
	It("Installing zookeeper-operator Helm chart", func() {
		installHelmChart(
			kubectlOptions,
			"https://charts.pravega.io",
			"zookeeper-operator",
			zookeeperOperatorVersion,
			"zookeeper-operator",
			nil,
		)
	})
}
