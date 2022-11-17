// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

package util

import (
	"reflect"
	"testing"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/istioingress"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestIntstrPointer(t *testing.T) {
	i := int(10)
	ptr := IntstrPointer(i)
	if ptr.String() != "10" {
		t.Error("Expected 10, got:", ptr.String())
	}
}

func TestInt64Pointer(t *testing.T) {
	i := int64(10)
	ptr := Int64Pointer(i)
	if *ptr != int64(10) {
		t.Error("Expected pointer to 10, got:", *ptr)
	}
}

func TestInt32Pointer(t *testing.T) {
	i := int32(10)
	ptr := Int32Pointer(i)
	if *ptr != int32(10) {
		t.Error("Expected pointer to 10, got:", *ptr)
	}
}

func TestBoolPointer(t *testing.T) {
	b := true
	ptr := BoolPointer(b)
	if !*ptr {
		t.Error("Expected ptr to true, got false")
	}

	b = false
	ptr = BoolPointer(b)
	if *ptr {
		t.Error("Expected ptr to false, got true")
	}
}

func TestStringPointer(t *testing.T) {
	str := "test"
	ptr := StringPointer(str)
	if *ptr != "test" {
		t.Error("Expected ptr to 'test', got:", *ptr)
	}
}

func TestMapStringStringPointer(t *testing.T) {
	m := map[string]string{
		"test-key": "test-value",
	}
	ptr := MapStringStringPointer(m)
	for k, v := range ptr {
		if k != "test-key" {
			t.Error("Expected ptr map key 'test-key', got:", k)
		}
		if *v != "test-value" {
			t.Error("Expected ptr map value 'test-value', got:", *v)
		}
	}
}

func TestConvertStringToInt32(t *testing.T) {
	i := ConvertStringToInt32("10")
	if i != 10 {
		t.Error("Expected 10, got:", i)
	}
	i = ConvertStringToInt32("string")
	if i != -1 {
		t.Error("Expected -1 for non-convertable string, got:", i)
	}
}

func TestIsSSLEnabledForInternalCommunication(t *testing.T) {
	lconfig := []v1beta1.InternalListenerConfig{
		{
			UsedForInnerBrokerCommunication: true,
			CommonListenerSpec:              v1beta1.CommonListenerSpec{Type: "ssl"},
		},
	}
	if !IsSSLEnabledForInternalCommunication(lconfig) {
		t.Error("Expected ssl enabled for internal communication, got disabled")
	}
	lconfig = []v1beta1.InternalListenerConfig{
		{
			UsedForInnerBrokerCommunication: true,
			CommonListenerSpec:              v1beta1.CommonListenerSpec{Type: "plaintext"},
		},
	}
	if IsSSLEnabledForInternalCommunication(lconfig) {
		t.Error("Expected ssl disabled for internal communication, got enabled")
	}
}

func TestStringSliceContains(t *testing.T) {
	slice := []string{"1", "2", "3"}
	if !StringSliceContains(slice, "1") {
		t.Error("Expected slice contains 1, got false")
	}
	if StringSliceContains(slice, "4") {
		t.Error("Expected slice not contains 4, got true")
	}
}

func TestStringSliceRemove(t *testing.T) {
	slice := []string{"1", "2", "3"}
	removed := StringSliceRemove(slice, "3")
	expected := []string{"1", "2"}
	if !reflect.DeepEqual(removed, expected) {
		t.Error("Expected:", expected, "got:", removed)
	}
}

func TestMergeAnnotations(t *testing.T) {
	annotations := map[string]string{"foo": "bar", "bar": "foo"}
	annotations2 := map[string]string{"thing": "1", "other_thing": "2"}

	combined := MergeAnnotations(annotations, annotations2)

	if len(combined) != 4 {
		t.Error("Annotations didn't combine correctly")
	}
}

func TestCreateLogger(t *testing.T) {
	logger := CreateLogger(false, false)
	if logger.GetSink() == nil {
		t.Fatal("created Logger instance should not be nil")
	}
	if logger.V(1).Enabled() {
		t.Error("debug level should not be enabled")
	}
	if !logger.V(0).Enabled() {
		t.Error("info level should be enabled")
	}

	logger = CreateLogger(true, true)
	if !logger.V(1).Enabled() {
		t.Error("debug level should be enabled")
	}
	if !logger.V(0).Enabled() {
		t.Error("info level should be enabled")
	}
}

