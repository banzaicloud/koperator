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

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/gruntwork-io/terratest/modules/k8s"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// createKafkaClusterIfDoesNotExist creates a KafkaCluster if
// there isn't a preexisting one
func createKafkaClusterIfDoesNotExist(kubectlOptions *k8s.KubectlOptions, koperatorVersion string, sampleFile string) {
	It("Deploying a KafkaCluster", func() {
		By("Checking existing KafkaClusters")
		err := checkExistenceOfK8sResource(kubectlOptions, kafkaKind, kafkaClusterName)

		if err == nil {
			By(fmt.Sprintf("KafkaCluster %s already exists\n", kafkaClusterName))
		} else {
			By("Deploying a KafkaCluster")
			requireApplyingKoperatorSampleResource(kubectlOptions, koperatorVersion, sampleFile)
		}
	})
}

// requireCreatingKafkaCluster creates a KafkaCluster and
// checks the success of that operation.
func requireCreatingKafkaCluster(kubectlOptions *k8s.KubectlOptions, koperatorVersion string, sampleFile string) {
	When("Creating a KafkaCluster", func() {
		createKafkaClusterIfDoesNotExist(kubectlOptions, koperatorVersion, sampleFile)
		requireKafkaClusterReady(kubectlOptions)
	})
}

func requireKafkaClusterReady(kubectlOptions *k8s.KubectlOptions) {
	It("Verifying KafkaCluster health", func() {
		By("Verifying the KafkaCluster state")
		waitK8sResourceCondition(kubectlOptions, kafkaKind, fmt.Sprintf("jsonpath={.status.state}=%s", string(v1beta1.KafkaClusterRunning)), kafkaClusterCreateTimeout, "", kafkaClusterName)
		By("Verifying the CruiseControl pod")
		Eventually(context.Background(), func() bool {
			resources := getK8sResources(kubectlOptions, []string{"pod"}, v1beta1.KafkaCRLabelKey+"="+kafkaClusterName+",app=cruisecontrol", "")
			if len(resources) > 1 {
				waitK8sResourceCondition(kubectlOptions, "pod", "condition=Ready", cruiseControlPodReadinessTimeout, v1beta1.KafkaCRLabelKey+"="+kafkaClusterName+",app=cruisecontrol", "")
				return true
			}
			return false
		}, kafkaClusterResourceReadinessTimeout, 3*time.Second).Should(BeTrue())
		By("Verifying all Kafka pods")
		waitK8sResourceCondition(kubectlOptions, "pod", "condition=Ready", defaultPodReadinessWaitTime, v1beta1.KafkaCRLabelKey+"="+kafkaClusterName, "")
	})
}
