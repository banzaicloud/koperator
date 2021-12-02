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

package kafkaclient

import (
	"emperror.dev/errors"
)

func (k *kafkaClient) AllOfflineReplicas() ([]int32, error) {
	availableTopics, err := k.client.Topics()
	if err != nil {
		return nil, errors.WrapIf(err, "could not fetch topics")
	}
	allOfflineReplicas := make(map[int32]struct{})
	for _, topic := range availableTopics {
		partitions, err := k.client.Partitions(topic)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "could not fetch partition", "topic", topic)
		}
		for _, partition := range partitions {
			offlineReplicas, err := k.client.OfflineReplicas(topic, partition)
			if err != nil {
				return nil, errors.WrapIfWithDetails(err, "could not fetch offline replicas", "topic", topic, "partition", partition)
			}
			for _, brokerID := range offlineReplicas {
				allOfflineReplicas[brokerID] = struct{}{}
			}
		}
	}

	brokerIDs := make([]int32, 0, len(allOfflineReplicas))
	for brokerID := range allOfflineReplicas {
		brokerIDs = append(brokerIDs, brokerID)
	}

	return brokerIDs, nil
}

func (k *kafkaClient) OutOfSyncReplicas() ([]int32, error) {
	availableTopics, err := k.client.Topics()
	if err != nil {
		return nil, errors.WrapIf(err, "could not fetch topics")
	}
	outOfSyncReplicas := make(map[int32]struct{})
	for _, topic := range availableTopics {
		partitions, err := k.client.Partitions(topic)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "could not fetch partition", "topic", topic)
		}
		for _, partition := range partitions {
			replicas, err := k.client.Replicas(topic, partition)
			if err != nil {
				return nil, errors.WrapIfWithDetails(err, "could not fetch replicas", "topic", topic, "partition", partition)
			}
			isrReplicas, err := k.client.InSyncReplicas(topic, partition)
			if err != nil {
				return nil, errors.WrapIfWithDetails(err, "could not fetch isr replicas", "topic", topic, "partition", partition)
			}
			if len(replicas) != len(isrReplicas) {
				for i := range replicas {
					found := false
					for j := range isrReplicas {
						if replicas[i] == isrReplicas[j] {
							found = true
							break
						}
					}
					if !found {
						outOfSyncReplicas[replicas[i]] = struct{}{}
					}
				}
			}
		}
	}

	brokerIDs := make([]int32, 0, len(outOfSyncReplicas))
	for brokerID := range outOfSyncReplicas {
		brokerIDs = append(brokerIDs, brokerID)
	}
	return brokerIDs, nil
}
