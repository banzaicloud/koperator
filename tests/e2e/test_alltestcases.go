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
	"time"

	"github.com/banzaicloud/koperator/tests/e2e/pkg/tests"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/onsi/gomega/format"
)

var alltestCase = tests.TestCase{
	TestDuration: 20 * time.Minute,
	TestName:     "ALL_TESTCASE",
	TestFn:       allTestCase,
}

// TODO (marbarta): kubectlOptions should be passed for the subtests
func allTestCase(kubectlOptions k8s.KubectlOptions) {
	format.MaxLength = 0
	var snapshottedInfo = &clusterSnapshot{}
	snapshotCluster(snapshottedInfo)
	testInstall(kubectlOptions)
	testInstallZookeeperCluster(kubectlOptions)
	testInstallKafkaCluster(kubectlOptions, "../../config/samples/simplekafkacluster.yaml")
	//testProduceConsumeExternal(kubectlOptions, "")
	testProduceConsumeInternal(kubectlOptions)
	testUninstallKafkaCluster(kubectlOptions)
	testInstallKafkaCluster(kubectlOptions, "../../config/samples/simplekafkacluster_ssl.yaml")
	//testProduceConsumeExternal(kubectlOptions, "")
	//testProduceConsumeInternal(kubectlOptions)
	testProduceConsumeInternalSSL(kubectlOptions, defaultTLSSecretName)
	testUninstallKafkaCluster(kubectlOptions)
	testUninstallZookeeperCluster(kubectlOptions)
	testUninstall(kubectlOptions)
	snapshotClusterAndCompare(snapshottedInfo)
}
