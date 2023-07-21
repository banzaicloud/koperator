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
	"github.com/banzaicloud/koperator/tests/e2e/pkg/tests"
	"github.com/gruntwork-io/terratest/modules/k8s"
)

var alltestCase = tests.TestCase{
	TestName: "Testing e2e test altogether",
	TestFn:   allTestCase,
}

// TODO (marbarta): kubectlOptions should be passed for the subtests
func allTestCase(kubectlOptions k8s.KubectlOptions) {
	var snapshottedInfo = &clusterSnapshot{}
	snapshotCluster(snapshottedInfo)
	testInstall()
	testInstallZookeeperCluster()
	testInstallKafkaCluster("../../config/samples/simplekafkacluster.yaml")
	testUninstallKafkaCluster()
	testInstallKafkaCluster("../../config/samples/simplekafkacluster_ssl.yaml")
	testUninstallKafkaCluster()
	testUninstallZookeeperCluster()
	testUninstall()
	snapshotClusterAndCompare(snapshottedInfo)
}
