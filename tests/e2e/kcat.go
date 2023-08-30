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

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
)

// consumingMessagesInternally consuming messages based on parameters from Kafka cluster.
// It returns messages in string slice.
func consumingMessagesInternally(kubectlOptions k8s.KubectlOptions, kcatPodName string, internalKafkaAddress string, topicName string, tlsMode bool) (string, error) {
	By(fmt.Sprintf("Consuming messages from internalKafkaAddress: '%s' topicName: '%s'", internalKafkaAddress, topicName))

	kcatTLSParameters := ""
	if tlsMode {
		kcatTLSParameters += "-X security.protocol=SSL -X ssl.key.location=/ssl/certs/tls.key -X ssl.certificate.location=/ssl/certs/tls.crt -X ssl.ca.location=/ssl/certs/ca.crt"
	}

	consumedMessages, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
		k8s.NewKubectlOptions(kubectlOptions.ContextName, kubectlOptions.ConfigPath, ""),
		"exec", kcatPodName,
		"-n", kubectlOptions.Namespace,
		"--",
		"/bin/sh", "-c", fmt.Sprintf("kcat -L -b %s %s -t %s -e -C ", internalKafkaAddress, kcatTLSParameters, topicName),
	)

	if err != nil {
		return "", err
	}

	return consumedMessages, nil
}

// producingMessagesInternally produces messages based on the parameters into kafka cluster.
func producingMessagesInternally(kubectlOptions k8s.KubectlOptions, kcatPodName string, internalKafkaAddress string, topicName string, message string, tlsMode bool) error {
	By(fmt.Sprintf("Producing messages: '%s' to internalKafkaAddress: '%s' topicName: '%s'", message, internalKafkaAddress, topicName))

	kcatTLSParameters := ""
	if tlsMode {
		kcatTLSParameters += "-X security.protocol=SSL -X ssl.key.location=/ssl/certs/tls.key -X ssl.certificate.location=/ssl/certs/tls.crt -X ssl.ca.location=/ssl/certs/ca.crt"
	}

	_, err := k8s.RunKubectlAndGetOutputE(GinkgoT(),
		k8s.NewKubectlOptions(kubectlOptions.ContextName, kubectlOptions.ConfigPath, ""),
		"exec", kcatPodName,
		"-n", kubectlOptions.Namespace,
		"--",
		"/bin/sh", "-c", fmt.Sprintf("echo %s | kcat -L -b %s %s -t %s -P",
			message, internalKafkaAddress, kcatTLSParameters, topicName),
	)

	return err
}
