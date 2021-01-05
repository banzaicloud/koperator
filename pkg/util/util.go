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
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/Shopify/sarama"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8s_zap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	envoyutils "github.com/banzaicloud/kafka-operator/pkg/util/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/util/istioingress"
	properties "github.com/banzaicloud/kafka-operator/properties/pkg"
)

const (
	symbolSet               = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	IngressConfigGlobalName = "globalConfig"
)

// IntstrPointer generate IntOrString pointer from int
func IntstrPointer(i int) *intstr.IntOrString {
	is := intstr.FromInt(i)
	return &is
}

// Int64Pointer generates int64 pointer from int64
func Int64Pointer(i int64) *int64 {
	return &i
}

// Int32Pointer generates int32 pointer from int32
func Int32Pointer(i int32) *int32 {
	return &i
}

// BoolPointer generates bool pointer from bool
func BoolPointer(b bool) *bool {
	return &b
}

// IntPointer generates int pointer from int
func IntPointer(i int) *int {
	return &i
}

// StringPointer generates string pointer from string
func StringPointer(s string) *string {
	return &s
}

// QuantityPointer generates Quantity pointer from Quantity
func QuantityPointer(q resource.Quantity) *resource.Quantity {
	return &q
}

// MapStringStringPointer generates a map[string]*string
func MapStringStringPointer(in map[string]string) (out map[string]*string) {
	out = make(map[string]*string, 0)
	for k, v := range in {
		out[k] = StringPointer(v)
	}
	return
}

// MergeLabels merges two given labels
func MergeLabels(l ...map[string]string) map[string]string {
	res := make(map[string]string)

	for _, v := range l {
		for lKey, lValue := range v {
			res[lKey] = lValue
		}
	}
	return res
}

func MergeAnnotations(annotations ...map[string]string) map[string]string {
	rtn := make(map[string]string)
	for _, a := range annotations {
		for k, v := range a {
			rtn[k] = v
		}
	}

	return rtn
}

// ConvertStringToInt32 converts the given string to int32
func ConvertStringToInt32(s string) int32 {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return -1
	}
	return int32(i)
}

// IsSSLEnabledForInternalCommunication checks if ssl is enabled for internal communication
func IsSSLEnabledForInternalCommunication(l []v1beta1.InternalListenerConfig) (enabled bool) {

	for _, listener := range l {
		if strings.ToLower(listener.Type) == "ssl" {
			enabled = true
			break
		}
	}
	return enabled
}

// ConvertPropertiesToMapStringPointer converts a Properties object to map[string]*string
func ConvertPropertiesToMapStringPointer(pp *properties.Properties) map[string]*string {

	result := make(map[string]*string, pp.Len())
	for _, key := range pp.Keys() {
		if p, ok := pp.Get(key); ok {
			result[key] = StringPointer(p.Value())
		}
	}
	return result
}

// StringSliceContains returns true if list contains s
func StringSliceContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// StringSliceRemove will remove s from list
func StringSliceRemove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func AreStringSlicesIdentical(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	return reflect.DeepEqual(a, b)
}

// returns the union of the ids of the configured (Spec.Brokers) and the running (BrokerState) brokers
func GetBrokerIdsFromStatusAndSpec(brokerStatuses map[string]v1beta1.BrokerState, brokers []v1beta1.Broker, log logr.Logger) []int {
	brokerIdMap := make(map[int]struct{})

	// add brokers from spec
	for _, broker := range brokers {
		brokerIdMap[int(broker.Id)] = struct{}{}
	}

	// add brokers from status
	for brokerId := range brokerStatuses {
		id, err := strconv.Atoi(brokerId)
		if err != nil {
			log.Error(err, "could not parse brokerId properly")
			continue
		}
		brokerIdMap[id] = struct{}{}
	}

	// collect unique broker ids
	brokerIds := make([]int, 0, len(brokerIdMap))
	for id := range brokerIdMap {
		brokerIds = append(brokerIds, id)
	}
	sort.Ints(brokerIds)
	return brokerIds
}

