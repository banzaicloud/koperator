// Copyright © 2019 Banzai Cloud
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

package certmanagerpki

import (
	"flag"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util/pki"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const spiffeIdTemplate = "spiffe://%s/%s"

var namespaceCertManager string

func init() {
	flag.StringVar(&namespaceCertManager, "cert-manager-namespace", "cert-manager", "The namespace where cert-manager is running")
}

type CertManager interface {
	pki.Manager
}

// certManager implements a PKIManager using cert-manager as the backend
type certManager struct {
	client  client.Client
	cluster *v1beta1.KafkaCluster
}

func New(client client.Client, cluster *v1beta1.KafkaCluster) CertManager {
	return &certManager{client: client, cluster: cluster}
}