func TestGetBrokerIdsFromStatusAndSpec(t *testing.T) {
	testCases := []struct {
		states         map[string]v1beta1.BrokerState
		brokers        []v1beta1.Broker
		expectedOutput []int
	}{
		// the states and the spec matches
		{
			map[string]v1beta1.BrokerState{
				"1": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
				"2": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
				"3": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
			},
			[]v1beta1.Broker{
				{
					Id: 1,
				},
				{
					Id: 2,
				},
				{
					Id: 3,
				},
			},
			[]int{1, 2, 3},
		},
		// broker has been deleted from spec
		{
			map[string]v1beta1.BrokerState{
				"1": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
				"2": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
				"3": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
			},
			[]v1beta1.Broker{
				{
					Id: 1,
				},
				{
					Id: 2,
				},
			},
			[]int{1, 2, 3},
		},
		// broker is added to spec
		{
			map[string]v1beta1.BrokerState{
				"1": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
				"2": {
					ConfigurationState: v1beta1.ConfigInSync,
				},
			},
			[]v1beta1.Broker{
				{
					Id: 1,
				},
				{
					Id: 2,
				},
				{
					Id: 3,
				},
			},
			[]int{1, 2, 3},
		},
	}

	logger := zap.New()
	for _, testCase := range testCases {
		brokerIds := GetBrokerIdsFromStatusAndSpec(testCase.states, testCase.brokers, logger)
		if len(brokerIds) != len(testCase.expectedOutput) {
			t.Errorf("size of the merged slice of broker ids mismatch - expected: %d, actual: %d", len(testCase.expectedOutput), len(brokerIds))
		}
		for i, brokerId := range brokerIds {
			if brokerId != testCase.expectedOutput[i] {
				t.Errorf("broker id is not the expected - index: %d, expected: %d, actual: %d", i, testCase.expectedOutput[i], brokerId)
			}
		}
	}
}
func TestGetIngressConfigs(t *testing.T) {
	defaultKafkaClusterWithEnvoy := &v1beta1.KafkaClusterSpec{
		EnvoyConfig: v1beta1.EnvoyConfig{
			Image: "envoyproxy/envoy:v1.22.2",
			Resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
			},
			Replicas:           1,
			ServiceAccountName: "default",
		},
	}

	defaultKafkaClusterWithIstioIngress := &v1beta1.KafkaClusterSpec{
		IngressController: istioingress.IngressControllerName,
		IstioIngressConfig: v1beta1.IstioIngressConfig{
			Resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
			},
			Replicas: 1,
		},
	}

	testCases := []struct {
		globalConfig                     v1beta1.KafkaClusterSpec
		externalListenerSpecifiedConfigs v1beta1.ExternalListenerConfig
		expectedOutput                   map[string]v1beta1.IngressConfig
	}{
		// only globalEnvoy configuration is set
		{
			*defaultKafkaClusterWithEnvoy,
			v1beta1.ExternalListenerConfig{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9094,
				},
				ExternalStartingPort: 19090,
			},
			map[string]v1beta1.IngressConfig{
				IngressConfigGlobalName: {EnvoyConfig: &defaultKafkaClusterWithEnvoy.EnvoyConfig},
			},
		},
		// only globalIstio ingress configuration is set
		{
			*defaultKafkaClusterWithIstioIngress,
			v1beta1.ExternalListenerConfig{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9094,
				},
				ExternalStartingPort: 19090,
			},
			map[string]v1beta1.IngressConfig{
				IngressConfigGlobalName: {IstioIngressConfig: &defaultKafkaClusterWithIstioIngress.IstioIngressConfig},
			},
		},
		// ExternalListener Specified config is set with EnvoyIngress
		{
			*defaultKafkaClusterWithEnvoy,
			v1beta1.ExternalListenerConfig{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9094,
				},
				ExternalStartingPort: 19090,
				Config: &v1beta1.Config{
					DefaultIngressConfig: "az1",
					IngressConfig: map[string]v1beta1.IngressConfig{
						"az1": {
							IngressServiceSettings: v1beta1.IngressServiceSettings{
								HostnameOverride: "foo.bar",
							},
							EnvoyConfig: &v1beta1.EnvoyConfig{
								Replicas:    3,
								Annotations: map[string]string{"az1": "region"},
							},
						},
						"az2": {
							EnvoyConfig: &v1beta1.EnvoyConfig{
								Image:       "envoyproxy/envoy:v1.22.2",
								Annotations: map[string]string{"az2": "region"},
							},
						},
					},
				},
			},
			map[string]v1beta1.IngressConfig{
				"az1": {
					IngressServiceSettings: v1beta1.IngressServiceSettings{
						HostnameOverride: "foo.bar",
					},
					EnvoyConfig: &v1beta1.EnvoyConfig{
						Image: "envoyproxy/envoy:v1.22.2",
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
						},
						Replicas:           3,
						Annotations:        map[string]string{"az1": "region"},
						ServiceAccountName: "default",
					},
				},
				"az2": {
					EnvoyConfig: &v1beta1.EnvoyConfig{
						Image: "envoyproxy/envoy:v1.22.2",
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
						},
						Annotations:        map[string]string{"az2": "region"},
						Replicas:           1,
						ServiceAccountName: "default",
					},
				},
			},
		},
		// ExternalListener Specified config is set with IstioIngress
		{
			*defaultKafkaClusterWithIstioIngress,
			v1beta1.ExternalListenerConfig{
				CommonListenerSpec: v1beta1.CommonListenerSpec{
					Type:          "plaintext",
					Name:          "external",
					ContainerPort: 9094,
				},
				ExternalStartingPort: 19090,
				Config: &v1beta1.Config{
					DefaultIngressConfig: "az1",
					IngressConfig: map[string]v1beta1.IngressConfig{
						"az1": {
							IngressServiceSettings: v1beta1.IngressServiceSettings{
								HostnameOverride: "foo.bar",
							},
							IstioIngressConfig: &v1beta1.IstioIngressConfig{
								Replicas:    3,
								Annotations: map[string]string{"az1": "region"},
							},
						},
						"az2": {
							IstioIngressConfig: &v1beta1.IstioIngressConfig{
								Annotations: map[string]string{"az2": "region"},
							},
						},
					},
				},
			},
			map[string]v1beta1.IngressConfig{
				"az1": {
					IngressServiceSettings: v1beta1.IngressServiceSettings{
						HostnameOverride: "foo.bar",
					},
					IstioIngressConfig: &v1beta1.IstioIngressConfig{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
						},
						Replicas:    3,
						Annotations: map[string]string{"az1": "region"},
					},
				},
				"az2": {
					IstioIngressConfig: &v1beta1.IstioIngressConfig{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
						},
						Annotations: map[string]string{"az2": "region"},
						Replicas:    1,
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		ingressConfigs, _, err := GetIngressConfigs(testCase.globalConfig, testCase.externalListenerSpecifiedConfigs)
		if err != nil {
			t.Errorf("unexpected error occurred during merging envoyconfigs")
		}
		if len(ingressConfigs) != len(testCase.expectedOutput) {
			t.Errorf("size of the merged slice of envoyConfig mismatch - expected: %d, actual: %d", len(testCase.expectedOutput), len(ingressConfigs))
		}
		for i, envoyConfig := range ingressConfigs {
			assert.DeepEqual(t, envoyConfig, testCase.expectedOutput[i])
		}
	}
}

