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
	"fmt"

	"emperror.dev/errors"
)

func (k *kafkaClient) OfflineReplicaCount() (int, error) {
	availableTopics, err := k.client.Topics()
	if err != nil {
		return 0, errors.WrapIf(err, "could not fetch topics")
	}
	offlineReplicaCount := 0
	for _, topic := range availableTopics {
		partitions, err := k.client.Partitions(topic)
		if err != nil {
			return 0, errors.WrapIfWithDetails(err, "could not fetch partition", "topic", topic)
		}
		for _, partition := range partitions {
			offlineReplicas, err := k.client.OfflineReplicas(topic, partition)
			if err != nil {
				return 0, errors.WrapIfWithDetails(err, "could not fetch offline replicas", "topic", topic, "partition", partition)
			}
			offlineReplicaCount = offlineReplicaCount + len(offlineReplicas)
		}
	}
	log.Info(fmt.Sprintf("offline Replica Count is %d", offlineReplicaCount))
	return offlineReplicaCount, nil
}

func (k *kafkaClient) AllReplicaInSync() (bool, error) {
	availableTopics, err := k.client.Topics()
	if err != nil {
		return false, errors.WrapIf(err, "could not fetch topics")
	}
	for _, topic := range availableTopics {
		partitions, err := k.client.Partitions(topic)
		if err != nil {
			return false, errors.WrapIfWithDetails(err, "could not fetch partition", "topic", topic)
		}
		for _, partition := range partitions {
			replicas, err := k.client.Replicas(topic, partition)
			if err != nil {
				return false, errors.WrapIfWithDetails(err, "could not fetch replicas", "topic", topic, "partition", partition)
			}
			isrReplicas, err := k.client.InSyncReplicas(topic, partition)
			if err != nil {
				return false, errors.WrapIfWithDetails(err, "could not fetch isr replicas", "topic", topic, "partition", partition)
			}
			if len(replicas) != len(isrReplicas) {
				log.Info("not all replicas are in sync")
				return false, nil
			}
		}
	}
	log.Info("all replicas are in sync")
	return true, nil
}
