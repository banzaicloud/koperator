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
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = When("Installing Koperator", Ordered, func() {
	var kubeconfigPath string
	var kubecontextName string

	It("Acquiring K8s config and context", func() {
		var err error
		kubeconfigPath, kubecontextName, err = currentEnvK8sContext()

		Expect(err).NotTo(HaveOccurred())
	})

	// requireInstallingCertManager(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "cert-manager"), "v1.11.0")
	// requireInstallingZookeeperOperator(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "zookeeper"), "0.2.14")
	// requireInstallingPrometheusOperator(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "prometheus"), "42.0.1")
	//requireInstallingKoperator(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "kafka"), LocalVersion)
	// // -----------------------------------------------------------------------------------------------------------
	requireUninstallKafkaCluster(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "kafka"))
	// // -----------------------------------------------------------------------------------------------------------
	//requireUninstallingKoperator(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "kafka"))
	//requireUninstallingPrometheusOperator(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "prometheus"))
	// requireUninstallingZookeeperOperator(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "zookeeper"))
	// requireUninstallingCertManager(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "cert-manager"))
	//requireInternalProducerConsumer(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "kafka"))
	// requireExternalProducerConsumer(k8s.NewKubectlOptions(kubecontextName, kubeconfigPath, "kafka"))
})
