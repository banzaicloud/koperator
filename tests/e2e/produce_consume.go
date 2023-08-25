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
	"crypto/tls"
	"fmt"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twmb/franz-go/pkg/kgo"
)

// requireDeployingKcatPod deploys kcat pod form a template and checks the pod readiness
func requireDeployingKcatPod(kubectlOptions k8s.KubectlOptions, podName string, tlsSecretName string) {
	It("Deploying Kcat Pod", func() {
		templateParameters := map[string]interface{}{
			"Name":      podName,
			"Namespace": kubectlOptions.Namespace,
		}
		if tlsSecretName != "" {
			templateParameters["TLSSecretName"] = tlsSecretName
		}

		err := applyK8sResourceFromTemplate(kubectlOptions,
			kcatPodTemplate,
			templateParameters,
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = waitK8sResourceCondition(kubectlOptions, "pods",
			"condition=Ready", defaultPodReadinessWaitTime, "", podName)

		Expect(err).ShouldNot(HaveOccurred())
	})
}

// requireDeleteKcatPod deletes kcat pod.
func requireDeleteKcatPod(kubectlOptions k8s.KubectlOptions, podName string) {
	It("Deleting Kcat pod", func() {
		err := deleteK8sResource(kubectlOptions, kcatDeleetionTimeout, "pods", "", podName)
		Expect(err).NotTo(HaveOccurred())
	})
}

// requireInternalProducingConsumingMessage produces and consumes messages internally through a kcat pod
// and makes comparisons between the produced and consumed messages.
// When internalAddress parameter is empty, it gets the internal address from the kafkaCluster CR status.
// When tlsSecretName is set
func requireInternalProducingConsumingMessage(kubectlOptions k8s.KubectlOptions, internalAddress, kcatPodName, topicName string, tlsSecretName string) {
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

		tlsMode := false
		if tlsSecretName != "" {
			tlsMode = true
		}

		currentTime := time.Now()
		err := producingMessagesInternally(kubectlOptions, kcatPodName, internalAddress, topicName, currentTime.String(), tlsMode)
		Expect(err).NotTo(HaveOccurred())

		consumedMessages, err := consumingMessagesInternally(kubectlOptions, kcatPodName, internalAddress, topicName, tlsMode)

		Expect(err).NotTo(HaveOccurred())
		Expect(consumedMessages).Should(ContainSubstring(currentTime.String()))
	})
}

// requireExternalProducingConsumingMessage gets the Kafka cluster external addresses from the kafkaCluster CR status
// when externalAddresses is not specified. It also produces and consumes messages and makes a comparison between them.
func requireExternalProducingConsumingMessage(kubectlOptions k8s.KubectlOptions, topicName, tlsSecretName string, externalAddresses ...string) { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	It("Producing and consuming messages", func() {
		if len(externalAddresses) == 0 {
			var err error
			externalAddresses, err = getExternalListenerAddresses(kubectlOptions, "", kafkaClusterName)
			Expect(err).ShouldNot(HaveOccurred())
		}

		var tlsConfig *tls.Config
		var clientOptions []kgo.Opt
		if tlsSecretName != "" {
			var err error
			tlsConfig, err = getTLSConfigFromSecret(kubectlOptions, tlsSecretName)
			Expect(err).ShouldNot(HaveOccurred())
			clientOptions = append(clientOptions, kgo.DialTLSConfig(tlsConfig))
		}

		message := time.Now().String()

		err := producingMessagesExternally(externalAddresses, topicName, []string{message}, clientOptions...)
		Expect(err).ShouldNot(HaveOccurred())

		consumedMessages, err := consumingMessagesExternally(externalAddresses, topicName, clientOptions...)
		Expect(err).ShouldNot(HaveOccurred())

		By(fmt.Sprintf("Comparing produced: '%s' and consumed message: '%s'", message, consumedMessages))
		found := false
		for i := range consumedMessages {
			if consumedMessages[i] == message {
				found = true
				break
			}
		}
		Expect(found).Should(BeTrue())
	})
}

// getExternalListenerNames gets the names of the KafkaCluster CR's external listeners.
func getExternalListenerNames(kubectlOptions k8s.KubectlOptions, kafkaClusterName string) ([]string, error) { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	By("Getting external listener names from KafkaCluster status")
	externalListenerNames, err := getK8sResources(kubectlOptions,
		[]string{kafkaKind},
		"",
		kafkaClusterName,
		kubectlArgGoTemplateExternalListenersName,
	)
	if err != nil {
		return nil, fmt.Errorf("getting external listeners name: %w", err)
	}
	return externalListenerNames, nil
}

// getExternalListenerAddresses gets the Kafka cluster external addresses from the kafkaCluster CR.
// When externalListenerName is not specified it uses the first externalListener name in the CR to get addresses.
func getExternalListenerAddresses(kubectlOptions k8s.KubectlOptions, externalListenerName, kafkaClusterName string) ([]string, error) { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	By(fmt.Sprintf("Getting Kafka cluster '%s' external listener addresses", kafkaClusterName))
	if externalListenerName == "" {
		externalListenerNames, err := getExternalListenerNames(kubectlOptions, kafkaClusterName)
		if err != nil {
			return nil, err
		}
		Expect(getExternalListenerNames).ShouldNot(BeEmpty())
		externalListenerName = externalListenerNames[0]
	}
	By(fmt.Sprintf("Using external listener name: '%s'", externalListenerName))
	externalListenerAddresses, err := getK8sResources(kubectlOptions,
		[]string{kafkaKind},
		"",
		kafkaClusterName,
		fmt.Sprintf(kubectlArgGoTemplateExternalListenerAddressesTemplate, externalListenerName),
	)
	if err != nil {
		return nil, fmt.Errorf("getting external listener addresses: %w", err)
	}
	if len(externalListenerAddresses) == 0 {
		return nil, fmt.Errorf("external listener address %w", ErrorNotFound)
	}

	return externalListenerAddresses, nil
}

// requireAvailableExternalKafkaAddress checks that is there any available external address for the Kafka cluster.
func requireAvailableExternalKafkaAddress(kubectlOptions k8s.KubectlOptions, externalListenerName, kafkaClusterName string) { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	It(fmt.Sprintf("Checks that the KafkaCluster '%s' has external address", kafkaClusterName), func() {
		_, err := getExternalListenerAddresses(kubectlOptions, externalListenerName, kafkaClusterName)
		Expect(err).ShouldNot(HaveOccurred())
	})
}
