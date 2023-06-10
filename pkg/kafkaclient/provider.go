// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

type Provider interface {
	NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, func(), error)
}

type mockProvider struct {
}

func NewMockProvider() Provider {
	return &mockProvider{}
}

func (mp *mockProvider) NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, func(), error) {
	return NewMockFromCluster(client, cluster)
}

type defaultProvider struct {
}

func NewDefaultProvider() Provider {
	return &defaultProvider{}
}

func (dp *defaultProvider) NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, func(), error) {
	return NewFromCluster(client, cluster)
}

// MockedProvider is a Testify mock for providing Kafka clients that can be mocks too
type MockedProvider struct {
	mock.Mock
}

func (m *MockedProvider) NewFromCluster(client client.Client, cluster *v1beta1.KafkaCluster) (KafkaClient, func(), error) {
	args := m.Called(client, cluster)
	return args.Get(0).(KafkaClient), args.Get(1).(func()), args.Error(2)
}
