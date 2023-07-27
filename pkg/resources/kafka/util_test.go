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
	"reflect"
	"testing"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func TestGenerateClusterID(t *testing.T) {
	// one random cluster ID serves for the entire Kafka cluster, therefore testing 100000 cluster IDs should be enough
	numOfIDs := 100000
	test := make(map[string]bool, numOfIDs)
	for i := 0; i < numOfIDs; i++ {
		clusterID := generateRandomClusterID()
		_, err := base64.URLEncoding.DecodeString(clusterID)
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}

		if test[clusterID] {
			t.Error("expected random cluster ID that does not collide with previous ones")
		}

		// mark the map to note that this cluster ID has been generated
		test[clusterID] = true
	}
}

//nolint:funlen
func TestGenerateQuorumVoters(t *testing.T) {
	kafkaCluster := &v1beta1.KafkaCluster{}

	tests := []struct {
		testName             string
		brokers              []v1beta1.Broker
		listenersStatuses    map[string]v1beta1.ListenerStatusList
		expectedQuorumVoters []string
	}{
		{
			testName: "brokers with ascending order by IDs; controller listener statuses has the same order as brokers",
			brokers: []v1beta1.Broker{
				{
					Id: int32(0),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(10),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(20),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(30),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(40),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(50),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(60),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
			},
			listenersStatuses: map[string]v1beta1.ListenerStatusList{
				"test-listener": {
					{
						Name:    "broker-0",
						Address: "fakeKafka-0.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-10",
						Address: "fakeKafka-10.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-20",
						Address: "fakeKafka-20.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-30",
						Address: "fakeKafka-30.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-40",
						Address: "fakeKafka-40.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-50",
						Address: "fakeKafka-50.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-60",
						Address: "fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093",
					},
				},
			},
			expectedQuorumVoters: []string{
				"40@fakeKafka-40.fakeKafka-headless.default.svc.cluster.local:29093",
				"50@fakeKafka-50.fakeKafka-headless.default.svc.cluster.local:29093",
				"60@fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093"},
		},
		{
			testName: "brokers with descending order by IDs; controller listener statuses has the same order as brokers",
			brokers: []v1beta1.Broker{
				{
					Id: int32(60),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(50),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(40),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(30),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(20),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(10),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(0),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
			},
			listenersStatuses: map[string]v1beta1.ListenerStatusList{
				"test-listener": {
					{
						Name:    "broker-60",
						Address: "fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-50",
						Address: "fakeKafka-50.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-40",
						Address: "fakeKafka-40.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-30",
						Address: "fakeKafka-30.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-20",
						Address: "fakeKafka-20.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-10",
						Address: "fakeKafka-10.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-0",
						Address: "fakeKafka-0.fakeKafka-headless.default.svc.cluster.local:29093",
					},
				},
			},
			expectedQuorumVoters: []string{
				"0@fakeKafka-0.fakeKafka-headless.default.svc.cluster.local:29093",
				"10@fakeKafka-10.fakeKafka-headless.default.svc.cluster.local:29093",
				"20@fakeKafka-20.fakeKafka-headless.default.svc.cluster.local:29093"},
		},
		{
			testName: "brokers with ascending order by IDs; controller listener statuses has the opposite order as brokers",
			brokers: []v1beta1.Broker{
				{
					Id: int32(0),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(10),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(20),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(30),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(40),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(50),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(60),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
			},
			listenersStatuses: map[string]v1beta1.ListenerStatusList{
				"test-listener": {
					{
						Name:    "broker-60",
						Address: "fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-50",
						Address: "fakeKafka-50.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-40",
						Address: "fakeKafka-40.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-30",
						Address: "fakeKafka-30.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-20",
						Address: "fakeKafka-20.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-10",
						Address: "fakeKafka-10.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-0",
						Address: "fakeKafka-0.fakeKafka-headless.default.svc.cluster.local:29093",
					},
				},
			},
			expectedQuorumVoters: []string{
				"40@fakeKafka-40.fakeKafka-headless.default.svc.cluster.local:29093",
				"50@fakeKafka-50.fakeKafka-headless.default.svc.cluster.local:29093",
				"60@fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093"},
		},
		{
			testName: "brokers and controller listener statuses with random order",
			brokers: []v1beta1.Broker{
				{
					Id: int32(100),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker", "controller"},
					},
				},
				{
					Id: int32(50),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(80),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(30),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(90),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
				{
					Id: int32(40),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"broker"},
					},
				},
				{
					Id: int32(60),
					BrokerConfig: &v1beta1.BrokerConfig{
						Roles: []string{"controller"},
					},
				},
			},
			listenersStatuses: map[string]v1beta1.ListenerStatusList{
				"test-listener": {
					{
						Name:    "broker-30",
						Address: "fakeKafka-30.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-50",
						Address: "fakeKafka-50.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-60",
						Address: "fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-100",
						Address: "fakeKafka-100.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-80",
						Address: "fakeKafka-80.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-90",
						Address: "fakeKafka-90.fakeKafka-headless.default.svc.cluster.local:29093",
					},
					{
						Name:    "broker-40",
						Address: "fakeKafka-40.fakeKafka-headless.default.svc.cluster.local:29093",
					},
				},
			},
			expectedQuorumVoters: []string{
				"60@fakeKafka-60.fakeKafka-headless.default.svc.cluster.local:29093",
				"80@fakeKafka-80.fakeKafka-headless.default.svc.cluster.local:29093",
				"90@fakeKafka-90.fakeKafka-headless.default.svc.cluster.local:29093",
				"100@fakeKafka-100.fakeKafka-headless.default.svc.cluster.local:29093",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			kafkaCluster.Spec.Brokers = test.brokers
			gotQuorumVoters, err := generateQuorumVoters(kafkaCluster, test.listenersStatuses)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(gotQuorumVoters, test.expectedQuorumVoters) {
				t.Error("Expected:", test.expectedQuorumVoters, "Got:", gotQuorumVoters)
			}
		})
	}
}
