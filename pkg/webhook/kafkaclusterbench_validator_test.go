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
	"fmt"
	"testing"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/koperator/api/v1beta1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	json "github.com/json-iterator/go"
)

func CalculatePatch(currentObject, modifiedObject runtimeClient.Object, scheme *runtime.Scheme, opts ...patch.CalculateOption) (*patch.PatchResult, error) {

	// // when we ignore fields when comparing old and new objects we should
	// // ignore resourceVersion changes as well since that may be caused by changes to the ignored field
	// opts = append(opts, ignoreResourceVersion())

	// // ignore API server maintained 'managedFields' field as we are not interested in changes made to it
	// opts = append(opts, ignoreManagedFields())
	var err error

	currentObject, err = convertTypedResourceToUnstructured(currentObject, scheme)
	if err != nil {
		return nil, err
	}

	modifiedObject, err = convertTypedResourceToUnstructured(modifiedObject, scheme)
	if err != nil {
		return nil, err
	}

	return patch.DefaultPatchMaker.Calculate(currentObject, modifiedObject)
}

func convertTypedResourceToUnstructured(srcObj runtimeClient.Object, scheme *runtime.Scheme) (runtimeClient.Object, error) {
	// runtime.DefaultUnstructuredConverter.ToUnstructured converts PeerAuthentication incorrectly
	// thus we need to convert srcObj to map[string]interface{} first and than create Unstructured resource from it
	src, err := json.Marshal(srcObj)
	if err != nil {
		return nil, err
	}

	content := map[string]interface{}{}
	err = json.Unmarshal(src, &content)
	if err != nil {
		return nil, err
	}

	dst := &unstructured.Unstructured{Object: content}
	gvk, err := apiutil.GVKForObject(srcObj, scheme)
	if err != nil {
		return nil, err
	}

	dst.SetKind(gvk.Kind)
	dst.SetAPIVersion(gvk.GroupVersion().String())

	return dst, nil
}
func BenchmarkCheckBrokerStorageRemoval(t *testing.B) {
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
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
	}
	for k := 2; k < 1000; k++ {
		testCases[0].kafkaClusterSpecOld.Brokers = append(testCases[0].kafkaClusterSpecOld.Brokers, v1beta1.Broker{Id: int32(k), BrokerConfigGroup: "default"})
		testCases[0].kafkaClusterSpecNew.Brokers = append(testCases[0].kafkaClusterSpecNew.Brokers, v1beta1.Broker{Id: int32(k), BrokerConfigGroup: "default"})
	}
	for i := 0; i < t.N; i++ {
		_ = checkBrokerStorageRemoval(&testCases[0].kafkaClusterSpecOld, &testCases[0].kafkaClusterSpecNew)
	}

}

func checkBrokerStorageRemovalMap(kafkaClusterSpecOld, kafkaClusterSpecNew *banzaicloudv1beta1.KafkaClusterSpec) *admissionv1.AdmissionResponse {
	idToBroker := make(map[int32]banzaicloudv1beta1.Broker)

	for _, broker := range kafkaClusterSpecNew.Brokers {
		idToBroker[broker.Id] = broker
	}

	for _, brokerOld := range kafkaClusterSpecOld.Brokers {
		if brokerNew, ok := idToBroker[brokerOld.Id]; ok {
			brokerConfigsOld, _ := brokerOld.GetBrokerConfig(*kafkaClusterSpecOld)
			brokerConfigsNew, _ := brokerNew.GetBrokerConfig(*kafkaClusterSpecNew)
			for _, storageConfigOld := range brokerConfigsOld.StorageConfigs {
				isStorageFound := false

				for _, storageConfigNew := range brokerConfigsNew.StorageConfigs {
					if storageConfigOld.MountPath == storageConfigNew.MountPath {
						isStorageFound = true
						break
					}
				}
				if !isStorageFound {
					log.Info(fmt.Sprintf("Not allowed to remove broker storage with mountPath: %s from brokerID: %v", storageConfigOld.MountPath, brokerOld.Id))
					return notAllowed(fmt.Sprintf("Removing storage from a runnng broker is not supported! (mountPath: %s, brokerID: %v)", storageConfigOld.MountPath, brokerOld.Id), metav1.StatusReasonInvalid)
				}
			}
		}
	}
	return nil
}

