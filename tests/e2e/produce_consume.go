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
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// requireDeployingKcatPod deploys kcat pod form a template and checks the pod readiness
func requireDeployingKcatPod(kubectlOptions k8s.KubectlOptions, podName string) {
	It("Deploying Kcat Pod", func() {
		err := applyK8sResourceFromTemplate(kubectlOptions,
			kcatPodTemplate,
			map[string]interface{}{
				"Name":      kcatPodName,
				"Namespace": kubectlOptions.Namespace,
			},
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = waitK8sResourceCondition(kubectlOptions, "pods", "condition=Ready", defaultPodReadinessWaitTime, "", kcatPodName)
		Expect(err).ShouldNot(HaveOccurred())
	})

}

// requireDeleteKcatPod deletes kcat pod.
func requireDeleteKcatPod(kubectlOptions k8s.KubectlOptions, podName string) {
	It("Deleting Kcat pod", func() {
		err := deleteK8sResource(kubectlOptions, defaultDeletionTimeout, "pods", "", podName)
		Expect(err).NotTo(HaveOccurred())
	})
}

// requireInternalProducingConsumingMessage produces and consumes messages internally through a kcat pod
// and makes comparisons between the produced and consumed messages.
// When internalAddress parameter is empty, it gets the internal address from the kafkaCluster CR status
func requireInternalProducingConsumingMessage(kubectlOptions k8s.KubectlOptions, internalAddress, kcatPodName, topicName string) {
	It(fmt.Sprintf("Producing and consuming messages to/from topicName: '%s", topicName), func() {
		if internalAddress == "" {
			By("Getting Kafka cluster internal addresses")
			internalListenerNames, err := getK8sResources(kubectlOptions,
				[]string{kafkaKind},
				"",
				kafkaClusterName,
				kubectlArgGoTemplateInternalListenersName,
			)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(internalListenerNames).ShouldNot(BeEmpty())

			internalListenerAddresses, err := getK8sResources(kubectlOptions,
				[]string{kafkaKind},
				"",
				kafkaClusterName,
				fmt.Sprintf(kubectlArgGoTemplateInternalListenerAddressesTemplate, internalListenerNames[0]),
			)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(internalListenerAddresses).ShouldNot(BeEmpty())

			internalAddress = internalListenerAddresses[0]
		}

		currentTime := time.Now()
		err := producingMessagesInternally(kubectlOptions, kcatPodName, internalAddress, topicName, currentTime.String())
		Expect(err).NotTo(HaveOccurred())

		consumedMessages, err := consumingMessagesInternally(kubectlOptions, kcatPodName, internalAddress, topicName)

		Expect(err).NotTo(HaveOccurred())
		Expect(consumedMessages).Should(ContainSubstring(currentTime.String()))
	})
}
