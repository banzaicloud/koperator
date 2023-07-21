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

package common

import (
	"os"
	"path"

	"emperror.dev/errors"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"gopkg.in/yaml.v2"
)

// currentKubernetesContext returns the currently set Kubernetes context based
// on the the environment variables and the KUBECONFIG file.
func CurrentEnvK8sContext() (kubeconfigPath string, kubecontextName string, err error) {
	kubeconfigPath, isExisting := os.LookupEnv("KUBECONFIG")
	if !isExisting {
		homePath, err := os.UserHomeDir()
		if err != nil {
			return "", "", errors.WrapIf(err, "retrieving user home directory failed")
		}

		kubeconfigPath = path.Join(homePath, ".kube", "config")
	}

	kubecontext, err := GetDefaultKubeContext(kubeconfigPath)
	if err != nil {
		return "", "", err
	}

	return kubeconfigPath, kubecontext, nil
}

// kubectlOptionsForCurrentContext returns a kubectlOptions object for the
// current Kubernetes context or alternatively an error.
func KubectlOptionsForCurrentContext() (k8s.KubectlOptions, error) {
	kubeconfigPath, kubecontextName, err := CurrentEnvK8sContext()
	if err != nil {
		return k8s.KubectlOptions{}, errors.WrapIf(err, "retrieving current environment Kubernetes context failed")
	}

	return k8s.KubectlOptions{
		ConfigPath:  kubeconfigPath,
		ContextName: kubecontextName,
		Namespace:   "",
	}, nil
}

func GetDefaultKubeContext(kubeconfigPath string) (string, error) {
	kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return "", errors.WrapIfWithDetails(err, "reading KUBECONFIG file failed", "path", kubeconfigPath)
	}

	structuredKubeconfig := make(map[string]interface{})
	err = yaml.Unmarshal(kubeconfigBytes, &structuredKubeconfig)
	if err != nil {
		return "", errors.WrapIfWithDetails(
			err,
			"parsing kubeconfig failed",
			"kubeconfig", string(kubeconfigBytes),
		)
	}

	kubecontext, isOk := structuredKubeconfig["current-context"].(string)
	if !isOk {
		return "", errors.WrapIfWithDetails(
			err,
			"kubeconfig current-context is not string",
			"current-context", structuredKubeconfig["current-context"],
		)
	}

	return kubecontext, nil
}
