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
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
)

// applyKoperatorSampleResource deploys the specified sample resource (config/samples).
// The full path of the manifest also can be specified.
// It supports different versions that can be specified with the koperatorVersion parameter.
func applyKoperatorSampleResource(kubectlOptions k8s.KubectlOptions, koperatorVersion Version, sampleFile string) error {
	By(fmt.Sprintf("Retrieving Koperator sample resource: '%s' with version: '%s' ", sampleFile, koperatorVersion))

	sampleFileSplit := strings.Split(sampleFile, "/")
	if len(sampleFileSplit) == 0 {
		return errors.Errorf("sample file path shouldn't be empty")
	}

	var err error
	var rawKoperatorSampleResource []byte

	switch koperatorVersion {
	case LocalVersion:
		if len(sampleFileSplit) == 1 {
			sampleFile = fmt.Sprintf("../../config/samples/%s", sampleFile)
		}

		rawKoperatorSampleResource, err = os.ReadFile(sampleFile)
		if err != nil {
			return err
		}
	default:
		httpClient := new(http.Client)
		httpClient.Timeout = 5 * time.Second

		if len(sampleFileSplit) != 1 {
			return errors.Errorf("sample file path shouldn't contain a \"/\" character")
		}

		response, err := httpClient.Get("https://raw.githubusercontent.com/banzaicloud/koperator/" + koperatorVersion + "/config/samples/" + sampleFile)
		if response != nil {
			defer func() { _ = response.Body.Close() }()
		}
		if err != nil {
			return err
		}

		rawKoperatorSampleResource, err = io.ReadAll(response.Body)
		if err != nil {
			return err
		}
	}

	By(fmt.Sprintf("Applying K8s manifest %s", sampleFile))
	k8s.KubectlApplyFromString(GinkgoT(), &kubectlOptions, string(rawKoperatorSampleResource))
	return nil
}
