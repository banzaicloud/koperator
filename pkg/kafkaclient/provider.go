// Copyright © 2020 Banzai Cloud
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

package kafkaclient

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

type Provider interface {
	NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, error)
}

type mockProvider struct {
}

func NewMockProvider() Provider {
	return &mockProvider{}
}

func (mp *mockProvider) NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, error) {
	return NewMockFromCluster(client, cluster)
}

type defaultProvider struct {
}

func NewDefaultProvider() Provider {
	return &defaultProvider{}
}

func (dp *defaultProvider) NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, error) {
	return NewFromCluster(client, cluster)
}
