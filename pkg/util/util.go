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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"dario.cat/mergo"
	"emperror.dev/errors"
	"github.com/IBM/sarama"
	clusterregv1alpha1 "github.com/cisco-open/cluster-registry-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientCtrl "sigs.k8s.io/controller-runtime/pkg/client"
	k8s_zap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/util/cert"
	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
	"github.com/banzaicloud/koperator/pkg/util/istioingress"
	properties "github.com/banzaicloud/koperator/properties/pkg"
)

const (
	symbolSet                         = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	IngressConfigGlobalName           = "globalConfig"
	ExternalListenerLabelNameTemplate = "%s-%s"
	ExternalListenerLabelNameKey      = "eListenerName"
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
	out = make(map[string]*string)
	for k, v := range in {
		out[k] = StringPointer(v)
	}
	return
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
		if listener.UsedForInnerBrokerCommunication && listener.Type.IsSSL() {
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
	brokerIDMap := make(map[int]struct{})

	// add brokers from spec
	for _, broker := range brokers {
		brokerIDMap[int(broker.Id)] = struct{}{}
	}

	// add brokers from status
	for brokerID := range brokerStatuses {
		id, err := strconv.Atoi(brokerID)
		if err != nil {
			log.Error(err, "could not parse brokerID properly")
			continue
		}
		brokerIDMap[id] = struct{}{}
	}

	// collect unique broker ids
	brokerIDs := make([]int, 0, len(brokerIDMap))
	for id := range brokerIDMap {
		brokerIDs = append(brokerIDs, id)
	}
	sort.Ints(brokerIDs)
	return brokerIDs
}

// IsIngressConfigInUse returns true if the provided ingressConfigName is bound to the given broker
func IsIngressConfigInUse(iConfigName, defaultConfigName string, cluster *v1beta1.KafkaCluster, log logr.Logger) bool {
	// Check if the global default is in use
	if iConfigName == IngressConfigGlobalName {
		return true
	}
	// Check if the given iConfigName is bound to a given broker
	for _, broker := range cluster.Spec.Brokers {
		brokerConfig, err := broker.GetBrokerConfig(cluster.Spec)
		if err != nil {
			log.Error(err, "could not determine if ingressConfig is in use")
			return false
		}
		if len(brokerConfig.BrokerIngressMapping) == 0 && iConfigName == defaultConfigName ||
			StringSliceContains(brokerConfig.BrokerIngressMapping, iConfigName) {
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

// ConstructEListenerLabelName construct an eListener label name based on ingress config name and listener name
func ConstructEListenerLabelName(ingressConfigName, eListenerName string) string {
	if ingressConfigName == IngressConfigGlobalName {
		return eListenerName
	}

	return fmt.Sprintf(ExternalListenerLabelNameTemplate, eListenerName, ingressConfigName)
}

// ShouldIncludeBroker returns true if the broker should be included as a resource on external listener resources
func ShouldIncludeBroker(brokerConfig *v1beta1.BrokerConfig, status v1beta1.KafkaClusterStatus, brokerID int,
	defaultIngressConfigName, ingressConfigName string) bool {
	if brokerConfig != nil {
		if len(brokerConfig.BrokerIngressMapping) == 0 && (ingressConfigName == defaultIngressConfigName || defaultIngressConfigName == "") ||
			StringSliceContains(brokerConfig.BrokerIngressMapping, ingressConfigName) {
			return true
		}
	}
	if brokerState, ok := status.BrokersState[strconv.Itoa(brokerID)]; ok {
		if StringSliceContains(brokerState.ExternalListenerConfigNames, ingressConfigName) {
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

// GetBrokerImage returns the used broker image
func GetBrokerImage(brokerConfig *v1beta1.BrokerConfig, clusterImage string) string {
	if brokerConfig.Image != "" {
		return brokerConfig.Image
	}
	return clusterImage
}

// GetBrokerMetricsReporterImage returns the image used for cruise-control metrics reporter
func GetBrokerMetricsReporterImage(brokerConfig *v1beta1.BrokerConfig, kafkaClusterSpec v1beta1.KafkaClusterSpec) string {
	if brokerConfig.MetricsReporterImage != "" {
		return brokerConfig.MetricsReporterImage
	}
	return kafkaClusterSpec.GetClusterMetricsReporterImage()
}

// getRandomString returns a random string containing uppercase, lowercase and number characters with the length given
func GetRandomString(length int) (string, error) {
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

func GenerateEnvoyResourceName(resourceNameFormat string, resourceNameWithScopeFormat string, extListener v1beta1.ExternalListenerConfig, ingressConfig v1beta1.IngressConfig,
	ingressConfigName, clusterName string) string {
	var resourceName string
	if ingressConfigName == IngressConfigGlobalName {
		resourceName = fmt.Sprintf(resourceNameFormat, extListener.Name, clusterName)
	} else {
		resourceName = fmt.Sprintf(resourceNameWithScopeFormat, extListener.Name, ingressConfigName, clusterName)
	}

	return resourceName
}

func StorageConfigKafkaMountPath(mountPath string) string {
	return mountPath + "/kafka"
}

func GetClientTLSConfig(client clientCtrl.Reader, secretNamespaceName types.NamespacedName) (*tls.Config, error) {
	tlsKeys := &corev1.Secret{}
	err := client.Get(context.TODO(),
		types.NamespacedName{
			Namespace: secretNamespaceName.Namespace,
			Name:      secretNamespaceName.Name,
		},
		tlsKeys,
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = errorfactory.New(errorfactory.ResourceNotReady{}, err, "client secret not found")
		}
		return nil, err
	}

	config, err := CreateTLSConfigFromSecret(tlsKeys)
	if err != nil {
		return nil, errorfactory.New(errorfactory.InternalError{}, err, fmt.Sprintf("could not get client certificate for kafka client, name: %s namespace: %s", secretNamespaceName.Name, secretNamespaceName.Namespace))
	}
	return config, nil
}

func CreateTLSConfigFromSecret(tlsKeys *corev1.Secret) (*tls.Config, error) {
	rootCAs := x509.NewCertPool()
	if err := cert.CheckSSLCertSecret(tlsKeys); err != nil {
		return nil, err
	}
	tlsCert, err := cert.ParseKeyStoreToTLSCertificate(tlsKeys.Data[v1alpha1.TLSJKSKeyStore], tlsKeys.Data[v1alpha1.PasswordKey])
	if err != nil {
		return nil, errors.WrapIf(err, "couldn't parse keystore")
	}
	caCerts, err := cert.ParseTrustStoreToCaChain(tlsKeys.Data[v1alpha1.TLSJKSTrustStore], tlsKeys.Data[v1alpha1.PasswordKey])
	if err != nil {
		return nil, errors.WrapIf(err, "couldn't parse truststore")
	}
	for i := range caCerts {
		rootCAs.AddCert(caCerts[i])
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      rootCAs,
	}
	return config, nil
}

func ObjectManagedByClusterRegistry(object metav1.Object) bool {
	annotations := object.GetAnnotations()
	_, ok := annotations[clusterregv1alpha1.OwnershipAnnotation]
	return ok
}

func GzipAndBase64BrokerConfiguration(broker *v1beta1.Broker) (string, error) {
	if broker == nil {
		return "", nil
	}
	configJSON, err := json.Marshal(broker)
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	gz := gzip.NewWriter(&buff)
	if _, err := gz.Write(configJSON); err != nil {
		return "", err
	}
	gz.Close()
	return base64.StdEncoding.EncodeToString(buff.Bytes()), nil
}

func GetBrokerFromBrokerConfigurationBackup(config string) (v1beta1.Broker, error) {
	if config == "" {
		return v1beta1.Broker{}, errors.New("broker configurationBackup is empty")
	}
	configGzip, err := base64.StdEncoding.DecodeString(config)
	if err != nil {
		return v1beta1.Broker{}, err
	}
	reader := bytes.NewBuffer(configGzip)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return v1beta1.Broker{}, err
	}
	configJSON, err := io.ReadAll(gzipReader)
	if err != nil {
		return v1beta1.Broker{}, err
	}
	gzipReader.Close()
	broker := v1beta1.Broker{}
	if err := json.Unmarshal(configJSON, &broker); err != nil {
		return v1beta1.Broker{}, err
	}

	return broker, nil
}

// start with 20 ms, multiply by 2.5 each step, 6 steps = ~3 seconds
var DefaultBackOffForConflict wait.Backoff = wait.Backoff{
	Duration: 20 * time.Millisecond,
	Jitter:   0,
	Factor:   2.5,
	Steps:    6,
}

func RetryOnError(backoff wait.Backoff, fn func() error, isRetryableError func(error) bool) error {
	var lastErr error
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := fn()
		switch {
		case err == nil:
			return true, nil
		case isRetryableError(err):
			lastErr = err
			return false, nil
		default:
			return false, err
		}
	})

	if wait.Interrupted(err) {
		err = lastErr
	}
	return err
}

func RetryOnConflict(backoff wait.Backoff, fn func() error) error {
	return RetryOnError(backoff, fn, apierrors.IsConflict)
}

func GetExternalPortForBroker(externalStartingPort, brokerId int32) int32 {
	return externalStartingPort + brokerId
}
