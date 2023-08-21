// Copyright © 2022 Cisco Systems, Inc. and/or its affiliates
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
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources"
	"github.com/banzaicloud/koperator/pkg/resources/kafka/mocks"
)

func TestReconciler_pvc(t *testing.T) {
	kafkaCluster := &v1beta1.KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka",
			Namespace: "kafka",
		},
	}

	mockCtrl := gomock.NewController(t)
	mockClient := mocks.NewMockClient(mockCtrl)

	r := Reconciler{
		Reconciler: resources.Reconciler{
			Client:       mockClient,
			KafkaCluster: kafkaCluster,
		},
	}

	testCases := []struct {
		testName                      string
		storageConfig                 v1beta1.StorageConfig
		expectedPersistentVolumeClaim *corev1.PersistentVolumeClaim
	}{
		{
			testName: "storage config with no template",
			storageConfig: v1beta1.StorageConfig{
				MountPath: "/kafka-logs-1",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"label1": "value1"},
					},
				},
			},
			expectedPersistentVolumeClaim: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    kafkaCluster.GetNamespace(),
					Name:         "",
					GenerateName: fmt.Sprintf("%s-2-storage-1-", kafkaCluster.GetName()),
					Labels: map[string]string{
						v1beta1.AppLabelKey:      "kafka",
						v1beta1.KafkaCRLabelKey:  kafkaCluster.GetName(),
						v1beta1.BrokerIdLabelKey: "2",
					},
					Annotations: map[string]string{
						"mountPath": "/kafka-logs-1",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"label1": "value1"},
					},
				},
			},
		},
		{
			testName: "storage config with template",
			storageConfig: v1beta1.StorageConfig{
				MountPath: "/kafka-logs-1",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"label1":                 "value1",
							v1beta1.BrokerIdLabelKey: "{{ .BrokerId }}",
							// strip '/' from mount path as label selector values
							// has to start with an alphanumeric character': https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
							"mountPath": "{{ trimPrefix \"/\" .MountPath }}",
						},
					},
				},
			},
			expectedPersistentVolumeClaim: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    kafkaCluster.GetNamespace(),
					Name:         "",
					GenerateName: fmt.Sprintf("%s-2-storage-1-", kafkaCluster.GetName()),
					Labels: map[string]string{
						v1beta1.AppLabelKey:      "kafka",
						v1beta1.KafkaCRLabelKey:  kafkaCluster.GetName(),
						v1beta1.BrokerIdLabelKey: "2",
					},
					Annotations: map[string]string{
						"mountPath": "/kafka-logs-1",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"label1":                 "value1",
							v1beta1.BrokerIdLabelKey: "2",
							"mountPath":              "kafka-logs-1",
						},
					},
				},
			},
		},
		{
			testName: "storage config with template and very long mount path",
			storageConfig: v1beta1.StorageConfig{
				MountPath: "/mountpath/that/exceeds63characters/kafka-logs-123456789123456789",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"label1":                 "value1",
							v1beta1.BrokerIdLabelKey: "{{ .BrokerId }}",
							"mountPath":              "{{ trimPrefix \"/\" .MountPath | sha1sum }}",
						},
					},
				},
			},
			expectedPersistentVolumeClaim: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    kafkaCluster.GetNamespace(),
					Name:         "",
					GenerateName: fmt.Sprintf("%s-2-storage-1-", kafkaCluster.GetName()),
					Labels: map[string]string{
						v1beta1.AppLabelKey:      "kafka",
						v1beta1.KafkaCRLabelKey:  kafkaCluster.GetName(),
						v1beta1.BrokerIdLabelKey: "2",
					},
					Annotations: map[string]string{
						"mountPath": "/mountpath/that/exceeds63characters/kafka-logs-123456789123456789",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"label1":                 "value1",
							v1beta1.BrokerIdLabelKey: "2",
							"mountPath":              "4efe1a6cf77dced13b2906585c33b0e1f615d90e",
						},
					},
				},
			},
		},
		{
			testName: "storage config with volume name template",
			storageConfig: v1beta1.StorageConfig{
				MountPath: "/kafka-logs-1",
				PvcSpec: &corev1.PersistentVolumeClaimSpec{
					VolumeName: "{{ printf \"pv-kafka-kafka-%d-%s\" .BrokerId (sha1sum .MountPath) }}",
				},
			},
			expectedPersistentVolumeClaim: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    kafkaCluster.GetNamespace(),
					Name:         "",
					GenerateName: fmt.Sprintf("%s-2-storage-1-", kafkaCluster.GetName()),
					Labels: map[string]string{
						v1beta1.AppLabelKey:      "kafka",
						v1beta1.KafkaCRLabelKey:  kafkaCluster.GetName(),
						v1beta1.BrokerIdLabelKey: "2",
					},
					Annotations: map[string]string{
						"mountPath": "/kafka-logs-1",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "pv-kafka-kafka-2-9a2dfa25bcd3dc4884a6ba1a3a5afd62e055fc29",
				},
			},
		},
	}

	t.Parallel()

	for _, test := range testCases {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			pvc, err := r.pvc(2, 1, test.storageConfig)

			assert.NilError(t, err, "PVC creation should succeed")

			assert.Equal(t, test.expectedPersistentVolumeClaim.GetNamespace(), pvc.GetNamespace())
			assert.Equal(t, test.expectedPersistentVolumeClaim.GetName(), pvc.GetName())
			assert.Equal(t, test.expectedPersistentVolumeClaim.GetGenerateName(), pvc.GetGenerateName())

			matcher := gomega.Equal(test.expectedPersistentVolumeClaim.GetLabels())
			if ok, _ := matcher.Match(pvc.GetLabels()); !ok {
				t.Error(matcher.FailureMessage(pvc.GetLabels()))
				return
			}

			matcher = gomega.Equal(test.expectedPersistentVolumeClaim.GetAnnotations())
			if ok, _ := matcher.Match(pvc.GetAnnotations()); !ok {
				t.Error(matcher.FailureMessage(pvc.GetAnnotations()))
				return
			}

			matcher = gomega.BeEquivalentTo(test.expectedPersistentVolumeClaim.Spec)
			if ok, _ := matcher.Match(pvc.Spec); !ok {
				t.Error(matcher.FailureMessage(pvc.Spec))
				return
			}
		})
	}
}
