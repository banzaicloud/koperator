// Copyright © 2019 Banzai Cloud
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

package pki

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
)

func testCluster(t *testing.T) *v1beta1.KafkaCluster {
	t.Helper()
	cluster := &v1beta1.KafkaCluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-namespace"
	cluster.Spec = v1beta1.KafkaClusterSpec{}
	return cluster
}

func TestDN(t *testing.T) {
	cert, _, expected, err := certutil.GenerateTestCert()
	if err != nil {
		t.Fatal("failed to generate certificate for testing:", err)
	}
	userCert := &UserCertificate{
		Certificate: cert,
	}
	dn := userCert.DN()
	if dn != expected {
		t.Error("Expected:", expected, "got:", dn)
	}
}

func TestGetCommonName(t *testing.T) {
	cluster := &v1beta1.KafkaCluster{}
	cluster.Name = "test-cluster"
	cluster.Namespace = "test-namespace"

	cluster.Spec = v1beta1.KafkaClusterSpec{HeadlessServiceEnabled: true}
	headlessCN := GetCommonName(cluster)
	expected := "test-cluster-headless.test-namespace.svc.cluster.local"
	if headlessCN != expected {
		t.Error("Expected:", expected, "Got:", headlessCN)
	}

	cluster.Spec = v1beta1.KafkaClusterSpec{HeadlessServiceEnabled: false}
	allBrokerCN := GetCommonName(cluster)
	expected = "test-cluster-all-broker.test-namespace.svc.cluster.local"
	if allBrokerCN != expected {
		t.Error("Expected:", expected, "Got:", allBrokerCN)
	}

	cluster.Spec = v1beta1.KafkaClusterSpec{HeadlessServiceEnabled: true, KubernetesClusterDomain: "foo.bar"}
	kubernetesClusterDomainCN := GetCommonName(cluster)
	expected = "test-cluster-headless.test-namespace.svc.foo.bar"
	if kubernetesClusterDomainCN != expected {
		t.Error("Expected:", expected, "Got:", kubernetesClusterDomainCN)
	}
}

func TestLabelsForKafkaPKI(t *testing.T) {
	expected := map[string]string{
		"app":          "kafka",
		"kafka_issuer": fmt.Sprintf(BrokerClusterIssuerTemplate, "kafka", "test"),
	}
	got := LabelsForKafkaPKI("test", "kafka")
	if !reflect.DeepEqual(got, expected) {
		t.Error("Expected:", expected, "got:", got)
	}
}

func TestGetInternalDNSNames(t *testing.T) {
	cluster := testCluster(t)
	cluster.Spec.Brokers = []v1beta1.Broker{
		{
			Id: 0,
		},
	}

	cluster.Spec.HeadlessServiceEnabled = true
	headlessNames := GetInternalDNSNames(cluster)
	expected := []string{
		"*.test-cluster-headless.test-namespace.svc.cluster.local",
		"test-cluster-headless.test-namespace.svc.cluster.local",
		"*.test-cluster-headless.test-namespace.svc",
		"test-cluster-headless.test-namespace.svc",
		"*.test-cluster-headless.test-namespace",
		"test-cluster-headless.test-namespace",
		"test-cluster-headless",
	}
	if !reflect.DeepEqual(expected, headlessNames) {
		t.Error("Expected:", expected, "got:", headlessNames)
	}

	cluster.Spec.HeadlessServiceEnabled = false
	allBrokerNames := GetInternalDNSNames(cluster)
	expected = []string{
		"*.test-cluster-all-broker.test-namespace.svc.cluster.local",
		"test-cluster-all-broker.test-namespace.svc.cluster.local",
		"*.test-cluster-all-broker.test-namespace.svc",
		"test-cluster-all-broker.test-namespace.svc",
		"test-cluster-0.test-namespace.svc.cluster.local",
		"test-cluster-0.test-namespace.svc",
		"*.test-cluster-0.test-namespace",
		"*.test-cluster-all-broker.test-namespace",
		"test-cluster-all-broker.test-namespace",
		"test-cluster-all-broker",
	}
	if !reflect.DeepEqual(expected, allBrokerNames) {
		t.Error("Expected:", expected, "got:", allBrokerNames)
	}
}

func TestBrokerUserForCluster(t *testing.T) {
	cluster := testCluster(t)
	user := BrokerUserForCluster(cluster, make(map[string]v1beta1.ListenerStatusList))

	expected := &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(GetCommonName(cluster),
			LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster),
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: fmt.Sprintf(BrokerServerCertTemplate, cluster.Name),
			DNSNames:   GetInternalDNSNames(cluster),
			IncludeJKS: true,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}

	if !reflect.DeepEqual(user, expected) {
		t.Errorf("Expected %+v\nGot %+v", expected, user)
	}
}

func TestControllerUserForCluster(t *testing.T) {
	cluster := testCluster(t)
	user := ControllerUserForCluster(cluster)

	expected := &v1alpha1.KafkaUser{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(BrokerControllerFQDNTemplate, fmt.Sprintf(BrokerControllerTemplate, cluster.Name),
				cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain()),
			LabelsForKafkaPKI(cluster.Name, cluster.Namespace), cluster,
		),
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: fmt.Sprintf(BrokerControllerTemplate, cluster.Name),
			IncludeJKS: true,
			ClusterRef: v1alpha1.ClusterReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		},
	}

	if !reflect.DeepEqual(user, expected) {
		t.Errorf("Expected %+v\nGot %+v", expected, user)
	}
}

func TestTruncatedCommonName(t *testing.T) {
	testCases := []struct {
		testName           string
		commonName         string
		commonNameLenLimit int
		expected           string
	}{
		{
			testName:           "Common name with length exceeded limitation",
			commonName:         "kafka-test-controller-exceeded-length.test-namespace.mgt.cluster.local",
			commonNameLenLimit: MaxCertManagerCNLen,
			expected:           "kafka-test-controller-exceeded-length.test-namespace.mgt.cluster",
		},
		{
			testName:           "Common name with length within limitation",
			commonName:         "kafka-test-controller.test-namespace.mgt.cluster.local",
			commonNameLenLimit: MaxCertManagerCNLen,
			expected:           "kafka-test-controller.test-namespace.mgt.cluster.local",
		},
	}

	t.Parallel()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			get := TruncatedCommonName(test.commonName, test.commonNameLenLimit)
			if get != test.expected {
				t.Errorf("Expected CN name after truncation:%s, got:%s", test.expected, get)
			}
		})
	}
}