func TestIsIngressConfigInUse(t *testing.T) {
	logger := zap.New()
	testCases := []struct {
		cluster           *v1beta1.KafkaCluster
		iConfigName       string
		defaultConfigName string
		expectedOutput    bool
	}{
		// Only the global config is in use
		{
			iConfigName:    IngressConfigGlobalName,
			expectedOutput: true,
		},
		// Config is in use with config group
		{
			iConfigName: "foo",
			cluster: &v1beta1.KafkaCluster{
				Spec: v1beta1.KafkaClusterSpec{
					BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
						"default": {
							BrokerIngressMapping: []string{"foo"},
						},
					},
					Brokers: []v1beta1.Broker{
						{Id: 0, BrokerConfigGroup: "default"},
						{Id: 1, BrokerConfigGroup: "default"},
						{Id: 2, BrokerConfigGroup: "default"},
					},
				},
			},
			expectedOutput: true,
		},
		// Config is in use without config group
		{
			iConfigName: "foo",
			cluster: &v1beta1.KafkaCluster{Spec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 0, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{"foo"},
					}},
					{Id: 1, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{"foo"},
					}},
					{Id: 2, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{"foo"},
					}},
				},
			},
			},
			expectedOutput: true,
		},
		// Config is not in use with config group
		{
			iConfigName: "bar",
			cluster: &v1beta1.KafkaCluster{Spec: v1beta1.KafkaClusterSpec{
				BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
					"default": {
						BrokerIngressMapping: []string{"foo"},
					},
				},
				Brokers: []v1beta1.Broker{
					{Id: 0, BrokerConfigGroup: "default"},
					{Id: 1, BrokerConfigGroup: "default"},
					{Id: 2, BrokerConfigGroup: "default"},
				},
			},
			},
			expectedOutput: false,
		},
		// Config is not in use without config group
		{
			iConfigName: "bar",
			cluster: &v1beta1.KafkaCluster{Spec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 0, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{"foo"},
					}},
					{Id: 1, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{"foo"},
					}},
					{Id: 2, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{"foo"},
					}},
				},
			},
			},
			expectedOutput: false,
		},
		// Config is in use as a default config
		{
			iConfigName:       "bar",
			defaultConfigName: "bar",
			cluster: &v1beta1.KafkaCluster{Spec: v1beta1.KafkaClusterSpec{
				Brokers: []v1beta1.Broker{
					{Id: 0, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{},
					}},
					{Id: 1, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{},
					}},
					{Id: 2, BrokerConfig: &v1beta1.BrokerConfig{
						BrokerIngressMapping: []string{},
					}},
				},
			},
			},
			expectedOutput: true,
		},
	}
	for _, testCase := range testCases {
		result := IsIngressConfigInUse(testCase.iConfigName, testCase.defaultConfigName, testCase.cluster, logger)
		if result != testCase.expectedOutput {
			t.Errorf("result does not match with the expected output - expected: %v, actual: %v", result, testCase.expectedOutput)
		}
	}
}

