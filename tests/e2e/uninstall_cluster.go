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

	koperator_v1beta1 "github.com/banzaicloud/koperator/api/v1beta1"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// requireDeleteKafkaCluster deletes KafkaCluster resource and
// checks the removal of the Kafka cluster related resources
func requireDeleteKafkaCluster(kubectlOptions k8s.KubectlOptions, name string) {
	It("Delete KafkaCluster custom resource", func() {
		err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, kafkaKind, name)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(context.Background(), func() []string {
			By("Verifying the Kafka cluster resource cleanup")

			// Check only those Koperator related resource types we have in K8s (istio usecase)
			k8sResourceKinds, err := listK8sResourceKinds(kubectlOptions, "")
			Expect(err).ShouldNot(HaveOccurred())

			koperatorAvailableResourceKinds := stringSlicesInstersect(koperatorRelatedResourceKinds(), k8sResourceKinds)
			koperatorAvailableResourceKinds = append(koperatorAvailableResourceKinds, basicK8sResourceKinds()...)

			resources, err := getK8sResources(kubectlOptions,
				koperatorAvailableResourceKinds,
				fmt.Sprintf("%s=%s", koperator_v1beta1.KafkaCRLabelKey, name),
				"",
				"--all-namespaces", kubectlArgGoTemplateKindNameNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			return resources
		}, kafkaClusterResourceCleanupTimeout, 1*time.Second).Should(BeEmpty())
	})
}

// requireDeleteZookeeperCluster deletes the ZookeeperCluster CR and verify the corresponding resources cleanup
func requireDeleteZookeeperCluster(kubectlOptions k8s.KubectlOptions, name string) {
	It("Delete ZookeeperCluster custom resource", func() {
		err := deleteK8sResourceNoErrNotFound(kubectlOptions, defaultDeletionTimeout, zookeeperKind, name)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(context.Background(), func() []string {
			By("Verifying the Zookeeper cluster resource cleanup")

			zookeeperK8sResources := basicK8sResourceKinds()
			zookeeperK8sResources = append(zookeeperK8sResources, dependencyCRDs.Zookeeper()...)

			resources, err := getK8sResources(kubectlOptions,
				zookeeperK8sResources,
				fmt.Sprintf("app=%s", name),
				"",
				"--all-namespaces", kubectlArgGoTemplateKindNameNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			return resources
		}, zookeeperClusterResourceCleanupTimeout, 1*time.Second).Should(BeEmpty())
	})
}
