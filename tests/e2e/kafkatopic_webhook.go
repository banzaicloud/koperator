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
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TODO(mihalexa): move to k8s.go
const (
	dryRunStrategyNone   string = "none"
	dryRunStrategyClient string = "client"
	dryRunStrategyServer string = "server"
)

// TODO(mihalexa): move to k8s.go
// applyK8sResourceFromTemplateWithDryRun is copy of applyK8sResourceFromTemplate which calls a "--dry-run=<strategy>" kubectl command
func applyK8sResourceFromTemplateWithDryRun(kubectlOptions k8s.KubectlOptions, templateFile string, values map[string]interface{}, dryRunStrategy string) (string, error) {
	By(fmt.Sprintf("Generating K8s manifest from template %s for dry-run apply", templateFile))
	var manifest bytes.Buffer
	rawTemplate, err := os.ReadFile(templateFile)
	if err != nil {
		return "", err
	}
	t := template.Must(template.New("template").Funcs(sprig.TxtFuncMap()).Parse(string(rawTemplate)))
	err = t.Execute(&manifest, values)
	if err != nil {
		return "", err
	}

	By("Replicating terratest's k8s.KubectlApplyFromStringE")
	tmpfile, err := k8s.StoreConfigToTempFileE(GinkgoT(), manifest.String())
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile)

	return k8s.RunKubectlAndGetOutputE(GinkgoT(), &kubectlOptions, "apply", "-f", tmpfile, "--dry-run="+dryRunStrategy, "--output=yaml")
}

func testWebhooks() bool {
	return When("Testing webhooks", func() {
		// temporary section; to be refactored after kubeconfig injection PR
		var kubectlOptions k8s.KubectlOptions
		var err error
		kubectlOptions, err = kubectlOptionsForCurrentContext()
		if err != nil {
			GinkgoT().Fail()
		}

		testWebhookKafkaTopic(kubectlOptions)
	})
}

func testWebhookKafkaTopic(kubectlOptions k8s.KubectlOptions) {
	kubectlOptions.Namespace = koperatorLocalHelmDescriptor.Namespace
	requireCreatingKafkaCluster(kubectlOptions, "../../config/samples/simplekafkacluster.yaml")
	testWebhookCreateKafkaTopic(kubectlOptions)
	testWebhookUpdateKafkaTopic(kubectlOptions)
	requireDeleteKafkaCluster(kubectlOptions, kafkaClusterName)
}

