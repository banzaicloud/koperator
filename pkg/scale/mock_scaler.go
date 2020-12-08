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
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

type mockCruiseControlScaler struct {
	CruiseControlScaler
	namespace               string
	kubernetesClusterDomain string
	endpoint                string
	clusterName             string
}

func (mc *mockCruiseControlScaler) GetLiveKafkaBrokersFromCruiseControl(brokerIds []string) ([]string, error) {
	return nil, nil
}

func (mc *mockCruiseControlScaler) GetBrokerIDWithLeastPartition() (string, error) {
	return "", nil
}

func (mc *mockCruiseControlScaler) UpScaleCluster(brokerIds []string) (string, string, error) {
	return "", "", nil
}

func (mc *mockCruiseControlScaler) DownsizeCluster(brokerIds []string) (string, string, error) {
	return "", "", nil
}

func (mc *mockCruiseControlScaler) RebalanceDisks(brokerIdsWithMountPath map[string][]string) (string, string, error) {
	return "", "", nil
}

func (mc *mockCruiseControlScaler) RebalanceCluster() (string, error) {
	return "", nil
}

func (mc *mockCruiseControlScaler) RunPreferedLeaderElectionInCluster() (string, error) {
	return "", nil
}

func (mc *mockCruiseControlScaler) KillCCTask() error {
	return nil
}

func (mc *mockCruiseControlScaler) GetCCTaskState(uTaskId string) (v1beta1.CruiseControlUserTaskState, error) {
	return "", nil
}
