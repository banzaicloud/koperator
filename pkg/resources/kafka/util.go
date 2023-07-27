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

package kafka

import (
	"encoding/base64"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

// generateQuorumVoters generates the quorum voters in the format of brokerID@nodeAddress:listenerPort
// The generated quorum voters are guaranteed in ascending order by broker IDs to ensure same quorum voters configurations are returned
// regardless of the order of brokers and controllerListenerStatuses are passed in - this is needed to avoid triggering
// unnecessary rolling upgrade operations
func generateQuorumVoters(kafkaCluster *v1beta1.KafkaCluster, controllerListenerStatuses map[string]v1beta1.ListenerStatusList) ([]string, error) {
	var (
		quorumVoters []string
		brokerIDs    []int32
	)
	idToListenerAddrMap := make(map[int32]string)

	// find the controller nodes and their corresponding listener addresses
	for _, b := range kafkaCluster.Spec.Brokers {
		brokerConfig, err := b.GetBrokerConfig(kafkaCluster.Spec)
		if err != nil {
			return nil, err
		}

		if brokerConfig.IsControllerNode() {
			for _, controllerListenerStatus := range controllerListenerStatuses {
				for _, status := range controllerListenerStatus {
					if status.Name == fmt.Sprintf("broker-%d", b.Id) {
						idToListenerAddrMap[b.Id] = status.Address
						brokerIDs = append(brokerIDs, b.Id)
						break
					}
				}
			}
		}
	}

	sort.Slice(brokerIDs, func(i, j int) bool {
		return brokerIDs[i] < brokerIDs[j]
	})

	for _, brokerId := range brokerIDs {
		quorumVoters = append(quorumVoters, fmt.Sprintf("%d@%s", brokerId, idToListenerAddrMap[brokerId]))
	}

	return quorumVoters, nil
}

// generateRandomClusterID() generates a based64-encoded random UUID with 16 bytes as the cluster ID
// it uses URL based64 encoding since that's what Kafka expects
func generateRandomClusterID() string {
	randomUUID := uuid.New()
	return base64.URLEncoding.EncodeToString(randomUUID[:])
}
