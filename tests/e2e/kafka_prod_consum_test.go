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
	"github.com/twmb/franz-go/pkg/kgo"
)

// requireInternalProducerConsumer deploys a kcat pod and a kafkaTopic into the K8s cluster
// and producing, consuming messages and make comparison between them.
// After deletes the kafkaTopic and kcat pod
func requireInternalProducerConsumer(kubectlOptions *k8s.KubectlOptions) {
	When("Internally produce and consume message to Kafka cluster", Ordered, func() {
		requireDeployingKcatPod(kubectlOptions, kcatPodName)
		requireDeployingKafkaTopic(kubectlOptions, testTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "kafka-headless:29092", kcatPodName, testTopicName)
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
		deleteK8sResource(kubectlOptions, "", "kafkatopic", topicName)
	})
}

// requireDeleteKcatPod deletes kcat pod.
func requireDeleteKcatPod(kubectlOptions *k8s.KubectlOptions, podName string) {
	It("Deleting Kcat pod", func() {
		deleteK8sResource(kubectlOptions, "", "pods", podName)
	})
}

// requireInternalProducingConsumingMessage produces and consuming messages internally through kcat pod
// and make comparison between the produced and consumed messages.
// When internalAddress parameter is empty, it gets the internal address from the kafkaCluster CR status
func requireInternalProducingConsumingMessage(kubectlOptions *k8s.KubectlOptions, internalAddress, kcatPodName, topicName string) {
	It(fmt.Sprintf("Producing and consuming messages into topicName: '%s", topicName), func() {
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
		kubectlNamespace := kubectlOptions.Namespace
		kubectlOptions.Namespace = ""

		currentTime := time.Now()
		_, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
			kubectlOptions,
			"exec",
			"kcat",
			"-n",
			kubectlNamespace,
			"--",
			"/bin/sh",
			"-c",
			fmt.Sprintf("echo %s | kcat -L -b %s -t %s -P", currentTime.String(), internalAddress, topicName),
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
		Expect(consumedMessage).Should(ContainSubstring(currentTime.String()))

		kubectlOptions.Namespace = kubectlNamespace
	})
}

// requireExternalProducerConsumer deploys a kafkaTopic into the K8s cluster
// and producing, consuming messages and make comparison between the produced and consumed messages.
func requireExternalProducerConsumer(kubectlOptions *k8s.KubectlOptions) {
	When("Internally produce and consume message to Kafka cluster", Ordered, func() {
		requireDeployingKafkaTopic(kubectlOptions, testTopicName)
		// TODO: requireAvailableExternalKafkaAddress()
		requireExternalProducingConsumingMessage(kubectlOptions, testTopicName, "a293a4e8347fe40408529348014d1887-1887576550.eu-north-1.elb.amazonaws.com:19090")
		requireDeleteKafkaTopic(kubectlOptions, testTopicName)
	})
}

// requireExternalProducingConsumingMessage producing, consuming messages and make comparison between them.
func requireExternalProducingConsumingMessage(kubectlOptions *k8s.KubectlOptions, topicName string, externalAddresses ...string) {
	It(fmt.Sprintf("Producing and consuming messages (kafkaAddr: '%s' topicName: '%s", externalAddresses, topicName), func() {
		By("Producing message")
		message := time.Now().String()
		// One client can both produce and consume!
		// Consuming can either be direct (no consumer group), or through a group.
		cl, err := kgo.NewClient(
			kgo.SeedBrokers(externalAddresses...),
			//kgo.ConsumerGroup("my-group-identifier"),
			kgo.ConsumeTopics(topicName),
		)
		Expect(err).NotTo(HaveOccurred())
		defer cl.Close()

		ctx := context.Background()
		ctxProducerTimeout, cancelCtxProducerTimeout := context.WithTimeout(ctx, externalProducerTimeout)
		defer cancelCtxProducerTimeout()

		record := &kgo.Record{Topic: topicName, Value: []byte(message)}

		// Exists at the first error occurrence
		err = cl.ProduceSync(ctxProducerTimeout, record).FirstErr()
		Expect(err).NotTo(HaveOccurred())

		By("Consuming message")

		ctxConsumerTimeout, cancelCtxConsumerTimeout := context.WithTimeout(ctx, externalProducerTimeout)
		defer cancelCtxConsumerTimeout()

		// Send fetch request for the Kafka cluster
		fetches := cl.PollFetches(ctxConsumerTimeout)
		errs := fetches.Errors()
		Expect(errs).Should(BeEmpty())

		foundMessage := false
		// We can iterate through a record iterator...
		iter := fetches.RecordIter()
		for !iter.Done() {
			consumedRecord := iter.Next()
			if string(consumedRecord.Value) == message {
				foundMessage = true
				break
			}
		}

		By("Comparing produced and consumed message")
		Expect(foundMessage).Should(BeTrue())
	})
}
