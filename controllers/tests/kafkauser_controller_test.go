// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/banzaicloud/koperator/pkg/pki/k8scsrpki"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/IBM/sarama"

	certsigningreqv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
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

	JustBeforeEach(func(ctx SpecContext) {
		By("creating namespace " + namespace)
		err := k8sClient.Create(ctx, namespaceObj)
		Expect(err).NotTo(HaveOccurred())

		By("creating kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err = k8sClient.Create(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())

		waitForClusterRunningState(ctx, kafkaCluster, namespace)
	})

	JustAfterEach(func(ctx SpecContext) {
		resetMockKafkaClient(kafkaCluster)

		By("deleting Kafka cluster object " + kafkaCluster.Name + " in namespace " + namespace)
		err := k8sClient.Delete(ctx, kafkaCluster)
		Expect(err).NotTo(HaveOccurred())
		kafkaCluster = nil
	})

	It("generates topic grants correctly", func(ctx SpecContext) {
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
				CreateCert: util.BoolPointer(false),
			},
		}

		err := k8sClient.Create(ctx, &user)
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, func() (v1alpha1.UserState, error) {
			user := v1alpha1.KafkaUser{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      userCRName,
			}, &user)
			if err != nil {
				return "", err
			}
			return user.Status.State, nil
		}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.UserStateCreated))

		// check label on user
		err = k8sClient.Get(ctx, types.NamespacedName{
			Namespace: kafkaCluster.Namespace,
			Name:      userCRName,
		}, &user)
		Expect(err).NotTo(HaveOccurred())
		Expect(user.Labels).To(HaveKeyWithValue("kafkaCluster", fmt.Sprintf("%s.%s", kafkaClusterCRName, namespace)))

		mockKafkaClient, _ := getMockedKafkaClientForCluster(kafkaCluster)
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
	It("k8s csr and belonging secret correctly", func(ctx SpecContext) {
		userCRName := fmt.Sprintf("kafkauser-%v", count)
		user := v1alpha1.KafkaUser{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userCRName,
				Namespace: namespace,
			},
			Spec: v1alpha1.KafkaUserSpec{
				SecretName: userCRName,
				IncludeJKS: true,
				ClusterRef: v1alpha1.ClusterReference{
					Namespace: namespace,
					Name:      kafkaClusterCRName,
				},
				PKIBackendSpec: &v1alpha1.PKIBackendSpec{
					PKIBackend: string(v1beta1.PKIBackendK8sCSR),
					SignerName: "foo.bar/foobar",
				},
			},
		}
		err := k8sClient.Create(ctx, &user)
		Expect(err).NotTo(HaveOccurred())

		secret := &corev1.Secret{}
		Eventually(ctx, func() map[string]string {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      user.Spec.SecretName,
				Namespace: user.Namespace}, secret)
			if !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
			return secret.Annotations
		}, 5*time.Second, 100*time.Millisecond).Should(HaveKey(k8scsrpki.DependingCsrAnnotation))

		csrName := secret.Annotations[k8scsrpki.DependingCsrAnnotation]
		csr := &certsigningreqv1.CertificateSigningRequest{}
		Eventually(ctx, func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      csrName,
				Namespace: user.Namespace}, csr)
			if err != nil {
				return err
			}
			return nil
		}, 5*time.Second, 100*time.Millisecond).Should(BeNil())

		csr.Status.Conditions = append(csr.Status.Conditions, certsigningreqv1.CertificateSigningRequestCondition{
			Type:   certsigningreqv1.CertificateApproved,
			Status: "True",
		})
		csr, err = csrClient.CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      csrName,
				Namespace: user.Namespace}, csr)
			if err != nil {
				return err
			}
			return nil
		}, 5*time.Second, 100*time.Millisecond).Should(BeNil())
		Expect(csr.Status.Conditions).NotTo(BeNil())

		csr.Status.Certificate = []byte(`-----BEGIN CERTIFICATE-----
MIIDWDCCAkCgAwIBAgIRAIHglYhplUvkCLoKrzTpT5cwDQYJKoZIhvcNAQELBQAw
ZDELMAkGA1UEBhMCSFUxETAPBgNVBAgMCEJ1ZGFwZXN0MREwDwYDVQQHDAhCdWRh
cGVzdDEVMBMGA1UECgwMQmFuemFpIENsb3VkMRgwFgYDVQQDDA9DU1IgT3BlcmF0
b3IgQ0EwHhcNMjEwODA5MDgxOTMyWhcNMjEwODEwMDgxOTMyWjAnMQkwBwYDVQQK
EwAxGjAYBgNVBAMTEWV4YW1wbGUta2Fma2F1c2VyMIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEAu9p3fpK22UBrufdlxZEcUQo2rP9d6tXx9P1BChPoJ3DM
S8b3XOqDWEGvfw+qNLgfQx2fsainu5PD2TDMH+44BUT4m94SCuEV273THTIYcT2s
YedOKTyGLBs44CdD/8SWgBlEr6pnMNzrgr/VlPM6jYqrIZd/mtZcBQVwbkD6Lw4J
m1aIzg7NzCh1W/tv4+L7fEwJyGRYAcT3sZbkeU/Es11ZWQSYj5ar3ET5Z2MZdwFh
l7TyVbd1gts4HkkafGnP/9ZK6yjUQG3QlDVHlzSbPq5vp1cghuEnPEiGAJ7N7eE2
VOQxgQZk6ycPbDZc/FB6R6DmllCZL4XvV+8AEh4y5QIDAQABo0IwQDAdBgNVHSUE
FjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwHwYDVR0jBBgwFoAUykkfnl5IZWXp2CQG
5sFob7TdzkcwDQYJKoZIhvcNAQELBQADggEBAH/I8ShIiPFavQlGM7HNk+oS1TPE
LRo23BIX2AB9B0qg9a4IohYNyEPkrLtV4T5+lYQGctwgiDIVlQb6PtcnNm8G9I35
s2bDvpmKE81E3oZK51iVymjf7UIL92cUza/4C2pIgpdPnKJG/3jFQHeNZjtn71Bh
12nRYhUqJNsFxtNVhJiqj8k880BL2YGGcnaDaGpwc6ZyjsI662n1/zDBStfn5GRn
G3JFndZFiWYLZhkF9umOvIqHadjjwwXUmu5KLTJirGUZMTZnrsFh8kcNF261dnH4
pwq+04gtU+xOk0yBPefQEYqmOIR26F+T8AATgLXBJ8Ll3AtzgV2t4ugOraw=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIEFTCCAv2gAwIBAgIJAN0hg1UUjv+lMA0GCSqGSIb3DQEBDQUAMGQxCzAJBgNV
BAYTAkhVMREwDwYDVQQIDAhCdWRhcGVzdDERMA8GA1UEBwwIQnVkYXBlc3QxFTAT
BgNVBAoMDEJhbnphaSBDbG91ZDEYMBYGA1UEAwwPQ1NSIE9wZXJhdG9yIENBMB4X
DTIxMDgwNDEzMzMwOVoXDTIyMDgwNDEzMzMwOVowZDELMAkGA1UEBhMCSFUxETAP
BgNVBAgMCEJ1ZGFwZXN0MREwDwYDVQQHDAhCdWRhcGVzdDEVMBMGA1UECgwMQmFu
emFpIENsb3VkMRgwFgYDVQQDDA9DU1IgT3BlcmF0b3IgQ0EwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDOyqB4IaMvzqMRCMNMLn8ryHQi03a2B4nbWDIE
xXjECSGc1CUe7pkxzkxJy9PSHoEntMKoSfDbnqgc2ceH5FHb13lyhMR86Q9DfDI8
fkK6OMzC6hIs/rXqaaJOyIOBGj38QB8F5lC5Yodzw5iSSIZBdkNIx9abVUFF0FII
4p0Khf3D8bRuQlQ1OBcf09a0FfipSw7ZWYXEf2zPeto5OdqUfMD+5XkfOMxGCsH1
Y9Uc7OWB/Vx+r4Ud9LO5nMtQATqoTINQOaPZHILRFENPuh3+G8kCIHMtLPbDkgTE
v98PCxfNBeNGHAGVIuJs3eI3QGBJShYb3aSVaI5tqqd6y+iTAgMBAAGjgckwgcYw
HQYDVR0OBBYEFMpJH55eSGVl6dgkBubBaG+03c5HMIGWBgNVHSMEgY4wgYuAFMpJ
H55eSGVl6dgkBubBaG+03c5HoWikZjBkMQswCQYDVQQGEwJIVTERMA8GA1UECAwI
QnVkYXBlc3QxETAPBgNVBAcMCEJ1ZGFwZXN0MRUwEwYDVQQKDAxCYW56YWkgQ2xv
dWQxGDAWBgNVBAMMD0NTUiBPcGVyYXRvciBDQYIJAN0hg1UUjv+lMAwGA1UdEwQF
MAMBAf8wDQYJKoZIhvcNAQENBQADggEBAIEEDzdUHK1a78MadPbDICDpZ/blPueZ
1gCSqjj9/llQPyKAFp33vBRDXgv/QIKzZ/Sglss5f7FJAlIi4dMwSCbLQD+VBu0o
8AcO1yLc6INTA4FDm6WX0A0N4AB5JnoOlIKY5EUwpNhiJyAeBMPffBy0iRue1yOa
a2XtH5jFl9AhECz2vjTCDWnAicihyI7IQvz2OWAWhrlj1hX1MVGKbJxmzzgKXD0L
x0eepFeUNacaeg7O1ftIrzNlYsSLi2Qm+tnu7odyxafZ65GJ9lcSLUqXuNDCrNOl
1KqnZqPj4ZdS6obB0ep879Z865k7OH0GFgTd7k+UcMPcjEkcFPLPNEI=
-----END CERTIFICATE-----`)
		csr, err = csrClient.CertificateSigningRequests().UpdateStatus(ctx, csr, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, func() map[string][]byte {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      user.Spec.SecretName,
				Namespace: user.Namespace}, secret)
			if !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
			return secret.Data
		}, 5*time.Second, 100*time.Millisecond).Should(HaveLen(6))

		err = k8sClient.Get(ctx, types.NamespacedName{
			Name:      user.Spec.SecretName,
			Namespace: user.Namespace}, secret)
		Expect(err).NotTo(HaveOccurred())
		for _, data := range secret.Data {
			Expect(len(data)).ShouldNot(BeZero())
		}

		Eventually(ctx, func() (v1alpha1.UserState, error) {
			user := v1alpha1.KafkaUser{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: kafkaCluster.Namespace,
				Name:      userCRName,
			}, &user)
			if err != nil {
				return "", err
			}
			return user.Status.State, nil
		}, 5*time.Second, 100*time.Millisecond).Should(Equal(v1alpha1.UserStateCreated))
	})
})
