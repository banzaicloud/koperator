package e2e

import (
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
)

func testProduceConsumeInternal(kubectlOptions k8s.KubectlOptions) bool {
	return When("Internally produce and consume message to/from Kafka cluster", func() {
		kubectlOptions.Namespace = koperatorLocalHelmDescriptor.Namespace

		requireDeployingKcatPod(kubectlOptions, kcatPodName, "")
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatPodName, testInternalTopicName, "")
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatPodName)
	})
}

func testProduceConsumeExternal(kubectlOptions k8s.KubectlOptions, tlsSecretName string) bool {
	return When("Externally produce and consume message to/from Kafka cluster", func() {
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