func testWebhookCreateKafkaTopic(kubectlOptions k8s.KubectlOptions) bool {
	return When("Testing KafkaTopic Create", func() {
		BeforeAll(func() {
			Expect(isExistingK8SResource(kubectlOptions, kafkaKind, kafkaClusterName)).To(BeTrue())
		})

		It("Test non-existent KafkaCluster", func() {
			output, err := applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":      testInternalTopicName,
					"TopicName": testInternalTopicName,
					"Namespace": kubectlOptions.Namespace,
					"ClusterRef": map[string]string{
						"Name":      kafkaClusterName + "NOT",
						"Namespace": kubectlOptions.Namespace,
					},
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error: The KafkaTopic "topic-test-internal" is invalid: spec.clusterRef.name: Invalid value: "kafkaNOT": kafkaCluster 'kafkaNOT' in the namespace 'kafka' does not exist
			Expect(len(strings.Split(output, "\n"))).To(Equal(1))
			Expect(output).To(
				ContainSubstring("spec.clusterRef.name: Invalid value: %[1]q: kafkaCluster '%[1]s' in the namespace '%[2]s' does not exist",
					kafkaClusterName+"NOT", kubectlOptions.Namespace),
			)
		})

		It("Test 0 partitions and replicationFactor", func() {
			output, err := applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":              testInternalTopicName,
					"TopicName":         testInternalTopicName,
					"Partition":         "0",
					"ReplicationFactor": "0",
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// The KafkaTopic "topic-test-internal" is invalid:
			// * spec.partitions: Invalid value: 0: number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)
			// * spec.replicationFactor: Invalid value: 0: replication factor must be larger than 0 (or set it to be -1 to use the broker's default)
			Expect(len(strings.Split(output, "\n"))).To(Equal(3))
			Expect(output).To(And(
				ContainSubstring("spec.partitions: Invalid value: %s: number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)", "0"),
				ContainSubstring("spec.replicationFactor: Invalid value: %s: replication factor must be larger than 0 (or set it to be -1 to use the broker's default)", "0"),
			))
		})

		// In the current validation webhook implementation, this case can only be encountered on a Create operation
		It("Test ReplicationFactor larger than number of brokers", func() {
			output, err := applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":              testInternalTopicName,
					"TopicName":         testInternalTopicName,
					"ReplicationFactor": "10",
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// The KafkaTopic "topic-test-internal" is invalid: spec.replicationFactor: Invalid value: 10: replication factor is larger than the number of nodes in the kafka cluster (available brokers: 3)
			Expect(len(strings.Split(output, "\n"))).To(Equal(1))
			Expect(output).To(And(
				ContainSubstring("spec.replicationFactor: Invalid value"),
				ContainSubstring("replication factor is larger than the number of nodes in the kafka cluster"),
			))
		})

		// Test case involving existing CRs but not necessarily an Update operation
		When("Testing conflicts similar CRs", func() {
			requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)

			It("Testing conflict on spec.name", func() {
				By("With managedBy koperator annotation")
				output, err := applyK8sResourceFromTemplateWithDryRun(
					kubectlOptions,
					kafkaTopicTemplate,
					map[string]interface{}{
						"Name":        testInternalTopicName + "-different-cr-name",
						"TopicName":   testInternalTopicName,
						"Namespace":   kubectlOptions.Namespace,
						"Annotations": []string{"managedBy: koperator"},
					},
					dryRunStrategyServer,
				)
				Expect(err).To(HaveOccurred())
				// Example error:
				// The KafkaTopic "topic-test-internal-different-cr-name" is invalid: spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
				Expect(len(strings.Split(output, "\n"))).To(Equal(1))
				Expect(output).To(
					ContainSubstring("spec.name: Invalid value: %[1]q: kafkaTopic CR '%[1]s' in namesapce '%[2]s' is already referencing to Kafka topic '%[1]s'",
						testInternalTopicName, kubectlOptions.Namespace),
				)

				By("Without managedBy koperator annotation")
				output, err = applyK8sResourceFromTemplateWithDryRun(
					kubectlOptions,
					kafkaTopicTemplate,
					map[string]interface{}{
						"Name":      testInternalTopicName + "-different-cr-name",
						"TopicName": testInternalTopicName,
						"Namespace": kubectlOptions.Namespace,
					},
					dryRunStrategyServer,
				)
				Expect(err).To(HaveOccurred())
				// Example error:
				// The KafkaTopic "topic-test-internal-different-cr-name" is invalid:
				// * spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
				// * spec.name: Invalid value: "topic-test-internal": topic "topic-test-internal" already exists on kafka cluster and it is not managed by Koperator,
				// 					if you want it to be managed by Koperator so you can modify its configurations through a KafkaTopic CR,
				// 					add this "managedBy: koperator" annotation to this KafkaTopic CR
				Expect(len(strings.Split(output, "\n"))).To(Equal(5))
				Expect(output).To(And(
					ContainSubstring("spec.name: Invalid value: %[1]q: kafkaTopic CR '%[1]s' in namesapce '%[2]s' is already referencing to Kafka topic '%[1]s'",
						testInternalTopicName, kubectlOptions.Namespace),
					ContainSubstring("spec.name: Invalid value: %[1]q: topic %[1]q already exists on kafka cluster and it is not managed by Koperator",
						testInternalTopicName),
				))

			})

			requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
		})
	})
}

