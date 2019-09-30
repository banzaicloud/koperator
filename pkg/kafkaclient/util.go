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
	"strconv"

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
)

// TopicMetaToStatus takes the output of KafkaClient.Brokers() and the output
// of KafkaClient.DescribeTopic() and converts it to a v1alpha1.KafkaTopicStatus
func TopicMetaToStatus(brokers map[int32]string, meta *sarama.TopicMetadata) *v1alpha1.KafkaTopicStatus {
	status := newKafkaTopicStatus()
	status.PartitionCount = int32(len(meta.Partitions))
	for _, partMeta := range meta.Partitions {
		status = appendPartMetaToStatus(status, brokers, partMeta)
	}
	return status
}

func appendPartMetaToStatus(status *v1alpha1.KafkaTopicStatus, brokers map[int32]string, partMeta *sarama.PartitionMetadata) *v1alpha1.KafkaTopicStatus {
	ID := strconv.Itoa(int(partMeta.ID))
	status.Leaders[ID] = brokerToIDAddrString(brokers, partMeta.Leader)
	status.ReplicaCounts[ID] = len(partMeta.Replicas)
	status.InSyncReplicas[ID] = parseSyncReplicas(brokers, partMeta.Isr)
	status.OfflineReplicas[ID] = parseSyncReplicas(brokers, partMeta.OfflineReplicas)
	return status
}

func newKafkaTopicStatus() *v1alpha1.KafkaTopicStatus {
	status := &v1alpha1.KafkaTopicStatus{}
	status.Leaders = make(map[string]string, 0)
	status.ReplicaCounts = make(map[string]int, 0)
	status.InSyncReplicas = make(map[string]v1alpha1.SyncReplicas, 0)
	status.OfflineReplicas = make(map[string]v1alpha1.SyncReplicas, 0)
	return status
}

func parseSyncReplicas(brokers map[int32]string, replicas []int32) v1alpha1.SyncReplicas {
	out := make(v1alpha1.SyncReplicas, 0)
	if replicas == nil {
		return out
	}
	for _, replica := range replicas {
		out = append(out, brokerToIDAddrString(brokers, replica))
	}
	return out
}

func brokerToIDAddrString(brokers map[int32]string, brokerId int32) string {
	var addr string
	var ok bool
	if addr, ok = brokers[brokerId]; !ok {
		addr = "unknown"
	}
	return fmt.Sprintf("%s/%s", strconv.Itoa(int(brokerId)), addr)
}
