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
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

// requireDeleteKafkaTopic deletes kafkaTopic resource.
func requireDeleteKafkaTopic(kubectlOptions k8s.KubectlOptions, topicName string) {
	It("Deleting KafkaTopic CR", func() {
		err := deleteK8sResource(kubectlOptions, defaultDeletionTimeout, kafkaTopicKind, "", topicName)
		Expect(err).NotTo(HaveOccurred())
	})
}

// requireDeployingKafkaTopic deploys a kafkaTopic resource from a template
func requireDeployingKafkaTopic(kubectlOptions k8s.KubectlOptions, topicName string) {
	It("Deploying KafkaTopic CR", func() {
		// err := applyK8sResourceFromTemplate(kubectlOptions,
		// 	kafkaTopicTemplate,
		// 	map[string]interface{}{
		// 		"Name":      topicName,
		// 		"TopicName": topicName,
		// 		"Namespace": kubectlOptions.Namespace,
		// 	},
		// )
		values := kafkaTopicTemplateData{
			Annotations: []string{"managedBy: koperator"},
			ClusterRef: kafkaTopicClusterRef{
				Name:      kafkaClusterName,
				Namespace: kubectlOptions.Namespace,
			},
			Name:              topicName,
			Namespace:         kubectlOptions.Namespace,
			Partitions:        ptr.To(int32(2)),
			ReplicationFactor: ptr.To(int32(2)),
			TopicName:         topicName,
		}

		err := applyK8sResourceFromTemplate_2(kubectlOptions, kafkaTopicTemplate, values)

		Expect(err).ShouldNot(HaveOccurred())

		err = waitK8sResourceCondition(kubectlOptions, kafkaTopicKind,
			"jsonpath={.status.state}=created", defaultTopicCreationWaitTime, "", topicName)

		Expect(err).ShouldNot(HaveOccurred())
	})
}

// requireDeleteKafkaUser deletes a kafkaUser resource by name
func requireDeleteKafkaUser(kubectlOptions k8s.KubectlOptions, userName string) {
	It("Deleting KafkaUser CR", func() {
		err := deleteK8sResource(kubectlOptions, defaultDeletionTimeout, kafkaUserKind, "", userName)
		Expect(err).NotTo(HaveOccurred())
	})
}

// requireDeployingKafkaUser creates a KafkaUser resource from a template
func requireDeployingKafkaUser(kubectlOptions k8s.KubectlOptions, userName string, tlsSecretName string) {
	It("Deploying KafkaUser CR", func() {
		templateParameters := map[string]interface{}{
			"Name":        userName,
			"Namespace":   kubectlOptions.Namespace,
			"ClusterName": kafkaClusterName,
		}
		if tlsSecretName != "" {
			templateParameters["TLSSecretName"] = tlsSecretName
		}

		err := applyK8sResourceFromTemplate(kubectlOptions,
			kafkaUserTemplate,
			templateParameters,
		)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(context.Background(), func() bool {
			return isExistingK8SResource(kubectlOptions, "Secret", tlsSecretName)
		}, defaultUserCreationWaitTime, 3*time.Second).Should(Equal(true))
	})
}

// kafkaTopicTemplateData is a struct that holds the relevant information and structure
// to fill out the template used to generate KafkaTopics.
// TODO: long term we should use the structs in the api module instead of these local structs.
type kafkaTopicTemplateData struct {
	Annotations       []string
	ClusterRef        kafkaTopicClusterRef
	Name              string
	Namespace         string
	Partitions        *int32
	ReplicationFactor *int32
	TopicName         string
}

// kafkaTopicClusterRef holds the information relevant to identifying a KafkaCluster within a KafkaTopic CR.
// TODO: Long term, we should use the structs in the api module instead of these local structs.
type kafkaTopicClusterRef struct {
	Name      string
	Namespace string
}
