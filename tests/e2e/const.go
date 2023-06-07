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

// HelmDescriptors.
var (
	// certManagerHelmDescriptor describes the cert-manager Helm component.
	certManagerHelmDescriptor = helmDescriptor{
		Repository:                   "https://charts.jetstack.io",
		ChartName:                    "cert-manager",
		ChartVersion:                 "v1.11.0",
		ReleaseName:                  "cert-manager",
		Namespace:                    "cert-manager",
		RemoteCRDPathVersionTemplate: "https://github.com/jetstack/cert-manager/releases/download/v%s/cert-manager.crds.yaml",
	}

	// koperatorLocalHelmDescriptor describes the Koperator Helm component with
	// a local chart and version.
	koperatorLocalHelmDescriptor = helmDescriptor{
		Repository:       "../../charts/kafka-operator",
		ChartVersion:     LocalVersion,
		ReleaseName:      "kafka-operator",
		Namespace:        "kafka",
		LocalCRDSubpaths: []string{"templates/crds.yaml"},
		LocalCRDTemplateRenderValues: map[string]string{
			"crd.enabled": "true",
		},
	}

	// koperatorLocalHelmDescriptor describes the Koperator Helm component with
	// a remote latest chart and version.
	koperatorRemoteLatestHelmDescriptor = helmDescriptor{ //nolint:unused // Note: intentional possibly needed in the future for upgrade test.
		Repository:                   "https://kubernetes-charts.banzaicloud.com",
		ChartName:                    "kafka-operator",
		ChartVersion:                 "", // Note: empty string translates to latest final version.
		ReleaseName:                  "kafka-operator",
		Namespace:                    "kafka",
		RemoteCRDPathVersionTemplate: "https://github.com/banzaicloud/koperator/releases/download/%s/kafka-operator.crds.yaml",
	}

	// prometheusOperatorHelmDescriptor describes the prometheus-operator Helm
	// component.
	prometheusOperatorHelmDescriptor = helmDescriptor{
		Repository:   "https://prometheus-community.github.io/helm-charts",
		ChartName:    "kube-prometheus-stack",
		ChartVersion: "42.0.1",
		ReleaseName:  "prometheus-operator",
		Namespace:    "prometheus",
		SetValues: map[string]string{
			"prometheusOperator.createCustomResource": "true",
			"defaultRules.enabled":                    "false",
			"alertmanager.enabled":                    "false",
			"grafana.enabled":                         "false",
			"kubeApiServer.enabled":                   "false",
			"kubelet.enabled":                         "false",
			"kubeControllerManager.enabled":           "false",
			"coreDNS.enabled":                         "false",
			"kubeEtcd.enabled":                        "false",
			"kubeScheduler.enabled":                   "false",
			"kubeProxy.enabled":                       "false",
			"kubeStateMetrics.enabled":                "false",
			"nodeExporter.enabled":                    "false",
			"prometheus.enabled":                      "false",
		},
	}

	// zookeeperOperatorHelmDescriptor describes the zookeeper-operator Helm
	// component.
	zookeeperOperatorHelmDescriptor = helmDescriptor{
		Repository:   "https://charts.pravega.io",
		ChartName:    "zookeeper-operator",
		ChartVersion: "0.2.14",
		ReleaseName:  "zookeeper-operator",
		Namespace:    "zookeeper",
	}
)

// Versions.
type Version = string

const (
	// LocalVersion means using the files in the local repository snapshot.
	LocalVersion Version = "local"
)
