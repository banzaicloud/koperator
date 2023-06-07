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
	"context"
	"fmt"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// createZookeeperClusterIfDoesNotExist creates a zookeeper cluster if
// there isn't a preexisting one
func createZookeeperClusterIfDoesNotExist(kubectlOptions *k8s.KubectlOptions, zkClusterReplicaCount int) {
	It("Deploying a Zookeeper cluster", func() {
		By("Checking existing ZookeeperClusters")
		err := checkExistenceOfK8sResource(kubectlOptions, zookeeperKind, zookeeperClusterName)

		if err == nil {
			By(fmt.Sprintf("Zookeeper cluster %s already exists\n", zookeeperClusterName))
		} else {
			By("Deploying the sample ZookeeperCluster")
			applyK8sResourceFromTemplate(kubectlOptions,
				zookeeperClusterTemplate,
				map[string]interface{}{
					"Name":      zookeeperClusterName,
					"Namespace": kubectlOptions.Namespace,
					"Replicas":  zkClusterReplicaCount,
				},
			)
		}
	})
}

// requireCreatingZookeeperCluster creates a zookeeper cluster and
// checks the success of that operation.
func requireCreatingZookeeperCluster(kubectlOptions *k8s.KubectlOptions, zkClusterReplicaCount int) {
	When("Creating a Zookeeper cluster", func() {
		createZookeeperClusterIfDoesNotExist(kubectlOptions, zkClusterReplicaCount)
		requireZookeeperClusterReady(kubectlOptions)
	})
}

func requireZookeeperClusterReady(kubectlOptions *k8s.KubectlOptions) {
	It("Verifying Zookeeper cluster health", func() {
		By("Verifying the Zookeeper cluster resource")
		waitK8sResourceCondition(kubectlOptions, zookeeperKind, "jsonpath={.status.readyReplicas}=1", zookeeperClusterCreateTimeout, "", zookeeperClusterName)
		By("Verifying the Zookeeper cluster's pods")
		waitK8sResourceCondition(kubectlOptions, "pod", "condition=Ready", defaultPodReadinessWaitTime, "app="+zookeeperClusterName, "")
	})
}

// requireUninstallZookeeperCluster uninstall the Zookeeper cluster
func requireUninstallZookeeperCluster(kubectlOptions *k8s.KubectlOptions, name string) {
	When("Uninstalling Zookeeper cluster", func() {
		requireDeleteZookeeperCluster(kubectlOptions, name)

	})
}

// requireDeleteZookeeperCluster deletes the ZookeeperCluster CR and verify the corresponding resources cleanup
func requireDeleteZookeeperCluster(kubectlOptions *k8s.KubectlOptions, name string) {
	It("Delete ZookeeperCluster custom resource", func() {
		deleteK8sResourceGlobalNoErrNotFound(kubectlOptions, defaultDeletionTimeout, zookeeperKind, name)
		Eventually(context.Background(), func() []string {
			By("Verifying the Zookeeper cluster resource cleanup")

			zookeeperK8sResources := basicK8sCRDs()
			zookeeperK8sResources = append(zookeeperK8sResources, zookeeperCRDs()...)

			return getK8sResources(kubectlOptions,
				zookeeperK8sResources,
				fmt.Sprintf("app=%s", name),
				"",
				"--all-namespaces", kubectlArgGoTemplateKindNameNamespace)
		}, zookeeperClusterResourceCleanupTimeout, 3*time.Millisecond).Should(BeEmpty())
	})
}
