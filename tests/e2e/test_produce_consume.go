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

		requireDeployingKcatPod(kubectlOptions, kcatPodName)
		requireDeployingKafkaTopic(kubectlOptions, testTopicName)
		requireInternalProducingConsumingMessage(kubectlOptions, "", kcatPodName, testTopicName)
		requireDeleteKafkaTopic(kubectlOptions, testTopicName)
		requireDeleteKcatPod(kubectlOptions, kcatPodName)
	})
}
