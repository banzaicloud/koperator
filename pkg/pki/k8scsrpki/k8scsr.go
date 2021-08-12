// Copyright © 2021 Banzai Cloud
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
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util/pki"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DependingCsrAnnotation string = "banzaicloud.io/csr"

type K8sCSR interface {
	pki.Manager
}

// k8sCSR implements a PKIManager using kubernetes csr api as the backend
type k8sCSR struct {
	client  client.Client
	cluster *v1beta1.KafkaCluster
	logger  logr.Logger
}

func New(client client.Client, cluster *v1beta1.KafkaCluster, logger logr.Logger) K8sCSR {
	return &k8sCSR{client: client, cluster: cluster, logger: logger}
}