// IsIngressConfigInUse returns true if the provided ingressConfigName is bound to the given broker
func IsIngressConfigInUse(iConfigName string, cluster *v1beta1.KafkaCluster, log logr.Logger) bool {
	// Check if the global default is in use
	if iConfigName == IngressConfigGlobalName {
		return true
	}
	// Check if the given iConfigName is bound to a given broker
	for _, broker := range cluster.Spec.Brokers {
		brokerConfig, err := GetBrokerConfig(broker, cluster.Spec)
		if err != nil {
			log.Error(err, "could not determine if ingressConfig is in use")
			return false
		}
		if StringSliceContains(brokerConfig.BrokerIdBindings, iConfigName) {
			return true
		}
	}
	for _, status := range cluster.Status.BrokersState {
		if StringSliceContains(status.ExternalListenerConfigNames, iConfigName) {
			return true
		}
	}
	return false
}

// GetIngressConfigs compose the ingress configuration for a given externalListener
func GetIngressConfigs(kafkaClusterSpec v1beta1.KafkaClusterSpec,
	eListenerConfig v1beta1.ExternalListenerConfig) (map[string]v1beta1.IngressConfig, string, error) {
	var ingressConfigs map[string]v1beta1.IngressConfig
	var defaultIngressConfigName string
	// Merge specific external listener configuration with the global one if none specified
	switch kafkaClusterSpec.GetIngressController() {
	case envoyutils.IngressControllerName:
		if eListenerConfig.Config != nil {
			defaultIngressConfigName = eListenerConfig.Config.DefaultIngressConfig
			ingressConfigs = make(map[string]v1beta1.IngressConfig, len(eListenerConfig.Config.IngressConfig))
			for k, iConf := range eListenerConfig.Config.IngressConfig {
				if iConf.EnvoyConfig != nil {
					err := mergo.Merge(iConf.EnvoyConfig, kafkaClusterSpec.EnvoyConfig)
					if err != nil {
						return nil, "", errors.WrapWithDetails(err,
							"could not merge global envoy config with local one", "envoyConfig", k)
					}
					err = mergo.Merge(&iConf.IngressServiceSettings, eListenerConfig.IngressServiceSettings)
					if err != nil {
						return nil, "", errors.WrapWithDetails(err,
							"could not merge global loadbalancer config with local one",
							"externalListenerName", eListenerConfig.Name)
					}
					ingressConfigs[k] = iConf
				}
			}
		} else {
			ingressConfigs = map[string]v1beta1.IngressConfig{
				IngressConfigGlobalName: {
					IngressServiceSettings: eListenerConfig.IngressServiceSettings,
					EnvoyConfig:            &kafkaClusterSpec.EnvoyConfig,
				},
			}
		}
	case istioingress.IngressControllerName:
		if eListenerConfig.Config != nil {
			defaultIngressConfigName = eListenerConfig.Config.DefaultIngressConfig
			ingressConfigs = make(map[string]v1beta1.IngressConfig, len(eListenerConfig.Config.IngressConfig))
			for k, iConf := range eListenerConfig.Config.IngressConfig {
				if iConf.IstioIngressConfig != nil {
					err := mergo.Merge(iConf.IstioIngressConfig, kafkaClusterSpec.IstioIngressConfig)
					if err != nil {
						return nil, "", errors.WrapWithDetails(err,
							"could not merge global istio config with local one", "istioConfig", k)
					}
					err = mergo.Merge(&iConf.IngressServiceSettings, eListenerConfig.IngressServiceSettings)
					if err != nil {
						return nil, "", errors.WrapWithDetails(err,
							"could not merge global loadbalancer config with local one",
							"externalListenerName", eListenerConfig.Name)
					}
					ingressConfigs[k] = iConf
				}
			}
		} else {
			ingressConfigs = map[string]v1beta1.IngressConfig{
				IngressConfigGlobalName: {
					IngressServiceSettings: eListenerConfig.IngressServiceSettings,
					IstioIngressConfig:     &kafkaClusterSpec.IstioIngressConfig,
				},
			}
		}
	default:
		return nil, "", errors.NewWithDetails("not supported ingress type", "name", kafkaClusterSpec.GetIngressController())
	}
	return ingressConfigs, defaultIngressConfigName, nil
}

