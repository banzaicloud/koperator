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

		requireDeployingKcatPod(kubectlOptions, kcatPodName, "")
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatPodName, testInternalTopicName, "")
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatPodName)
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

		requireCreatingKafkaUser(kubectlOptions, kafkaUserName, tlsSecretName)
		requireDeployingKcatPod(kubectlOptions, kcatPodName, tlsSecretName)
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatPodName, testInternalTopicName, tlsSecretName)
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatPodName)
	})
}

func testProduceConsumeExternal(tlsSecretName string) bool {
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
