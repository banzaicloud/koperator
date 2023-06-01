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

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twmb/franz-go/pkg/kgo"
)

func requireInternalProducerConsumer(kubectlOptions *k8s.KubectlOptions) {
	When("Internally produce and consume message to Kafka cluster", Ordered, func() {
		requireDeployingKcatPod(kubectlOptions, "kcat")
		requireDeployingKafkaTopic(kubectlOptions, "topic-icp")
		requireInternalProducingConsumingMessage(kubectlOptions, "kafka-headless:29092", "kcat", "topic-icp")
		//requireDeleteKafkaTopic()
	})
}

func requireDeployingKcatPod(kubectlOptions *k8s.KubectlOptions, podName string) {
	It("Deploying Kcat Pod", func() {
		applyK8sResourceFromTemplate(kubectlOptions,
			"templates/kcat.yaml.tmpl",
			map[string]interface{}{
				"Name":      "kcat",
				"Namespace": kubectlOptions.Namespace,
			},
		)
		waitK8sResourceCondition(kubectlOptions, "pods", "condition=Ready", "5s", "", "kcat")
	})

}

func requireDeployingKafkaTopic(kubectlOptions *k8s.KubectlOptions, topicName string) {
	It("Deploying KafkaTopic CR", func() {
		applyK8sResourceFromTemplate(kubectlOptions,
			"templates/topic.yaml.tmpl",
			map[string]interface{}{
				"Name":      topicName,
				"TopicName": topicName,
				"Namespace": kubectlOptions.Namespace,
			},
		)
		waitK8sResourceCondition(kubectlOptions, "kafkatopic", "jsonpath={.status.state}=created", "10s", "", topicName)
	})

}

func requireDeleteKafkaTopic(kubectlOptions *k8s.KubectlOptions, topicName string) {
	It("Deleting KafkaTopic CR", func() {
		deleteK8sResource(kubectlOptions, "", topicName, topicName)
	})
}

func requireInternalProducingConsumingMessage(kubectlOptions *k8s.KubectlOptions, internalAddress, kcatPodName, topicName string) {
	It(fmt.Sprintf("Producing and consuming messages (kafkaAddr: '%s' topicName: '%s", internalAddress, topicName), func() {
		By("Producing message")
		kubectlNamespace := kubectlOptions.Namespace
		kubectlOptions.Namespace = ""
		_, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
			kubectlOptions,
			"exec",
			"kcat",
			"-n",
			kubectlNamespace,
			"--",
			"/usr/bin/kcat",
			"-L",
			"-b",
			internalAddress,
			"-t",
			topicName,
			"-P",
			"/etc/passwd",
		)
		Expect(err).NotTo(HaveOccurred())

		By("Consuming message")
		consumedMessage, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
			kubectlOptions,
			"exec",
			"kcat",
			"-n",
			kubectlNamespace,
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
		Expect(consumedMessage).Should(ContainSubstring("root:x:0:0:root:/root:/bin/ash"))

		kubectlOptions.Namespace = kubectlNamespace
	})
}

func requireExternalProducerConsumer(kubectlOptions *k8s.KubectlOptions) {
	When("Internally produce and consume message to Kafka cluster", Ordered, func() {
		requireDeployingKafkaTopic(kubectlOptions, "topic-icp")
		requireExternalProducingConsumingMessage(kubectlOptions, "topic-icp", "a293a4e8347fe40408529348014d1887-1887576550.eu-north-1.elb.amazonaws.com:19090")
		requireDeleteKafkaTopic(kubectlOptions, "topic-icp")
	})
}

func requireExternalProducingConsumingMessage(kubectlOptions *k8s.KubectlOptions, topicName string, externalAddresses ...string) {
	It(fmt.Sprintf("Producing and consuming messages (kafkaAddr: '%s' topicName: '%s", externalAddresses, topicName), func() {
		By("Producing message")
		message := []byte("bar")
		// One client can both produce and consume!
		// Consuming can either be direct (no consumer group), or through a group. Below, we use a group.
		cl, err := kgo.NewClient(
			kgo.SeedBrokers(externalAddresses...),
			//kgo.ConsumerGroup("my-group-identifier"),
			kgo.ConsumeTopics(topicName),
		)
		Expect(err).NotTo(HaveOccurred())
		defer cl.Close()

		ctx := context.Background()
		record := &kgo.Record{Topic: topicName, Value: message}

		err = cl.ProduceSync(ctx, record).FirstErr()
		Expect(err).NotTo(HaveOccurred())

		By("Consuming message")
		fetches := cl.PollFetches(ctx)
		errs := fetches.Errors()
		Expect(errs).Should(HaveLen(0))

		// We can iterate through a record iterator...
		foundMessage := false

		iter := fetches.RecordIter()
		for !iter.Done() {
			consumedRecord := iter.Next()
			if string(consumedRecord.Value) == string(message) {
				foundMessage = true
				break
			}
		}

		By("Comparing produced and consumed message")
		Expect(foundMessage).Should(BeTrue())
	})
}
