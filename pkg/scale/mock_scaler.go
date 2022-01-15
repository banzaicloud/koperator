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

package scale

import (
	"context"
)

func MockNewCruiseControlScaler() {
	newCruiseControlScaler = createMockCruiseControlScaler
}

func createMockCruiseControlScaler(_ context.Context, _ string) (CruiseControlScaler, error) {
	return &mockCruiseControlScaler{}, nil
}

type mockCruiseControlScaler struct{}

func (mc *mockCruiseControlScaler) IsReady() bool {
	return true
}

func (mc *mockCruiseControlScaler) Status() CruiseControlStatus {
	return CruiseControlStatus{}
}

func (mc *mockCruiseControlScaler) GetUserTasks(taskIDs ...string) ([]*Result, error) {
	return []*Result{}, nil
}

func (mc *mockCruiseControlScaler) IsUp() bool {
	return true
}

func (mc *mockCruiseControlScaler) AddBrokers(brokerIDs ...string) (*Result, error) {
	return &Result{}, nil
}

func (mc *mockCruiseControlScaler) RemoveBrokers(brokerIDs ...string) (*Result, error) {
	return &Result{}, nil
}

func (mc *mockCruiseControlScaler) RebalanceDisks(brokerIDs ...string) (*Result, error) {
	return &Result{}, nil
}

func (mc *mockCruiseControlScaler) BrokersWithState(states ...KafkaBrokerState) ([]string, error) {
	return []string{}, nil
}

func (mc *mockCruiseControlScaler) PartitionReplicasByBroker() (map[string]int32, error) {
	return map[string]int32{}, nil
}

func (mc *mockCruiseControlScaler) BrokerWithLeastPartitionReplicas() (string, error) {
	return "", nil
}

func (mc *mockCruiseControlScaler) LogDirsByBroker() (map[string]map[LogDirState][]string, error) {
	return make(map[string]map[LogDirState][]string), nil
}
