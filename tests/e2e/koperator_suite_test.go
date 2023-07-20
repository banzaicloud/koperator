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

//go:build e2e

package e2e

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKoperator(t *testing.T) {
	RegisterFailHandler(Fail) // Note: Ginkgo - Gomega connector.
	RunSpecs(t, "Koperator end to end test suite")
}

var _ = BeforeSuite(func() {
	By("Acquiring K8s cluster")
	var kubeconfigPath string
	var kubecontextName string

	By("Acquiring K8s config and context", func() {
		var err error
		kubeconfigPath, kubecontextName, err = currentEnvK8sContext()
		Expect(err).NotTo(HaveOccurred())
	})

	By("Listing kube-system pods", func() {
		pods := k8s.ListPods(
			ginkgo.GinkgoT(),
			k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "kube-system"),
			v1.ListOptions{},
		)

		Expect(len(pods)).To(Not(BeZero()))
	})
})

var _ = When("Testing e2e test altogether", Ordered, func() {
	var snapshottedInfo = &clusterSnapshot{}
	snapshotCluster(snapshottedInfo)
	testInstall()
	testInstallZookeeperCluster()
	testInstallKafkaCluster("../../config/samples/simplekafkacluster.yaml")
	testProduceConsumeInternal()
	testUninstallKafkaCluster()
	testInstallKafkaCluster("../../config/samples/simplekafkacluster_ssl.yaml")
	testProduceConsumeInternalSSL(defaultTLSSecretName)
	testUninstallKafkaCluster()
	testUninstallZookeeperCluster()
	testUninstall()
	snapshotClusterAndCompare(snapshottedInfo)
})