func TestConfigurationBackup(t *testing.T) {
	testCases := []struct {
		testName string
		broker   v1beta1.Broker
	}{
		{
			testName: "empty broker",
			broker:   v1beta1.Broker{},
		},
		{
			testName: "detailed broker",
			broker: v1beta1.Broker{
				Id:                0,
				BrokerConfigGroup: "default",
				ReadOnlyConfig: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true`,
				BrokerConfig: &v1beta1.BrokerConfig{
					Image:                "Image",
					MetricsReporterImage: "MetricsReporterImage",
					Config: `advertised.listeners=INTERNAL://kafka-0.kafka.svc.cluster.local:9092
broker.id=0
cruise.control.metrics.reporter.bootstrap.servers=kafka-all-broker.kafka.svc.cluster.local:9092
cruise.control.metrics.reporter.kubernetes.mode=true`,
					BrokerLabels: map[string]string{"apple": "tree"},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: nil,
										MatchFields: []corev1.NodeSelectorRequirement{
											{
												Key:      "apple",
												Operator: "in",
												Values:   []string{"fruit"},
											},
										},
									},
								},
							},
						},
					},
					PodSecurityContext:   &corev1.PodSecurityContext{},
					SecurityContext:      &corev1.SecurityContext{},
					BrokerIngressMapping: []string{"apple"},
					InitContainers: []corev1.Container{
						{
							Name:  "test-initcontainer",
							Image: "busybox:latest",
						},
						{
							Name:  "a-test-initcontainer",
							Image: "test/image:latest",
						},
					},
				},
			},
		},
	}

	for _, test := range testCases {
		config, err := GzipAndBase64BrokerConfiguration(&test.broker)
		if err != nil {
			t.Errorf("error should be nil, got: %v", err)
		}

		broker, err := GetBrokerFromBrokerConfigurationBackup(config)
		if err != nil {
			t.Errorf("error should be nil, got: %v", err)
		}

		if !reflect.DeepEqual(test.broker, broker) {
			t.Errorf("Expected: %v  Got: %v", test.broker, broker)
		}
	}
}
