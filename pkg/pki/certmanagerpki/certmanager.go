// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/pki"
)

const spiffeIdTemplate = "spiffe://%s/ns/%s/kafkauser/%s"

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
