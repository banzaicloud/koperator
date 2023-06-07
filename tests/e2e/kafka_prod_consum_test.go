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

// requireInternalProducerConsumer deploys a kcat pod and a kafkaTopic into the K8s cluster
// and produces, consumes messages and makes comparisons between them.
// After deletes the kafkaTopic and kcat pod.
func requireInternalProducerConsumer(kubectlOptions *k8s.KubectlOptions) {
	When("Internally produce and consume message to Kafka cluster", func() {
		requireDeployingKcatPod(kubectlOptions, kcatPodName)
		requireDeployingKafkaTopic(kubectlOptions, testTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatPodName, testTopicName)
		requireDeleteKafkaTopic(kubectlOptions, testTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatPodName)
	})
}

// requireDeployingKcatPod deploys kcat pod form a template and checks the pod readiness
func requireDeployingKcatPod(kubectlOptions *k8s.KubectlOptions, podName string) {
	It("Deploying Kcat Pod", func() {
		applyK8sResourceFromTemplate(kubectlOptions,
			kcatPodTemplate,
			map[string]interface{}{
				"Name":      kcatPodName,
				"Namespace": kubectlOptions.Namespace,
			},
		)
		waitK8sResourceCondition(kubectlOptions, "pods", "condition=Ready", defaultPodReadinessWaitTime, "", kcatPodName)
	})

}

// requireDeployingKafkaTopic deploys a kafkaTopic resource from a template
func requireDeployingKafkaTopic(kubectlOptions *k8s.KubectlOptions, topicName string) {
	It("Deploying KafkaTopic CR", func() {
		applyK8sResourceFromTemplate(kubectlOptions,
			kafkaTopicTemplate,
			map[string]interface{}{
				"Name":      topicName,
				"TopicName": topicName,
				"Namespace": kubectlOptions.Namespace,
			},
		)
		waitK8sResourceCondition(kubectlOptions, "kafkatopic", "jsonpath={.status.state}=created", defaultTopicCreationWaitTime, "", topicName)
	})

}

// requireDeleteKafkaTopic deletes kafkaTopic resource.
func requireDeleteKafkaTopic(kubectlOptions *k8s.KubectlOptions, topicName string) {
	It("Deleting KafkaTopic CR", func() {
		deleteK8sResource(kubectlOptions, defaultDeletionTimeout, "kafkatopic", topicName)
	})
}

// requireDeleteKcatPod deletes kcat pod.
func requireDeleteKcatPod(kubectlOptions *k8s.KubectlOptions, podName string) {
	It("Deleting Kcat pod", func() {
		deleteK8sResource(kubectlOptions, defaultDeletionTimeout, "pods", podName)
	})
}

// requireInternalProducingConsumingMessage produces and consumes messages internally through a kcat pod
// and makes comparisons between the produced and consumed messages.
// When internalAddress parameter is empty, it gets the internal address from the kafkaCluster CR status
func requireInternalProducingConsumingMessage(kubectlOptions *k8s.KubectlOptions, internalAddress, kcatPodName, topicName string) {
	It(fmt.Sprintf("Producing and consuming messages to/from topicName: '%s", topicName), func() {
		if internalAddress == "" {
			By("Getting Kafka cluster internal addresses")
			internalListenerNames := getK8sResources(kubectlOptions,
				[]string{"kafkacluster"},
				"",
				kafkaClusterName,
				kubectlArgGoTemplateInternalListenersName,
			)
			Expect(internalListenerNames).ShouldNot(BeEmpty())

			internalListenerAddresses := getK8sResources(kubectlOptions,
				[]string{"kafkacluster"},
				"",
				kafkaClusterName,
				fmt.Sprintf(kubectlArgGoTemplateInternalListenerAddressesTemplate, internalListenerNames[0]),
			)
			Expect(internalListenerAddresses).ShouldNot(BeEmpty())

			internalAddress = internalListenerAddresses[0]
		}

		By("Producing message")
		// When kubectl exec command is used the namespace flag cannot be at the beginning
		// thus we need to specify by hand
		currentTime := time.Now()
		_, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
			k8s.NewKubectlOptions(kubectlOptions.ContextName, kubectlOptions.ConfigPath, ""),
			"exec",
			"kcat",
			"-n",
			kubectlOptions.Namespace,
			"--",
			"/bin/sh",
			"-c",
			fmt.Sprintf("echo %s | kcat -L -b %s -t %s -P", currentTime.String(), internalAddress, topicName),
		)
		Expect(err).NotTo(HaveOccurred())

		By("Consuming message")
		consumedMessage, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
			k8s.NewKubectlOptions(kubectlOptions.ContextName, kubectlOptions.ConfigPath, ""),
			"exec",
			"kcat",
			"-n",
			kubectlOptions.Namespace,
			"--",
			"/usr/bin/kcat",
			"-L",
			"-b",
			internalAddress,
			"-t",
			topicName,
			"-e",
			"-C",
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(consumedMessage).Should(ContainSubstring(currentTime.String()))
	})
}
