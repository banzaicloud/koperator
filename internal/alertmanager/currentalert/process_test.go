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
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

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
		t.Run(tt.name, func(t *testing.T) {

			for _, pvc := range tt.pvcList.Items {
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
