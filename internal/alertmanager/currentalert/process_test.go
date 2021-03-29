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

package currentalert

import (
	"context"
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

func Test_resizePvc(t *testing.T) {
	testClient := fake.NewFakeClientWithScheme(scheme.Scheme)

	//Setup test kafka cluster and pvc
	setupEnvironment(t, testClient)

	testCase := []struct {
		name      string
		alertList []model.Alert
		pvc       corev1.PersistentVolumeClaim
		cluster   v1beta1.KafkaCluster
	}{
		{
			name: "Resize pvc created by default group",
			alertList: []model.Alert{
				{
					Labels: model.LabelSet{
						"namespace":             "kafka",
						"persistentvolumeclaim": "testPvc",
					},
					Annotations: model.LabelSet{
						"command":     "resizePvc",
						"incrementBy": "2G",
					},
				},
			},
			pvc: corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testPvc",
					Namespace: "kafka",
					Labels: map[string]string{
						"app":      "kafka",
						"brokerId": "0",
						"kafka_cr": "test-cluster",
					},
					Annotations: map[string]string{
						"mountPath":                          "/kafka-logs",
						"volume.kubernetes.io/selected-node": "test-node",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: util.StringPointer("gp2"),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("2Gi"),
						},
					},
				},
			},
			cluster: v1beta1.KafkaCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "kafka",
				},
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{
									MountPath: "/kafka-logs",
									PvcSpec: &corev1.PersistentVolumeClaimSpec{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceStorage: resource.MustParse("4Gi"),
											},
										},
									},
								},
							},
						},
					},
					Brokers: []v1beta1.Broker{
						{
							Id:                int32(0),
							BrokerConfigGroup: "default",
						},
					},
				},
			},
		},
		{
			name: "Resize pvc from broker specific config",
			alertList: []model.Alert{
				{
					Labels: model.LabelSet{
						"namespace":             "kafka",
						"persistentvolumeclaim": "testPvc",
					},
					Annotations: model.LabelSet{
						"command":     "resizePvc",
						"incrementBy": "2G",
					},
				},
			},
			pvc: corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testPvc",
					Namespace: "kafka",
					Labels: map[string]string{
						"app":      "kafka",
						"brokerId": "0",
						"kafka_cr": "test-cluster",
					},
					Annotations: map[string]string{
						"mountPath":                          "/kafka-logs",
						"volume.kubernetes.io/selected-node": "test-node",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: util.StringPointer("gp2"),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("2Gi"),
						},
					},
				},
			},
			cluster: v1beta1.KafkaCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "kafka",
				},
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{
									MountPath: "/kafka-logs",
									PvcSpec: &corev1.PersistentVolumeClaimSpec{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceStorage: resource.MustParse("1Gi"),
											},
										},
									},
								},
							},
						},
					},
					Brokers: []v1beta1.Broker{
						{
							Id: int32(0),
							BrokerConfig: &v1beta1.BrokerConfig{
								StorageConfigs: []v1beta1.StorageConfig{
									{
										MountPath: "/kafka-logs",
										PvcSpec: &corev1.PersistentVolumeClaimSpec{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceStorage: resource.MustParse("4Gi"),
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
		},
	}

	for _, tt := range testCase {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := testClient.Create(context.Background(), &tt.pvc)
			if err != nil {
				t.Error("Pvc creation failed", err)
			}
			defer testClient.Delete(context.Background(), &tt.pvc)

			err = testClient.Create(context.Background(), &tt.cluster)
			if err != nil {
				t.Error("kafka cluster creation failed", err)
			}

			for _, alert := range tt.alertList {
				err := resizePvc(logf.NullLogger{}, alert.Labels, alert.Annotations, testClient)
				if err != nil {
					t.Errorf("process.resizePvc() error = %v", err)
				}
			}

			var kafkaCluster v1beta1.KafkaCluster
			err = testClient.Get(
				context.Background(),
				types.NamespacedName{Namespace: "kafka", Name: "test-cluster"},
				&kafkaCluster)
			if err != nil {
				t.Errorf("kafka cr was not found, error = %v", err)
			}
			defer testClient.Delete(context.Background(), &kafkaCluster)

			fmt.Println(kafkaCluster.Spec.Brokers)
			brokerStorageConfig := &kafkaCluster.Spec.Brokers[0].BrokerConfig.StorageConfigs[0]

			if brokerStorageConfig.PvcSpec.Resources.Requests.Storage().Value() != 6294967296 {
				t.Error("invalid storage size")
			}
			if kafkaCluster.Spec.BrokerConfigGroups["default"].StorageConfigs[0].PvcSpec.Resources.Requests.Storage().Value() !=
				tt.cluster.Spec.BrokerConfigGroups["default"].StorageConfigs[0].PvcSpec.Resources.Requests.Storage().Value() {
				t.Error("request mutated the config group")
			}
		})
	}
}

func Test_addPvc(t *testing.T) {
	testClient := fake.NewFakeClientWithScheme(scheme.Scheme)

	//Setup test kafka cluster and pvc
	setupEnvironment(t, testClient)

	testCase := []struct {
		name      string
		alertList []model.Alert
		pvcList   *corev1.PersistentVolumeClaimList
	}{
		{
			name: "addPvc alert successfully creates pvc",
			alertList: []model.Alert{
				{
					Labels: model.LabelSet{
						"kafka_cr":              "kafka",
						"namespace":             "kafka",
						"persistentvolumeclaim": "testPvc",
						"node":                  "test-node",
					},
					Annotations: model.LabelSet{
						"command":         "addPvc",
						"mountPathPrefix": "/kafka-logs",
						"diskSize":        "2G",
					},
				},
			},
			pvcList: &corev1.PersistentVolumeClaimList{
				Items: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      "testPvc",
							Namespace: "kafka",
							Labels: map[string]string{
								"app":      "kafka",
								"brokerId": "0",
								"kafka_cr": "kafka",
							},
							Annotations: map[string]string{
								"mountPath":                          "/kafka-logs",
								"volume.kubernetes.io/selected-node": "test-node",
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							StorageClassName: util.StringPointer("gp2"),
						},
					},
				},
			},
		},
		{
			name: "addPvc skips pvc creation because of an existing unbound pvc",
			alertList: []model.Alert{
				{
					Labels: model.LabelSet{
						"kafka_cr":              "kafka",
						"namespace":             "kafka",
						"persistentvolumeclaim": "testPvc1",
						"node":                  "test-node",
					},
					Annotations: model.LabelSet{
						"command":         "addPvc",
						"mountPathPrefix": "/kafka-logs",
						"diskSize":        "2G",
					},
				},
				{
					Labels: model.LabelSet{
						"kafka_cr":              "kafka",
						"namespace":             "kafka",
						"persistentvolumeclaim": "testPvc2",
						"node":                  "test-node",
					},
					Annotations: model.LabelSet{
						"command":         "addPvc",
						"mountPathPrefix": "/kafka-logs2",
						"diskSize":        "4G",
					},
				},
			},
			pvcList: &corev1.PersistentVolumeClaimList{
				Items: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      "testPvc1",
							Namespace: "kafka",
							Labels: map[string]string{
								"app":      "kafka",
								"brokerId": "0",
								"kafka_cr": "kafka",
							},
							Annotations: map[string]string{
								"mountPath":                          "/kafka-logs",
								"volume.kubernetes.io/selected-node": "test-node",
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							StorageClassName: util.StringPointer("gp2"),
						},
						Status: corev1.PersistentVolumeClaimStatus{
							Phase: corev1.ClaimPending,
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      "testPvc2",
							Namespace: "kafka",
							Labels: map[string]string{
								"app":      "kafka",
								"brokerId": "0",
								"kafka_cr": "kafka",
							},
							Annotations: map[string]string{
								"mountPath":                          "/kafka-logs",
								"volume.kubernetes.io/selected-node": "test-node",
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							StorageClassName: util.StringPointer("gp2"),
						},
						Status: corev1.PersistentVolumeClaimStatus{
							Phase: corev1.ClaimPending,
						},
					},
				},
			},
		},
	}

	for testCaseIndex, tt := range testCase {
		tt := tt
		testCaseIndex := testCaseIndex
		t.Run(tt.name, func(t *testing.T) {
			for _, pvc := range tt.pvcList.Items {
				pvc := pvc
				err := testClient.Create(context.Background(), &pvc)
				if err != nil {
					t.Error("Pvc creation failed", err)
				}
			}

			for _, alert := range tt.alertList {
				err := addPvc(logf.NullLogger{}, alert.Labels, alert.Annotations, testClient)
				if err != nil {
					t.Errorf("process.addPvc() error = %v", err)
				}
			}

			var kafkaCluster v1beta1.KafkaCluster
			err := testClient.Get(
				context.Background(),
				types.NamespacedName{Namespace: "kafka", Name: "kafka"},
				&kafkaCluster)
			if err != nil {
				t.Errorf("kafka cr was not found, error = %v", err)
			}

			if len(tt.pvcList.Items) <= 1 {
				brokerStorageConfig := &kafkaCluster.Spec.Brokers[0].BrokerConfig.StorageConfigs[0]

				if brokerStorageConfig.MountPath == string(testCase[testCaseIndex].alertList[0].Annotations["mountPathPrefix"]) {
					t.Error("Broker storage config mountpath should not be the same as the original mountPathPrefix")
				}

				storageRequest := brokerStorageConfig.PvcSpec.Resources.Requests["storage"]
				if storageRequest.String() !=
					string(testCase[testCaseIndex].alertList[0].Annotations["diskSize"]) {
					t.Error("Broker storage config memory request should be same as in the alert")
				}
			} else {
				brokerStorageConfig := &kafkaCluster.Spec.Brokers[0].BrokerConfig.StorageConfigs[0]

				for alertListIndex := range testCase[testCaseIndex].alertList {
					if brokerStorageConfig.MountPath == string(testCase[testCaseIndex].alertList[alertListIndex].Annotations["mountPathPrefix"]) {
						t.Error("Broker storage config mountpath should not be the same as the original mountPathPrefix")
					}

					storageRequest := brokerStorageConfig.PvcSpec.Resources.Requests["storage"]
					if storageRequest.String() == string(testCase[testCaseIndex].alertList[1].Annotations["diskSize"]) {
						t.Error("Broker storage config memory request should not be same as in the second alert")
					}
				}
			}

			//Cleanup Pvcs from previous tests
			cleanupPvcs(testClient, tt, t)
		})
	}
}

