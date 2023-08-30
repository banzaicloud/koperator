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
	. "github.com/onsi/gomega"
)

func testInstallZookeeperCluster() bool {
	return When("Installing Zookeeper cluster", func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		It("Acquiring K8s config and context", func() {
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		kubectlOptions.Namespace = zookeeperOperatorHelmDescriptor.Namespace
		requireCreatingZookeeperCluster(kubectlOptions)
	})
}

func testInstallKafkaCluster(kafkaClusterManifestPath string) bool { //nolint:unparam // Note: respecting Ginkgo testing interface by returning bool.
	return When("Installing Kafka cluster", func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		It("Acquiring K8s config and context", func() {
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		kubectlOptions.Namespace = koperatorLocalHelmDescriptor.Namespace
		requireCreatingKafkaCluster(kubectlOptions, kafkaClusterManifestPath)
	})
}