func BenchmarkCheckBrokerStorageRemovalMap(t *testing.B) {
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
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			kafkaClusterSpecOld: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						StorageConfigs: []v1beta1.StorageConfig{
							{MountPath: "logs1"},
							{MountPath: "logs2"},
							{MountPath: "logs3"},
						},
					},
				},
				Brokers: []v1beta1.Broker{
					{
						Id:                1,
						BrokerConfigGroup: "default",
					},
				},
			},
			isValid: true,
		},
	}
	for k := 2; k < 1000; k++ {
		testCases[0].kafkaClusterSpecOld.Brokers = append(testCases[0].kafkaClusterSpecOld.Brokers, v1beta1.Broker{Id: int32(k), BrokerConfigGroup: "default"})
		testCases[0].kafkaClusterSpecNew.Brokers = append(testCases[0].kafkaClusterSpecNew.Brokers, v1beta1.Broker{Id: int32(k), BrokerConfigGroup: "default"})
	}
	for i := 0; i < t.N; i++ {
		_ = checkBrokerStorageRemovalMap(&testCases[0].kafkaClusterSpecOld, &testCases[0].kafkaClusterSpecNew)
	}

}

func BenchmarkCalculate(b *testing.B) {
	testCases2 := []struct {
		testName        string
		kafkaClusterNew v1beta1.KafkaCluster
		kafkaClusterOld v1beta1.KafkaCluster
		isValid         bool
	}{
		{
			testName: "1",
			kafkaClusterNew: v1beta1.KafkaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-namespace"},
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs1"},
								{MountPath: "logs2"},
								{MountPath: "logs3"},
							},
						},
					},
					Brokers: []v1beta1.Broker{
						{
							Id:                1,
							BrokerConfigGroup: "default",
						},
						{
							Id:                2,
							BrokerConfigGroup: "default",
							BrokerConfig: &v1beta1.BrokerConfig{
								StorageConfigs: []v1beta1.StorageConfig{
									{MountPath: "logs4"},
									{MountPath: "logs5"},
									{MountPath: "logs6"},
								},
							},
						},
						{
							Id:                3,
							BrokerConfigGroup: "default",
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState:             map[string]v1beta1.BrokerState{},
					CruiseControlTopicStatus: "",
					State:                    "",
					RollingUpgrade:           v1beta1.RollingUpgradeStatus{},
					AlertCount:               0,
					ListenerStatuses: v1beta1.ListenerStatuses{
						InternalListeners: map[string]v1beta1.ListenerStatusList{},
						ExternalListeners: map[string]v1beta1.ListenerStatusList{},
					},
				},
			},
			kafkaClusterOld: v1beta1.KafkaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-namespace"},
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							StorageConfigs: []v1beta1.StorageConfig{
								{MountPath: "logs3"},
								{MountPath: "logs2"},
								{MountPath: "logs1"},
							},
						},
					},
					Brokers: []v1beta1.Broker{
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
						},
					},
				},
				Status: v1beta1.KafkaClusterStatus{
					BrokersState:             map[string]v1beta1.BrokerState{},
					CruiseControlTopicStatus: "",
					State:                    "",
					RollingUpgrade:           v1beta1.RollingUpgradeStatus{},
					AlertCount:               0,
					ListenerStatuses: v1beta1.ListenerStatuses{
						InternalListeners: map[string]v1beta1.ListenerStatusList{},
						ExternalListeners: map[string]v1beta1.ListenerStatusList{},
					},
				},
			},
			isValid: true,
		},
	}
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)
	for i := 0; i < b.N; i++ {
		_, _ = CalculatePatch(&testCases2[0].kafkaClusterOld, &testCases2[0].kafkaClusterNew, scheme)

	}

}
