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

func testProduceConsumeInternal() bool {
	return When("Internally produce and consume message to/from Kafka cluster", func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		It("Acquiring K8s config and context", func() {
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		kubectlOptions.Namespace = koperatorLocalHelmDescriptor.Namespace

		requireDeployingKcatPod(kubectlOptions, kcatName, "")
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatName, testInternalTopicName, "")
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatName)
	})
}

func testProduceConsumeInternalSSL(tlsSecretName string) bool {
	return When("Internally produce and consume message to/from Kafka cluster using SSL", func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		It("Acquiring K8s config and context", func() {
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		kubectlOptions.Namespace = koperatorLocalHelmDescriptor.Namespace

		requireDeployingKafkaUser(kubectlOptions, kafkaUserName, tlsSecretName)
		requireDeployingKcatPod(kubectlOptions, kcatName, tlsSecretName)
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatName, testInternalTopicName, tlsSecretName)
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatName)
		requireDeleteKafkaUser(kubectlOptions, kafkaUserName)
	})
}

func testProduceConsumeExternal(tlsSecretName string) bool { //nolint:unused // Note: unused linter disabled until External e2e tests are turned on.
	return When("Externally produce and consume message to/from Kafka cluster", func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		It("Acquiring K8s config and context", func() {
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		kubectlOptions.Namespace = koperatorLocalHelmDescriptor.Namespace

		requireDeployingKafkaTopic(kubectlOptions, testExternalTopicName)
		// requireAvailableExternalKafkaAddress is not necessary now but I think later can be useful
		// when we need to wait before consuming producing to get LB service ip address and domain resolution
		// in those cases when externalListener added dynamically.
		// Currently we dont know much about our testing environment.
		// requireAvailableExternalKafkaAddress(kubectlOptions, "", kafkaClusterName)
		requireExternalProducingConsumingMessage(kubectlOptions, testExternalTopicName, tlsSecretName)
		requireDeleteKafkaTopic(kubectlOptions, testExternalTopicName)
	})
}
