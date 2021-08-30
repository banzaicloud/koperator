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

package vaultpki

import (
	vaultapi "github.com/hashicorp/vault/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/pki"
)

// VaultPKI implements a PKIManager using vault as the backend
type VaultPKI interface {
	pki.Manager
}

type vaultPKI struct {
	client  client.Client
	cluster *v1beta1.KafkaCluster

	// Set as attribute for mocking
	getClient func() (*vaultapi.Client, error)
}

func New(client client.Client, cluster *v1beta1.KafkaCluster) VaultPKI {
	return &vaultPKI{
		client:  client,
		cluster: cluster,
		getClient: func() (*vaultapi.Client, error) {
			return getVaultClient(cluster.GetUID(), cluster.Spec.VaultConfig.AuthRole)
		},
	}
}
