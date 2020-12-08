// Copyright © 2020 Banzai Cloud
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

package tests

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/Shopify/sarama"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

var _ = Describe("KafkaTopic", func() {
	var (
		count              uint64 = 0
		namespace          string
		namespaceObj       *corev1.Namespace
		kafkaClusterCRName string
		kafkaCluster       *v1beta1.KafkaCluster
	)

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)

		namespace = fmt.Sprintf("kafka-user-%v", count)
		namespaceObj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		kafkaClusterCRName = fmt.Sprintf("kafkacluster-%d", count)
		kafkaCluster = createMinimalKafkaClusterCR(kafkaClusterCRName, namespace)
	})

	JustBeforeEach(func() {
		By("creating namespace " + namespace)
		err := k8sClient.Create(context.TODO(), namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(kafkaCluster, namespace)
	})

	JustAfterEach(func() {
		resetMockKafkaClient(kafkaCluster)

		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(context.TODO(), kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	It("reconciles objects properly", func() {
		userCRName := fmt.Sprintf("kafkauser-%v", count)
		user := v1alpha1.KafkaUser{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userCRName,
				Namespace: namespace,
			},
			Spec: v1alpha1.KafkaUserSpec{
				ClusterRef: v1alpha1.ClusterReference{
					Namespace: namespace,
					Name:      kafkaClusterCRName,
				},
				TopicGrants: []v1alpha1.UserTopicGrant{
					{
						TopicName:   "test-topic-1",
						AccessType:  v1alpha1.KafkaAccessTypeRead,
						PatternType: v1alpha1.KafkaPatternTypeAny,
					},
					{
						TopicName:   "test-topic-2",
						AccessType:  v1alpha1.KafkaAccessTypeWrite,
						PatternType: v1alpha1.KafkaPatternTypeLiteral,
					},
				},
				CreateCert: util.BoolPointer(false), // TODO test when this is true
			},
		}

		err := k8sClient.Create(context.TODO(), &user)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() (v1alpha1.UserState, error) {
			user := v1alpha1.KafkaUser{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      userCRName,
			}, &user)
			if err != nil {
				return "", err
			}
			return user.Status.State, nil
		}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.UserStateCreated))

		// check label on user
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      userCRName,
		}, &user)
		Expect(err).NotTo(HaveOccurred())
		Expect(user.Labels).To(HaveKeyWithValue("kafkaCluster", fmt.Sprintf("%s.%s", kafkaClusterCRName, namespace)))

		mockKafkaClient := getMockedKafkaClientForCluster(kafkaCluster)
		acls, _ := mockKafkaClient.ListUserACLs()
		Expect(acls).To(ContainElements(
			sarama.ResourceAcls{
				Resource: sarama.Resource{
					ResourceType:        sarama.AclResourceTopic,
					ResourceName:        "test-topic-1",
					ResourcePatternType: sarama.AclPatternAny,
				},
				Acls: []*sarama.Acl{
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationDescribe,
						PermissionType: sarama.AclPermissionAllow,
					},
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationDescribeConfigs,
						PermissionType: sarama.AclPermissionAllow,
					},
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationRead,
						PermissionType: sarama.AclPermissionAllow,
					},
				},
			},
			sarama.ResourceAcls{
				Resource: sarama.Resource{
					ResourceType:        sarama.AclResourceTopic,
					ResourceName:        "test-topic-2",
					ResourcePatternType: sarama.AclPatternLiteral,
				},
				Acls: []*sarama.Acl{
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationDescribe,
						PermissionType: sarama.AclPermissionAllow,
					},
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationDescribeConfigs,
						PermissionType: sarama.AclPermissionAllow,
					},
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationWrite,
						PermissionType: sarama.AclPermissionAllow,
					},
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationCreate,
						PermissionType: sarama.AclPermissionAllow,
					},
				},
			},
			sarama.ResourceAcls{
				Resource: sarama.Resource{
					ResourceType:        sarama.AclResourceGroup,
					ResourceName:        "*",
					ResourcePatternType: sarama.AclPatternLiteral,
				},
				Acls: []*sarama.Acl{
					{
						Principal:      "User:CN=kafkauser-1",
						Host:           "*",
						Operation:      sarama.AclOperationRead,
						PermissionType: sarama.AclPermissionAllow,
					},
				},
			}))

		Expect(user.Status.ACLs).To(ConsistOf(
			"User:CN=kafkauser-1,Topic,ANY,test-topic-1,Describe,Allow,*",
			"User:CN=kafkauser-1,Topic,ANY,test-topic-1,Read,Allow,*",
			"User:CN=kafkauser-1,Group,LITERAL,*,Read,Allow,*",
			"User:CN=kafkauser-1,Topic,LITERAL,test-topic-2,Describe,Allow,*",
			"User:CN=kafkauser-1,Topic,LITERAL,test-topic-2,Create,Allow,*",
			"User:CN=kafkauser-1,Topic,LITERAL,test-topic-2,Write,Allow,*",
		))
	})
})
