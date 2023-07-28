// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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

package k8scsrpki

import (
	"flag"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/pki"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DependingCsrAnnotation     string = "banzaicloud.io/csr"
	IncludeFullChainAnnotation string = "csr.banzaicloud.io/fullchain"
)

var namespaceCertManager string

func init() {
	flag.StringVar(&namespaceCertManager, "cert-manager-namespace", "cert-manager", "The namespace where cert-manager is running")
}

type K8sCSR interface {
	pki.Manager
}

// k8sCSR implements a PKIManager using kubernetes csr api as the backend
type k8sCSR struct {
	client  client.Client
	cluster *v1beta1.KafkaCluster
}

func New(client client.Client, cluster *v1beta1.KafkaCluster) K8sCSR {
	return &k8sCSR{client: client, cluster: cluster}
}
