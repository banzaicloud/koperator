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
	"strings"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

func testWebhookKafkaTopic(kafkaCluster types.NamespacedName) {
	// temporary section; to be refactored after kubeconfig injection PR
	var kubectlOptions k8s.KubectlOptions
	var err error
	kubectlOptions, err = kubectlOptionsForCurrentContext()
	if err != nil {
		GinkgoT().Fail()
	}

	kubectlOptions.Namespace = kafkaCluster.Namespace

	testWebhookCreateKafkaTopic(kubectlOptions, kafkaCluster)
	testWebhookUpdateKafkaTopic(kubectlOptions, kafkaCluster)

}

func testWebhookCreateKafkaTopic(kubectlOptions k8s.KubectlOptions, kafkaCluster types.NamespacedName) bool {
	return When("Testing KafkaTopic Create", func() {
		BeforeAll(func() {
			Expect(isExistingK8SResource(kubectlOptions, kafkaKind, kafkaClusterName)).To(BeTrue())
		})

		const nonExistent string = "non-existent"

		baseKafkaTopicTemplateValues := baseKafkaTopicData(
			types.NamespacedName{Name: testInternalTopicName, Namespace: kubectlOptions.Namespace},
			kafkaCluster,
		)

		It("Test non-existent KafkaCluster", func() {
			caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)
			caseData["ClusterRef"] = map[string]string{
				"Name":      nonExistent,
				"Namespace": kafkaCluster.Namespace,
			}
			err := applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal" is invalid: spec.clusterRef.name: Invalid value: "NON-EXISTENT": kafkaCluster 'NON-EXISTENT' in the namespace 'kafka' does not exist
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(1))
			Expect(err.Error()).To(And(
				ContainSubstring("The KafkaTopic %[1]q is invalid: spec.clusterRef.name: Invalid value: %[2]q: kafkaCluster '%[2]s' in the namespace '%[3]s' does not exist",
					caseData["Name"], nonExistent, kubectlOptions.Namespace),
			))
		})

		It("Test 0 partitions and replicationFactor", func() {
			caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)
			caseData["Partition"] = "0"
			caseData["ReplicationFactor"] = "0"
			err := applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal" is invalid:
			// * spec.partitions: Invalid value: 0: number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)
			// * spec.replicationFactor: Invalid value: 0: replication factor must be larger than 0 (or set it to be -1 to use the broker's default)
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(3))
			Expect(err.Error()).To(And(
				ContainSubstring("The KafkaTopic %q is invalid:", caseData["Name"]),
				ContainSubstring("spec.partitions: Invalid value: %s: number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)", caseData["Partition"]),
				ContainSubstring("spec.replicationFactor: Invalid value: %s: replication factor must be larger than 0 (or set it to be -1 to use the broker's default)", caseData["ReplicationFactor"]),
			))
		})

		// In the current validation webhook implementation, this case can only be encountered on a Create operation
		It("Test ReplicationFactor larger than number of brokers", func() {
			caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)
			caseData["ReplicationFactor"] = "10"
			err := applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal" is invalid: spec.replicationFactor: Invalid value: 10: replication factor is larger than the number of nodes in the kafka cluster (available brokers: 3)
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(1))
			Expect(err.Error()).To(
				ContainSubstring("The KafkaTopic %[1]q is invalid: spec.replicationFactor: Invalid value: %[2]s: replication factor is larger than the number of nodes in the kafka cluster",
					caseData["Name"], caseData["ReplicationFactor"]),
			)
		})

		// Test case involving existing CRs but not necessarily an Update operation
		When("Testing conflicts similar CRs", Ordered, func() {
			requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)

			It("Testing conflict on spec.name", func() {
				caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)

				switch v := caseData["Name"].(type) {
				case string:
					caseData["Name"] = v + "-different-cr-name"
				case fmt.Stringer:
					caseData["Name"] = v.String() + "-different-cr-name"
				default:
					caseData["Name"] = nonExistent
				}

				By("With managedBy koperator annotation")
				caseData["Annotations"] = []string{"managedBy: koperator"}
				err := applyK8sResourceFromTemplate(
					kubectlOptions,
					kafkaTopicTemplate,
					caseData,
					dryRunStrategyArgServer,
				)
				Expect(err).To(HaveOccurred())
				// Example error:
				// error while running command: exit status 1; The KafkaTopic "topic-test-internal-different-cr-name" is invalid: spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
				Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(1))
				Expect(err.Error()).To(
					ContainSubstring("The KafkaTopic %[1]q is invalid: spec.name: Invalid value: %[2]q: kafkaTopic CR '%[2]s' in namesapce '%[3]s' is already referencing to Kafka topic '%[2]s'",
						caseData["Name"], testInternalTopicName, kubectlOptions.Namespace),
				)

				By("Without managedBy koperator annotation")
				caseData["Annotations"] = []string{}
				err = applyK8sResourceFromTemplate(
					kubectlOptions,
					kafkaTopicTemplate,
					caseData,
					dryRunStrategyArgServer,
				)
				Expect(err).To(HaveOccurred())
				// Example error:
				// error while running command: exit status 1; The KafkaTopic "topic-test-internal-different-cr-name" is invalid:
				// * spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
				// * spec.name: Invalid value: "topic-test-internal": topic "topic-test-internal" already exists on kafka cluster and it is not managed by Koperator,
				// 					if you want it to be managed by Koperator so you can modify its configurations through a KafkaTopic CR,
				// 					add this "managedBy: koperator" annotation to this KafkaTopic CR
				Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(5))
				Expect(err.Error()).To(And(
					ContainSubstring("The KafkaTopic %q is invalid:", caseData["Name"]),
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

func testWebhookUpdateKafkaTopic(kubectlOptions k8s.KubectlOptions, kafkaCluster types.NamespacedName) bool {
	return When("Testing KafkaTopic Update", func() {
		BeforeAll(func() {
			Expect(isExistingK8SResource(kubectlOptions, kafkaKind, kafkaClusterName)).To(BeTrue())
		})

		const nonExistent string = "non-existent"

		baseKafkaTopicTemplateValues := baseKafkaTopicData(
			types.NamespacedName{Name: testInternalTopicName, Namespace: kubectlOptions.Namespace},
			kafkaCluster,
		)

		// Update operation implies having a CR with the same name in place
		requireDeployingKafkaTopic(kubectlOptions, testInternalTopicName)

		It("Test non-existent KafkaCluster", func() {
			caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)
			caseData["ClusterRef"] = map[string]string{
				"Name":      nonExistent,
				"Namespace": kafkaCluster.Namespace,
			}
			err := applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal" is invalid: spec.clusterRef.name: Invalid value: "non-existent": kafkaCluster 'non-existent' in the namespace 'kafka' does not exist
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(1))
			Expect(err.Error()).To(
				ContainSubstring("The KafkaTopic %[1]q is invalid: spec.clusterRef.name: Invalid value: %[2]q: kafkaCluster '%[2]s' in the namespace '%[3]s' does not exist",
					caseData["Name"], nonExistent, kubectlOptions.Namespace),
			)
		})

		// A successfully created KafkaTopic CR cannot have 0 for either Partition or ReplicationFactor.
		// At the same time, during an Update, a KafkaTopic cannot have its:
		// * spec.partitions decreased
		// * spec.replicationFactor changed (not just decreased)
		// Consequently, an Update test for 0 values will automatically also cover the decreasing/changing scenarios.
		It("Test 0 values partitions and replicationFactor", func() {
			caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)
			caseData["Partition"] = "0"
			caseData["ReplicationFactor"] = "0"
			err := applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal" is invalid:
			// * spec.partitions: Invalid value: 0: number of partitions must be larger than 0 (or set it to be -1 to use the broker's default)
			// * spec.replicationFactor: Invalid value: 0: replication factor must be larger than 0 (or set it to be -1 to use the broker's default)
			// * spec.partitions: Invalid value: 0: kafka does not support decreasing partition count on an existing topic (from 2 to 0)
			// * spec.replicationFactor: Invalid value: 0: kafka does not support changing the replication factor on an existing topic (from 2 to 0)
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(5))
			Expect(err.Error()).To(And(
				ContainSubstring("The KafkaTopic %q is invalid:", caseData["Name"]),
				ContainSubstring("spec.partitions: Invalid value: 0: number of partitions must be larger than 0"),
				ContainSubstring("spec.replicationFactor: Invalid value: 0: replication factor must be larger than 0"),
				ContainSubstring("spec.partitions: Invalid value: %s: kafka does not support decreasing partition count on an existing topic", caseData["Partition"]),
				ContainSubstring("spec.replicationFactor: Invalid value: %s: kafka does not support changing the replication factor on an existing topic", caseData["ReplicationFactor"]),
			))
		})

		It("Testing conflict on spec.name", func() {
			caseData := copyMapWithStringKeys(baseKafkaTopicTemplateValues)

			switch v := caseData["Name"].(type) {
			case string:
				caseData["Name"] = v + "-different-cr-name"
			case fmt.Stringer:
				caseData["Name"] = v.String() + "-different-cr-name"
			default:
				caseData["Name"] = nonExistent
			}

			By("With managedBy koperator annotation")
			caseData["Annotations"] = []string{"managedBy: koperator"}
			err := applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal-different-cr-name" is invalid: spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(1))
			Expect(err.Error()).To(
				ContainSubstring("The KafkaTopic %[1]q is invalid: spec.name: Invalid value: %[2]q: kafkaTopic CR '%[2]s' in namesapce '%[3]s' is already referencing to Kafka topic '%[2]s'",
					caseData["Name"], testInternalTopicName, kubectlOptions.Namespace),
			)

			By("Without managedBy koperator annotation")
			caseData["Annotations"] = []string{}
			err = applyK8sResourceFromTemplate(
				kubectlOptions,
				kafkaTopicTemplate,
				caseData,
				dryRunStrategyArgServer,
			)
			Expect(err).To(HaveOccurred())
			// Example error:
			// error while running command: exit status 1; The KafkaTopic "topic-test-internal-different-cr-name" is invalid:
			// * spec.name: Invalid value: "topic-test-internal": kafkaTopic CR 'topic-test-internal' in namesapce 'kafka' is already referencing to Kafka topic 'topic-test-internal'
			// * spec.name: Invalid value: "topic-test-internal": topic "topic-test-internal" already exists on kafka cluster and it is not managed by Koperator,
			// 					if you want it to be managed by Koperator so you can modify its configurations through a KafkaTopic CR,
			// 					add this "managedBy: koperator" annotation to this KafkaTopic CR
			Expect(len(strings.Split(err.Error(), "\n"))).To(Equal(5))
			Expect(err.Error()).To(And(
				ContainSubstring("The KafkaTopic %q is invalid:", caseData["Name"]),
				ContainSubstring("spec.name: Invalid value: %[1]q: kafkaTopic CR '%[1]s' in namesapce '%[2]s' is already referencing to Kafka topic '%[1]s'\n",
					testInternalTopicName, kubectlOptions.Namespace),
				ContainSubstring("spec.name: Invalid value: %[1]q: topic %[1]q already exists on kafka cluster and it is not managed by Koperator",
					testInternalTopicName),
			))
		})

		// Clean up the KafkaTopic set up to test Update operations against
		requireDeleteKafkaTopic(kubectlOptions, testInternalTopicName)
	})
}

func baseKafkaTopicData(kafkaTopic types.NamespacedName, kafkaCluster types.NamespacedName) map[string]interface{} {
	return map[string]interface{}{
		"Name":              kafkaTopic.Name,
		"TopicName":         kafkaTopic.Name,
		"Namespace":         kafkaTopic.Namespace,
		"Partition":         "2",
		"ReplicationFactor": "2",
		"ClusterRef": map[string]string{
			"Name":      kafkaCluster.Name,
			"Namespace": kafkaCluster.Namespace,
		},
		"Annotations": []string{"managedBy: koperator"},
	}
}

func copyMapWithStringKeys(oldMap map[string]interface{}) map[string]interface{} {
	var newMap = make(map[string]interface{})
	for k := range oldMap {
		newMap[k] = oldMap[k]
	}
	return newMap
}
