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

	"github.com/banzaicloud/koperator/api/v1beta1"
)

// requireCreatingKafkaCluster creates a KafkaCluster and
// checks the success of that operation.
func requireCreatingKafkaCluster(kubectlOptions k8s.KubectlOptions, manifestPath string) {
	It("Deploying a KafkaCluster", func() {
		By("Checking existing KafkaClusters")
		found := isExistingK8SResource(kubectlOptions, kafkaKind, kafkaClusterName)
		if found {
			By(fmt.Sprintf("KafkaCluster %s already exists\n", kafkaClusterName))
		} else {
			By("Deploying a KafkaCluster")
			applyK8sResourceManifest(kubectlOptions, manifestPath)
		}

		By("Verifying the KafkaCluster state")
		err := waitK8sResourceCondition(kubectlOptions, kafkaKind, fmt.Sprintf("jsonpath={.status.state}=%s", string(v1beta1.KafkaClusterRunning)), kafkaClusterCreateTimeout, "", kafkaClusterName)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the CruiseControl pod")
		Eventually(context.Background(), func() error {
			return waitK8sResourceCondition(kubectlOptions, "pod", "condition=Ready", cruiseControlPodReadinessTimeout, v1beta1.KafkaCRLabelKey+"="+kafkaClusterName+",app=cruisecontrol", "")
		}, kafkaClusterResourceReadinessTimeout, 3*time.Second).ShouldNot(HaveOccurred())

		By("Verifying all Kafka pods")
		err = waitK8sResourceCondition(kubectlOptions, "pod", "condition=Ready", defaultPodReadinessWaitTime, v1beta1.KafkaCRLabelKey+"="+kafkaClusterName, "")
		Expect(err).NotTo(HaveOccurred())
	})
}

// requireCreatingZookeeperCluster creates a ZookeeperCluster and
// checks the success of that operation.
func requireCreatingZookeeperCluster(kubectlOptions k8s.KubectlOptions) {
	It("Deploying a ZookeeperCluster", func() {
		By("Checking existing ZookeeperClusters")
		found := isExistingK8SResource(kubectlOptions, zookeeperKind, zookeeperClusterName)
		if found {
			By(fmt.Sprintf("ZookeeperCluster %s already exists\n", zookeeperClusterName))
		} else {
			By("Deploying the sample ZookeeperCluster")
			err := applyK8sResourceFromTemplate(kubectlOptions,
				zookeeperClusterTemplate,
				map[string]interface{}{
					"Name":      zookeeperClusterName,
					"Namespace": kubectlOptions.Namespace,
					"Replicas":  zookeeperClusterReplicaCount,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Verifying the ZookeeperCluster resource")
		err := waitK8sResourceCondition(kubectlOptions, zookeeperKind, "jsonpath={.status.readyReplicas}=1", zookeeperClusterCreateTimeout, "", zookeeperClusterName)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the ZookeeperCluster's pods")
		err = waitK8sResourceCondition(kubectlOptions, "pod", "condition=Ready", defaultPodReadinessWaitTime, "app="+zookeeperClusterName, "")
		Expect(err).NotTo(HaveOccurred())
	})
}
