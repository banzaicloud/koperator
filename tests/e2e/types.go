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

	"github.com/gruntwork-io/terratest/modules/k8s"
)

type dependencyCRDsType struct {
	zookeeper   []string
	prometheus  []string
	certManager []string
}

func (c *dependencyCRDsType) Zookeeper() []string {
	return c.zookeeper
}
func (c *dependencyCRDsType) Prometheus() []string {
	return c.prometheus
}
func (c *dependencyCRDsType) CertManager() []string {
	return c.certManager
}

func (c *dependencyCRDsType) Initialize(kubectlOptions k8s.KubectlOptions) error {
	var err error
	c.certManager, err = listK8sResourceKinds(kubectlOptions, apiGroupKoperatorDependencies()["cert-manager"])
	if err != nil {
		return fmt.Errorf("initialize Cert-manager CRDs error: %w", err)
	}
	c.prometheus, err = listK8sResourceKinds(kubectlOptions, apiGroupKoperatorDependencies()["prometheus"])
	if err != nil {
		return fmt.Errorf("initialize Prometheus CRDs error: %w", err)
	}
	c.zookeeper, err = listK8sResourceKinds(kubectlOptions, apiGroupKoperatorDependencies()["zookeeper"])
	if err != nil {
		return fmt.Errorf("initialize Zookeeper CRDs error: %w", err)
	}
	return nil
}