func Test_upScale(t *testing.T) {
	testClient := fake.NewFakeClientWithScheme(scheme.Scheme)

	testCases := []struct {
		testName        string
		kafkaCluster    v1beta1.KafkaCluster
		alert           model.Alert
		expectedBrokers []v1beta1.Broker
	}{
		{
			testName: "upScale with config group",
			kafkaCluster: v1beta1.KafkaCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{
									MountPath: "/kafka-logs",
									PvcSpec: &corev1.PersistentVolumeClaimSpec{
										AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceStorage: resource.MustParse("4Gi"),
											},
										},
									},
								},
							},
						},
					},
					Brokers: []v1beta1.Broker{{Id: 0, BrokerConfigGroup: "default"}},
				},
			},
			alert: model.Alert{
				Labels: model.LabelSet{
					"kafka_cr":   "test-cluster",
					"namespace":  "test-namespace",
					"severity":   "critical",
					"alertGroup": "test",
				},
				Annotations: map[model.LabelName]model.LabelValue{
					"command":           "upScale",
					"brokerConfigGroup": "default",
				},
			},
			expectedBrokers: []v1beta1.Broker{{Id: 0, BrokerConfigGroup: "default"}, {Id: 1, BrokerConfigGroup: "default"}},
		},
		{
			testName: "upScale with broker config",
			kafkaCluster: v1beta1.KafkaCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{
									MountPath: "/kafka-logs",
									PvcSpec: &corev1.PersistentVolumeClaimSpec{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceStorage: resource.MustParse("4Gi"),
											},
										},
									},
								},
							},
						},
					},
					Brokers: []v1beta1.Broker{{Id: 0, BrokerConfigGroup: "default"}},
				},
			},
			alert: model.Alert{
				Labels: model.LabelSet{
					"kafka_cr":   "test-cluster",
					"namespace":  "test-namespace",
					"severity":   "critical",
					"alertGroup": "test",
				},
				Annotations: map[model.LabelName]model.LabelValue{
					"command":           "upScale",
					"storageClass":      "gp2",
					"mountPath":         "/kafkalog",
					"diskSize":          "10G",
					"image":             "org/kafka:tag",
					"brokerAnnotations": "{ \"test annotation1\": \"ann value1\", \"test annotation2\": \"ann value2\" }",
				},
			},
			expectedBrokers: []v1beta1.Broker{
				{Id: 0, BrokerConfigGroup: "default"},
				{Id: 1, BrokerConfig: &v1beta1.BrokerConfig{
					Image:             "org/kafka:tag",
					BrokerAnnotations: map[string]string{"test annotation1": "ann value1", "test annotation2": "ann value2"},
					StorageConfigs: []v1beta1.StorageConfig{
						{
							MountPath: "/kafkalog",
							PvcSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: util.StringPointer("gp2"),
								AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceStorage: resource.MustParse("10G"),
									},
								},
							},
						},
					},
				}}},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			if err := testClient.Create(context.Background(), &test.kafkaCluster); err != nil {
				t.Error(err)
				return
			}

			defer testClient.Delete(context.Background(), &test.kafkaCluster)

			if err := upScale(logf.NullLogger{}, test.alert.Labels, test.alert.Annotations, testClient); err != nil {
				t.Error(err)
				return
			}

			var kafkaCluster v1beta1.KafkaCluster
			if err := testClient.Get(
				context.Background(),
				types.NamespacedName{Namespace: test.kafkaCluster.GetNamespace(), Name: test.kafkaCluster.GetName()},
				&kafkaCluster); err != nil {
				t.Error(err)
				return
			}

			m := gomega.ConsistOf(test.expectedBrokers)
			if ok, _ := m.Match(kafkaCluster.Spec.Brokers); !ok {
				t.Error(m.FailureMessage(kafkaCluster.Spec.Brokers))
				return
			}
		})
	}
}

func cleanupPvcs(testClient client.Client, tt struct {
	name      string
	alertList []model.Alert
	pvcList   *corev1.PersistentVolumeClaimList
}, t *testing.T) {
	for _, pvc := range tt.pvcList.Items {
		err := testClient.Delete(context.TODO(), &corev1.PersistentVolumeClaim{
			ObjectMeta: v1.ObjectMeta{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
			},
		})
		if err != nil {
			t.Error("Pvc cleanup failed")
		}
	}
}

func setupEnvironment(t *testing.T, testClient client.Client) {
	storageResourceQuantity, err := resource.ParseQuantity("10Gi")
	if err != nil {
		t.Error(err)
	}

	err = testClient.Create(context.Background(), &v1beta1.KafkaCluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      "kafka",
			Namespace: "kafka",
		},
		Spec: v1beta1.KafkaClusterSpec{
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
			},
			BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
				"default": {
					StorageConfigs: []v1beta1.StorageConfig{
						{
							MountPath: "/kafka-logs",
							PvcSpec: &corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									"ReadWriteOnce",
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										"storage": storageResourceQuantity,
									},
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		t.Error(err)
	}
}
