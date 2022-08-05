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

package util

import (
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/istioingress"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestGenerateTLSConfigFromJKS(t *testing.T) {
	testCases := []struct {
		testName   string
		trustStore string
		keyStore   string
		password   string
	}{
		{
			testName:   "basic",
			trustStore: "/u3+7QAAAAIAAAABAAAAAgACY2EAAAGCWalckAAEWDUwOQAAAuUwggLhMIIByaADAgECAhQBCtt/LQEHzYIQlEIG7nD5elJFfjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDDAdSb290LWNhMB4XDTIyMDgwMTExMTE0NFoXDTMyMDcyOTExMTE0NFowEjEQMA4GA1UEAwwHUm9vdC1jYTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKU87zpK7Of8baSg7+rSomug3D8OTrfdLDgT2+T37V/PfI6pkhfbJklRrH7DGNYet2JtRUEGgcuFBxw+FWVX4rz9WQa/X+RTsyDLFgXzhXH8AaoJrM1P3FeA9ebTz5FjVCruswsZAvLAd2GPqTaARZZ2qJoc77iBJciC4Xyyh7gnhakMXuJlVCI8KBmQsZit1hu6Wb+4Zpd5nLtjZ2wQibNsc6JWsrFUmOQ25mz6VfI1OkURDwcuLputNURdo32G3muFLBMZ2c94ah7JrcQzJ//0OtK4gNsiCrX+1egF23ijdJo6gEm2PTNXh6KstyfSMGXLYXBcx8HbtjUWkvJJadcCAwEAAaMvMC0wDAYDVR0TBAUwAwEB/zAdBgNVHQ4EFgQUE+WFlp22jAK/ZKrXpLfxagwQlk4wDQYJKoZIhvcNAQELBQADggEBAG1S7X7O99Wk3w90/P9Me74J6+4CcjfinOz/6Rycz9KRnOXDZ+uwAaNS/A5T6NtJMNg08EVsiYK6QH1xRE1Pe4yhSOz0KCSBtLDs9cC96Xh/wIAiXRB/phFaLf4qWE+xlPu7sLG6vR3RTVmnfFsW5z8D0YhcMHlfOsrEjn6lhUAPNpNGe18O5k0hWNd7Wz740wBMf4oos2OQpUl8OpM3YMyhCWEEJWoWcnug/0R6e7odgWOa11jRN4/FLEzDM3jJ/oAhwqYPcqRtPYsxde2uju+iuqyr5GKcvtu9+YPWDGHIuojlXkprNpVEm1BakbdSbyNCy5npyETYV9KLygPI0qiAjppZCvaedhVh8lA7QMFyM2Sqsw==",
			keyStore:   "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",
			password:   "MjNZcGRpSzVoNlNFY2o5OQ==",
		},
	}

	for _, test := range testCases {
		// trustStore := make([]bytes,base64.StdEncoding.DecodedLen(len(test.trustStore)))
		// keyStore := make([]bytes,base64.StdEncoding.DecodedLen(len(test.keyStore)))
		// password := make([]bytes,base64.StdEncoding.DecodedLen(len(test.password)))
		trustStore, _ := base64.StdEncoding.DecodeString(test.trustStore)
		keyStore, _ := base64.StdEncoding.DecodeString(test.keyStore)
		password, _ := base64.StdEncoding.DecodeString(test.password)

		data := map[string][]byte{
			v1alpha1.TLSJKSTrustStore: trustStore,
			v1alpha1.TLSJKSKeyStore:   keyStore,
			v1alpha1.PasswordKey:      password,
		}
		_, _ = generateTLSConfigFromJKS(data)
	}

}

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