func testWebhookUpdateKafkaTopic(kubectlOptions k8s.KubectlOptions) bool {
	return When("Testing KafkaTopic Update", func() {
		BeforeAll(func() {
			Expect(isExistingK8SResource(kubectlOptions, kafkaKind, kafkaClusterName)).To(BeTrue())
		})

		// Update operation implies having a CR with the same name in place
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)

		It("Test non-existent KafkaCluster", func() {
			output, err := applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":      testInternalTopicName,
					"TopicName": testInternalTopicName,
					"Namespace": kubectlOptions.Namespace,
					"ClusterRef": map[string]string{
						"Name":      kafkaClusterName + "NOT",
						"Namespace": kubectlOptions.Namespace,
					},
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error: The KafkaTopic "topic-test-internal" is invalid: spec.clusterRef.name: Invalid value: "kafkaNOT": kafkaCluster 'kafkaNOT' in the namespace 'kafka' does not exist
			Expect(len(strings.Split(output, "\n"))).To(Equal(1))
			Expect(output).To(
				ContainSubstring("spec.clusterRef.name: Invalid value: %[1]q: kafkaCluster '%[1]s' in the namespace '%[2]s' does not exist",
					kafkaClusterName+"NOT", kubectlOptions.Namespace),
			)
		})

		// A successfully created KafkaTopic CR cannot have 0 for either Partition or ReplicationFactor.
		// At the same time, during an Update, a KafkaTopic cannot have its:
		// * spec.partitions decreased
		// * spec.replicationFactor changed (not just decreased)
		// Consequently, an Update test for 0 values will automatically also cover the decreasing/changing scenarios.
		It("Test 0 values partitions and replicationFactor", func() {
			output, err := applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":              testInternalTopicName,
					"TopicName":         testInternalTopicName,
					"Partition":         "0",
					"ReplicationFactor": "0",
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// The KafkaTopic "topic-test-internal" is invalid:
			// * spec.partitions: Invalid value: 0: number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)
			// * spec.replicationFactor: Invalid value: 0: replication factor must be larger than 0 (or set it to be -1 to use the broker's default)
			// * spec.partitions: Invalid value: 0: kafka does not support decreasing partition count on an existing topic (from 2 to 0)
			// * spec.replicationFactor: Invalid value: 0: kafka does not support changing the replication factor on an existing topic (from 2 to 0)
			Expect(len(strings.Split(output, "\n"))).To(Equal(5))
			Expect(output).To(And(
				ContainSubstring("spec.partitions: Invalid value: 0: number of partitions must be larger than 0"),
				ContainSubstring("spec.replicationFactor: Invalid value: 0: replication factor must be larger than 0"),
				ContainSubstring("spec.partitions: Invalid value: %s: kafka does not support decreasing partition count on an existing topic", "0"),
				ContainSubstring("spec.replicationFactor: Invalid value: %s: kafka does not support changing the replication factor on an existing topic", "0"),
			))
		})

		It("Testing conflict on spec.name", func() {
			By("With managedBy koperator annotation")
			output, err := applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":        testInternalTopicName + "-different-cr-name",
					"TopicName":   testInternalTopicName, // same spec.name as the KafkaTopic deployed in the beginning of the Update test scenario
					"Namespace":   kubectlOptions.Namespace,
					"Annotations": []string{"managedBy: koperator"},
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// The KafkaTopic "topic-test-internal-different-cr-name" is invalid: spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
			Expect(len(strings.Split(output, "\n"))).To(Equal(1))
			Expect(output).To(
				ContainSubstring("spec.name: Invalid value: %q: kafkaTopic CR '%s' in namesapce '%s' is already referencing to Kafka topic '%s'",
					testInternalTopicName, testInternalTopicName, kubectlOptions.Namespace, testInternalTopicName),
			)

			By("Without managedBy koperator annotation")
			output, err = applyK8sResourceFromTemplateWithDryRun(
				kubectlOptions,
				kafkaTopicTemplate,
				map[string]interface{}{
					"Name":      testInternalTopicName + "-different-cr-name",
					"TopicName": testInternalTopicName, // same spec.name as the KafkaTopic deployed in the beginning of the Update test scenario
					"Namespace": kubectlOptions.Namespace,
				},
				dryRunStrategyServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// The KafkaTopic "topic-test-internal-different-cr-name" is invalid:
			// * spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
			// * spec.name: Invalid value: "topic-test-internal": topic "topic-test-internal" already exists on kafka cluster and it is not managed by Koperator,
			// 					if you want it to be managed by Koperator so you can modify its configurations through a KafkaTopic CR,
			// 					add this "managedBy: koperator" annotation to this KafkaTopic CR
			Expect(len(strings.Split(output, "\n"))).To(Equal(5))
			Expect(output).To(And(
				ContainSubstring("spec.name: Invalid value: %[1]q: kafkaTopic CR '%[1]s' in namesapce '%[2]s' is already referencing to Kafka topic '%[1]s'",
					testInternalTopicName, kubectlOptions.Namespace),
				ContainSubstring("spec.name: Invalid value: %[1]q: topic %[1]q already exists on kafka cluster and it is not managed by Koperator",
					testInternalTopicName),
			))
		})

		// Clean up the KafkaTopic set up to test Update operations against
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
	})
}
