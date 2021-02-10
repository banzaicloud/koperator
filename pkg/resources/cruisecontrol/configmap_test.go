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

package cruisecontrol

import (
	"encoding/json"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

func TestGenerateCapacityConfig(t *testing.T) {
	quantity, _ := resource.ParseQuantity("10Gi")
	cpuQuantity, _ := resource.ParseQuantity("2000m")

	testCases := []struct {
		testName              string
		kafkaCluster          v1beta1.KafkaCluster
		expectedConfiguration string
	}{
		{
			testName: "if config is set manually then use that one",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					Brokers: []v1beta1.Broker{
						{
							Id: 0,
							BrokerConfig: &v1beta1.BrokerConfig{
								Resources: &v1.ResourceRequirements{
									Limits: v1.ResourceList{
										"cpu": cpuQuantity,
									}},
							},
						},
						{
							Id: 1,
						},
						{
							Id: 2,
						},
						{
							Id: 4,
						},
					},
					CruiseControlConfig: v1beta1.CruiseControlConfig{
						CapacityConfig: `
                  {
				  "brokerCapacities":[
					{
					  "brokerId": "-1",
					  "capacity": {
						"DISK": {"/tmp/kafka-logs-1": "100000", "/tmp/kafka-logs-2": "100000", "/tmp/kafka-logs-3": "50000",
						  "/tmp/kafka-logs-4": "50000", "/tmp/kafka-logs-5": "150000", "/tmp/kafka-logs-6": "50000"},
						"CPU": "100",
						"NW_IN": "10000",
						"NW_OUT": "10000"
					  },
					  "doc": "The default capacity for a broker with multiple logDirs each on a separate heterogeneous disk."
					},
					{
					  "brokerId": "0",
					  "capacity": {
						"DISK": {"/tmp/kafka-logs": "500000"},
						"CPU": "100",
						"NW_IN": "50000",
						"NW_OUT": "50000"
					  },
					  "doc": "This overrides the capacity for broker 0. This broker is not a JBOD broker."
					},
					{
					  "brokerId": "1",
					  "capacity": {
						"DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
						"CPU": "100",
						"NW_IN": "50000",
						"NW_OUT": "50000"
					  },
					  "doc": "This overrides the capacity for broker 1. This broker is a JBOD broker."
					}
				  ]
				}`,
					},
				},
			},
			expectedConfiguration: `
                  {
				  "brokerCapacities":[
					{
					  "brokerId": "-1",
					  "capacity": {
						"DISK": {"/tmp/kafka-logs-1": "100000", "/tmp/kafka-logs-2": "100000", "/tmp/kafka-logs-3": "50000",
						  "/tmp/kafka-logs-4": "50000", "/tmp/kafka-logs-5": "150000", "/tmp/kafka-logs-6": "50000"},
						"CPU": "100",
						"NW_IN": "10000",
						"NW_OUT": "10000"
					  },
					  "doc": "The default capacity for a broker with multiple logDirs each on a separate heterogeneous disk."
					},
					{
					  "brokerId": "0",
					  "capacity": {
						"DISK": {"/tmp/kafka-logs": "500000"},
						"CPU": "100",
						"NW_IN": "50000",
						"NW_OUT": "50000"
					  },
					  "doc": "This overrides the capacity for broker 0. This broker is not a JBOD broker."
					},
					{
					  "brokerId": "1",
					  "capacity": {
						"DISK": {"/tmp/kafka-logs-1": "250000", "/tmp/kafka-logs-2": "250000"},
						"CPU": "100",
						"NW_IN": "50000",
						"NW_OUT": "50000"
					  },
					  "doc": "This overrides the capacity for broker 1. This broker is a JBOD broker."
					}
				  ]
				}`,
		},
		{
			testName: "generate correct capacity config when there is a broker config group",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{
									MountPath: "/path-from-default",
									PvcSpec: &v1.PersistentVolumeClaimSpec{
										Resources: v1.ResourceRequirements{
											Requests: v1.ResourceList{
												"storage": quantity,
											},
										},
									},
								},
							},
						},
					},
					Brokers: []v1beta1.Broker{
						{
							Id:                0,
							BrokerConfigGroup: "default",
							BrokerConfig: &v1beta1.BrokerConfig{
								Resources: &v1.ResourceRequirements{
									Limits: v1.ResourceList{
										"cpu": cpuQuantity,
									}},
							},
						},
						{
							Id:                1,
							BrokerConfigGroup: "default",
						},
						{
							Id:                2,
							BrokerConfigGroup: "default",
						},
						{
							Id:                3,
							BrokerConfigGroup: "default",
							BrokerConfig: &v1beta1.BrokerConfig{
								StorageConfigs: []v1beta1.StorageConfig{
									{
										MountPath: "/path1",
										PvcSpec: &v1.PersistentVolumeClaimSpec{
											Resources: v1.ResourceRequirements{
												Requests: v1.ResourceList{
													"storage": quantity,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConfiguration: `
				  {
					"brokerCapacities": [
                      {
					  "brokerId": "0",
					  "capacity": {
					   "DISK": {
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "200",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
                     {
					  "brokerId": "1",
					  "capacity": {
					   "DISK": {
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
                      {
					  "brokerId": "2",
					  "capacity": {
					   "DISK": {
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
					 {
					  "brokerId": "3",
					  "capacity": {
					   "DISK": {
						"/path1/kafka": "10737418240",
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
					 {
					  "brokerId": "-1",
					  "capacity": {
					   "DISK": {
						"/kafka-logs/kafka": "10737418240"
					   },
					   "CPU": "100",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 }
					]
                  }`,
		},
		{
			testName: "generate correct capacity config when there is no broker config group on last broker",
			kafkaCluster: v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{
									MountPath: "/path-from-default",
									PvcSpec: &v1.PersistentVolumeClaimSpec{
										Resources: v1.ResourceRequirements{
											Requests: v1.ResourceList{
												"storage": quantity,
											},
										},
									},
								},
							},
						},
					},
					Brokers: []v1beta1.Broker{
						{
							Id:                0,
							BrokerConfigGroup: "default",
						},
						{
							Id:                1,
							BrokerConfigGroup: "default",
						},
						{
							Id:                2,
							BrokerConfigGroup: "default",
						},
						{
							Id: 3,
							BrokerConfig: &v1beta1.BrokerConfig{
								NetworkConfig: &v1beta1.NetworkConfig{
									IncomingNetworkThroughPut: "200",
									OutgoingNetworkThroughPut: "200",
								},
								StorageConfigs: []v1beta1.StorageConfig{
									{
										MountPath: "/path1",
										PvcSpec: &v1.PersistentVolumeClaimSpec{
											Resources: v1.ResourceRequirements{
												Requests: v1.ResourceList{
													"storage": quantity,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConfiguration: `{
					"brokerCapacities": [
                      {
					  "brokerId": "0",
					  "capacity": {
					   "DISK": {
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
                     {
					  "brokerId": "1",
					  "capacity": {
					   "DISK": {
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
                      {
					  "brokerId": "2",
					  "capacity": {
					   "DISK": {
						"/path-from-default/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
					 {
					  "brokerId": "3",
					  "capacity": {
					   "DISK": {
						"/path1/kafka": "10737418240"
					   },
					   "CPU": "150",
					   "NW_IN": "200",
					   "NW_OUT": "200"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 },
					 {
					  "brokerId": "-1",
					  "capacity": {
					   "DISK": {
						"/kafka-logs/kafka": "10737418240"
					   },
					   "CPU": "100",
					   "NW_IN": "125000",
					   "NW_OUT": "125000"
					  },
					  "doc": "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
					 }
					]
                  }`,
		},
	}

	t.Parallel()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			var actual CapacityConfig
			err := json.Unmarshal([]byte(GenerateCapacityConfig(&test.kafkaCluster, log.NullLogger{}, nil)), &actual)
			if err != nil {
				t.Error(err, "could not actual unmarshal json")
			}

			var expected CapacityConfig
			err = json.Unmarshal([]byte(test.expectedConfiguration), &expected)
			if err != nil {
				t.Error(err, "could not unmarshal expected json")
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Error("Expected:", expected, ", got:", actual)
			}
		})
	}
}
