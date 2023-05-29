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

func requireInternalProducerConsumer(kubectlOptions *k8s.KubectlOptions) {
	When("Internally produce and consume message to Kafka cluster", Ordered, func() {
		requireDeployingKcatPod(kubectlOptions, "kcat")
		requireDeployingKafkaTopic(kubectlOptions, "topic-icp")
		requireInternalProducingConsumingMessage(kubectlOptions, "kcat", "topic-icp")
	})
}

func requireDeployingKcatPod(kubectlOptions *k8s.KubectlOptions, podName string) {
	It("Deploying Kcat Pod", func() {
		applyK8sResourceFromTemplate(kubectlOptions,
			"templates/kcat.tpl",
			map[string]interface{}{
				"Name":      "kcat",
				"Namespace": kubectlOptions.Namespace,
			},
		)
		waitK8sResourceCondition(kubectlOptions, "pods", "condition=Ready", "5s", "kcat")
	})

}

func requireDeployingKafkaTopic(kubectlOptions *k8s.KubectlOptions, topicName string) {
	It("Deploying KafkaTopic CR", func() {
		applyK8sResourceFromTemplate(kubectlOptions,
			"templates/topic.tpl",
			map[string]interface{}{
				"Name":      topicName,
				"TopicName": topicName,
				"Namespace": kubectlOptions.Namespace,
			},
		)
		waitK8sResourceCondition(kubectlOptions, "kafkatopic", "jsonpath={.status.state}=created", "10s", topicName)
	})

}

func requireInternalProducingConsumingMessage(kubectlOptions *k8s.KubectlOptions, kcatPodName, topicName string) {
	It("Producing and consuming messages", func() {
		kubectlOptions.Namespace = ""
		_, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
			kubectlOptions,
			"exec",
			"kcat",
			"--",
			"/usr/bin/kcat",
			"-L",
			"-b",
			"kafka-headless:29092",
			"-t",
			topicName,
			"-P",
			"/etc/passwd",
		)
		Expect(err).NotTo(HaveOccurred())
		_, err = k8s.RunKubectlAndGetOutputE(GinkgoT(),
			kubectlOptions,
			"exec",
			"kcat",
			"--",
			"/usr/bin/kcat",
			"-L",
			"-b",
			"kafka-headless:29092",
			"-t",
			topicName,
			"-e",
			"-C",
		)
		Expect(err).NotTo(HaveOccurred())

	})
}