// GetBrokerConfig compose the brokerConfig for a given broker
func GetBrokerConfig(broker v1beta1.Broker, clusterSpec v1beta1.KafkaClusterSpec) (*v1beta1.BrokerConfig, error) {

	bConfig := &v1beta1.BrokerConfig{}
	if broker.BrokerConfigGroup == "" {
		return broker.BrokerConfig, nil
	} else if broker.BrokerConfig != nil {
		bConfig = broker.BrokerConfig.DeepCopy()
	}

	groupConfig, exists := clusterSpec.BrokerConfigGroups[broker.BrokerConfigGroup]
	if !exists {
		return nil, errors.NewWithDetails("missing brokerConfigGroup", "key", broker.BrokerConfigGroup)
	}

	dstAffinity := &corev1.Affinity{}
	srcAffinity := &corev1.Affinity{}

	if groupConfig.Affinity != nil {
		dstAffinity = groupConfig.Affinity.DeepCopy()
	}
	if bConfig.Affinity != nil {
		srcAffinity = bConfig.Affinity
	}

	if err := mergo.Merge(dstAffinity, srcAffinity, mergo.WithOverride); err != nil {
		return nil, errors.WrapIf(err, "could not merge brokerConfig.Affinity with ConfigGroup.Affinity")
	}

	err := mergo.Merge(bConfig, groupConfig, mergo.WithAppendSlice)
	if err != nil {
		return nil, errors.WrapIf(err, "could not merge brokerConfig with ConfigGroup")
	}

	bConfig.StorageConfigs = dedupStorageConfigs(bConfig.StorageConfigs)

	if groupConfig.Affinity != nil || bConfig.Affinity != nil {
		bConfig.Affinity = dstAffinity
	}

	return bConfig, nil
}

func dedupStorageConfigs(elements []v1beta1.StorageConfig) []v1beta1.StorageConfig {
	encountered := make(map[string]struct{})
	result := make([]v1beta1.StorageConfig, 0)

	for _, v := range elements {
		if _, ok := encountered[v.MountPath]; !ok {
			encountered[v.MountPath] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}

// GetBrokerImage returns the used broker image
func GetBrokerImage(brokerConfig *v1beta1.BrokerConfig, clusterImage string) string {
	if brokerConfig.Image != "" {
		return brokerConfig.Image
	}
	return clusterImage
}

// getRandomString returns a random string containing uppercase, lowercase and number characters with the length given
func GetRandomString(length int) (string, error) {
	rand.Seed(time.Now().UnixNano())

	chars := []rune(symbolSet)

	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String(), nil
}

// computes the max between 2 ints
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func CreateLogger(debug bool, development bool) logr.Logger {
	// create encoder config
	var config zapcore.EncoderConfig
	if development {
		config = zap.NewDevelopmentEncoderConfig()
	} else {
		config = zap.NewProductionEncoderConfig()
	}
	// set human readable timestamp format regardless whether development mode is on
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	// create the encoder
	var encoder zapcore.Encoder
	if development {
		encoder = zapcore.NewConsoleEncoder(config)
	} else {
		encoder = zapcore.NewJSONEncoder(config)
	}

	// set the log level
	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	return k8s_zap.New(k8s_zap.UseDevMode(development), k8s_zap.Encoder(encoder), k8s_zap.Level(level))
}

// ConvertConfigEntryListToProperties function takes []sarama.ConfigEntry and coverts it to Properties object
func ConvertConfigEntryListToProperties(config []sarama.ConfigEntry) (*properties.Properties, error) {
	p := properties.NewProperties()

	for _, c := range config {
		err := p.Set(c.Name, c.Value)
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}
