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

package webhook

import (
	"testing"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func TestCheckBrokerStorageRemoval(t *testing.T) {
	testCases := []struct {
		testName            string
		kafkaClusterSpecNew v1beta1.KafkaClusterSpec
		kafkaClusterSpecOld v1beta1.KafkaClusterSpec
		isValid             bool
	}{
		{
			testName: "1",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "2",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
					"default2": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							//	v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default2",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: false,
		},
		{
			testName: "3",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							//v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: false,
		},
		{
			testName: "4",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							//v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "5",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs3"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs1"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
		{
			testName: "6",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								v1beta1.StorageConfig{MountPath: "logs4"},
								v1beta1.StorageConfig{MountPath: "logs5"},
								v1beta1.StorageConfig{MountPath: "logs6"},
							},
						},
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								v1beta1.StorageConfig{MountPath: "logs4"},
								v1beta1.StorageConfig{MountPath: "logs5"},
								v1beta1.StorageConfig{MountPath: "logs6"},
							},
						},
					},
				},
			},
			isValid: true,
		},
		{
			testName: "7",
			kafkaClusterSpecNew: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								v1beta1.StorageConfig{MountPath: "logs4"},
								v1beta1.StorageConfig{MountPath: "logs5"},
								v1beta1.StorageConfig{MountPath: "logs6"},
							},
						},
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": v1beta1.BrokerConfig{
						StorageConfigs: []v1beta1.StorageConfig{
							v1beta1.StorageConfig{MountPath: "logs1"},
							v1beta1.StorageConfig{MountPath: "logs2"},
							v1beta1.StorageConfig{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					v1beta1.Broker{
						Id:                1,
						BrokerConfigGroup: "default",
						BrokerConfig: &v1beta1.BrokerConfig{
							StorageConfigs: []v1beta1.StorageConfig{
								v1beta1.StorageConfig{MountPath: "logs4"},
								v1beta1.StorageConfig{MountPath: "logs5"},
								v1beta1.StorageConfig{MountPath: "logs8"},
							},
						},
					},
				},
			},
			isValid: false,
		},
	}

	for _, testCase := range testCases {
		res := checkBrokerStorageRemoval(&testCase.kafkaClusterSpecOld, &testCase.kafkaClusterSpecNew)
		if res != nil && testCase.isValid {
			t.Errorf("Message: %s, testName: %s", res.Result.Message, testCase.testName)
		} else if res == nil && !testCase.isValid {
			t.Errorf("there should be storage removal, testName: %s", testCase.testName)
		}

	}

}
