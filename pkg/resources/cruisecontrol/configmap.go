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

package cruisecontrol

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"emperror.dev/errors"

	"github.com/go-logr/logr"
	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	apiutil "github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
	zookeeperutils "github.com/banzaicloud/koperator/pkg/util/zookeeper"
	properties "github.com/banzaicloud/koperator/properties/pkg"

	corev1 "k8s.io/api/core/v1"
)

const MinLogDirSizeInMB = int64(1)

func (r *Reconciler) configMap(clientPass, capacityConfig string, log logr.Logger) runtime.Object {
	ccConfig := properties.NewProperties()

	// Add base Cruise Control configuration
	conf, err := properties.NewFromString(r.KafkaCluster.Spec.CruiseControlConfig.Config)
	if err != nil {
		log.Error(err, "parsing Cruise Control configuration failed", "config", r.KafkaCluster.Spec.CruiseControlConfig.Config)
	}
	ccConfig.Merge(conf)

	bootstrapServers, err := kafkautils.GetBootstrapServersService(r.KafkaCluster)
	if err != nil {
		log.Error(err, "getting Kafka bootstrap servers for Cruise Control failed")
	}
	if err = ccConfig.Set("bootstrap.servers", bootstrapServers); err != nil {
		log.Error(err, "setting bootstrap.servers in Cruise Control configuration failed", "config", bootstrapServers)
	}

	// Add Zookeeper configuration
	zkConnect := zookeeperutils.PrepareConnectionAddress(r.KafkaCluster.Spec.ZKAddresses, r.KafkaCluster.Spec.GetZkPath())
	if err = ccConfig.Set("zookeeper.connect", zkConnect); err != nil {
		log.Error(err, "setting zookeeper.connect in Cruise Control configuration failed", "config", zkConnect)
	}

	// Add SSL configuration
	sslConf := generateSSLConfig(r.KafkaCluster.Spec, clientPass, log)
	if sslConf.Len() != 0 {
		ccConfig.Merge(sslConf)
	}

	ccConfig.Sort()

	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name),
			apiutil.MergeLabels(ccLabelSelector(r.KafkaCluster.Name), r.KafkaCluster.Labels),
			r.KafkaCluster,
		),
		Data: map[string]string{
			"cruisecontrol.properties": ccConfig.String(),
			"capacity.json":            capacityConfig,
			"clusterConfigs.json":      r.KafkaCluster.Spec.CruiseControlConfig.ClusterConfig,
			"log4j.properties":         r.KafkaCluster.Spec.CruiseControlConfig.GetCCLog4jConfig(),
		},
	}
	return configMap
}

func generateSSLConfig(k v1beta1.KafkaClusterSpec, clientPass string, log logr.Logger) *properties.Properties {
	sslConf := properties.NewProperties()

	if k.IsClientSSLSecretPresent() && util.IsSSLEnabledForInternalCommunication(k.ListenersConfig.InternalListeners) {
		if err := sslConf.Set("security.protocol", "SSL"); err != nil {
			log.Error(err, "settings security.protocol in Cruise Control configuration failed")
		}
		if err := sslConf.Set("ssl.truststore.location", keystoreVolumePath+"/"+v1alpha1.TLSJKSTrustStore); err != nil {
			log.Error(err, "settings ssl.truststore.location in Cruise Control configuration failed")
		}
		if err := sslConf.Set("ssl.keystore.location", keystoreVolumePath+"/"+v1alpha1.TLSJKSKeyStore); err != nil {
			log.Error(err, "settings ssl.keystore.location in Cruise Control configuration failed")
		}
		if err := sslConf.Set("ssl.keystore.password", clientPass); err != nil {
			log.Error(err, "settings ssl.keystore.password in Cruise Control configuration failed")
		}
		if err := sslConf.Set("ssl.truststore.password", clientPass); err != nil {
			log.Error(err, "settings ssl.truststore.password in Cruise Control configuration failed")
		}
	}
	return sslConf
}

const (
	storageConfigCPUDefaultValue   = "100"
	storageConfigNWINDefaultValue  = "125000"
	storageConfigNWOUTDefaultValue = "125000"
	defaultDoc                     = "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB."
)

type CapacityConfig struct {
	BrokerCapacities []BrokerCapacity `json:"brokerCapacities"`
}
type BrokerCapacity struct {
	BrokerID string   `json:"brokerId"`
	Capacity Capacity `json:"capacity"`
	Doc      string   `json:"doc"`
}
type Capacity struct {
	DISK  map[string]string `json:"DISK"`
	CPU   string            `json:"CPU"`
	NWIN  string            `json:"NW_IN"`
	NWOUT string            `json:"NW_OUT"`
}

type JBODInvariantCapacityConfig struct {
	Capacities []interface{} `json:"brokerCapacities"`
}

// GenerateCapacityConfig generates a CC capacity config with default values or returns the manually overridden value if it exists
func GenerateCapacityConfig(kafkaCluster *v1beta1.KafkaCluster, log logr.Logger, config *corev1.ConfigMap) (string, error) {
	var err error

	log.Info("generating capacity config")

	var capacityConfig JBODInvariantCapacityConfig
	var userConfigBrokerIds []string
	// If there is already a config added manually, use that one
	if kafkaCluster.Spec.CruiseControlConfig.CapacityConfig != "" {
		userProvidedCapacityConfig := kafkaCluster.Spec.CruiseControlConfig.CapacityConfig
		err := json.Unmarshal([]byte(userProvidedCapacityConfig), &capacityConfig)
		if err != nil {
			return "", errors.Wrap(err, "could not unmarshal the user-provided broker capacity config")
		}
		for _, brokerCapacity := range capacityConfig.Capacities {
			brokerCapacityMap, ok := brokerCapacity.(map[string]interface{})
			if !ok {
				continue
			}
			brokerId, ok, err := unstructured.NestedString(brokerCapacityMap, "brokerId")
			if err != nil {
				return "", errors.WrapIfWithDetails(err,
					"could retrieve broker Id from broker capacity configuration",
					"capacity configuration", brokerCapacityMap)
			}
			if !ok {
				continue
			}
			// If the -1 default exists we don't have to do anything else here since all brokers will have values.
			if brokerId == "-1" {
				log.Info("Using user provided capacity config because it has universal default defined", "capacity config", userProvidedCapacityConfig)
				return userProvidedCapacityConfig, nil
			}
			userConfigBrokerIds = append(userConfigBrokerIds, brokerId)
		}
	}
	// During cluster downscale the CR does not contain data for brokers being downscaled which is
	// required to generate the proper capacity json for CC so we are reusing the old one.
	// We can only remove brokers from capacity config when they were removed (pods deleted) from CC as well.
	if config != nil {
		if data, ok := config.Data["capacity.json"]; ok {
			return data, err
		}
	}

	// If there was no user provided config we shall generate all configuration or
	// adding generated values to all Brokers not provided by the user.
	brokerCapacities, err := appendGeneratedBrokerCapacities(kafkaCluster, log, userConfigBrokerIds)
	if err != nil {
		return "", err
	}

	capacityConfig.Capacities = append(capacityConfig.Capacities, brokerCapacities...)
	result, err := json.MarshalIndent(capacityConfig, "", "    ")
	if err != nil {
		return "", errors.WrapIf(err, "could not marshal cruise control capacity config")
	}
	log.Info("broker capacity config generated successfully", "capacity config", string(result))
	return string(result), err
}

func appendGeneratedBrokerCapacities(kafkaCluster *v1beta1.KafkaCluster, log logr.Logger, userConfigBrokerIds []string) ([]interface{}, error) {
	var brokerCapacities []interface{}

	brokerIdFromStatus := make([]string, 0, len(kafkaCluster.Status.BrokersState))
	for brokerId := range kafkaCluster.Status.BrokersState {
		brokerIdFromStatus = append(brokerIdFromStatus, brokerId)
	}
	// Since maps aren't ordered we need to order this list before using it
	sort.Strings(brokerIdFromStatus)

	for _, userConfigBrokerId := range userConfigBrokerIds {
		brokerIdFromStatus = util.StringSliceRemove(brokerIdFromStatus, userConfigBrokerId)
	}

	if len(brokerIdFromStatus) == 0 {
		return nil, nil
	}

	for _, brokerId := range brokerIdFromStatus {
		brokerCapacity := BrokerCapacity{}
		brokerFoundInSpec := false
		for _, broker := range kafkaCluster.Spec.Brokers {
			if brokerId == strconv.Itoa(int(broker.Id)) {
				brokerFoundInSpec = true
				brokerDisks, err := generateBrokerDisks(broker, kafkaCluster.Spec, log)
				if err != nil {
					return nil, errors.WrapIfWithDetails(err, "could not generate broker disks config for broker", "brokerID", broker.Id)
				}
				brokerCapacity = BrokerCapacity{
					BrokerID: strconv.Itoa(int(broker.Id)),
					Capacity: Capacity{
						DISK:  brokerDisks,
						CPU:   generateBrokerCPU(broker, kafkaCluster.Spec, log),
						NWIN:  generateBrokerNetworkIn(broker, kafkaCluster.Spec, log),
						NWOUT: generateBrokerNetworkOut(broker, kafkaCluster.Spec, log),
					},
					Doc: defaultDoc,
				}
			}
		}
		// When removing a broker it still needs to have values assigned in capacity config
		// although it doesn't really matter what the values are, so we are setting defaults
		// here, this way we don't have to deal with a universal default.
		if !brokerFoundInSpec {
			log.Info("broker spec not found, using default fallback")
			brokerCapacity = generateDefaultBrokerCapacityWithId(brokerId)
		}
		log.V(1).Info("capacity config successfully generated for broker", "capacity config", brokerCapacity)

		brokerCapacities = append(brokerCapacities, &brokerCapacity)
	}
	return brokerCapacities, nil
}

// Generate default broker capacity
// This value is used by every broker not in the spec, for example when deleting a broker
func generateDefaultBrokerCapacityWithId(brokerId string) BrokerCapacity {
	return BrokerCapacity{
		BrokerID: brokerId,
		Capacity: Capacity{
			DISK: map[string]string{
				"/kafka-logs/kafka": "10737",
			},
			CPU:   "100",
			NWIN:  "125000",
			NWOUT: "125000",
		},
		Doc: defaultDoc,
	}
}

func generateBrokerNetworkIn(broker v1beta1.Broker, kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger) string {
	brokerConfig, err := broker.GetBrokerConfig(kafkaClusterSpec)
	if err != nil {
		log.V(warnLevel).Info("could not get incoming network resource limits falling back to default value")
		return storageConfigNWINDefaultValue
	}
	if brokerConfig.NetworkConfig != nil && brokerConfig.NetworkConfig.IncomingNetworkThroughPut != "" {
		return brokerConfig.NetworkConfig.IncomingNetworkThroughPut
	}

	log.Info("incoming network throughput is not set falling back to default value")
	return storageConfigNWINDefaultValue
}

func generateBrokerNetworkOut(broker v1beta1.Broker, kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger) string {
	brokerConfig, err := broker.GetBrokerConfig(kafkaClusterSpec)
	if err != nil {
		log.V(warnLevel).Info("could not get outgoing network resource limits falling back to default value")
		return storageConfigNWOUTDefaultValue
	}
	if brokerConfig.NetworkConfig != nil && brokerConfig.NetworkConfig.OutgoingNetworkThroughPut != "" {
		return brokerConfig.NetworkConfig.OutgoingNetworkThroughPut
	}

	log.Info("outgoing network throughput is not set falling back to default value")
	return storageConfigNWOUTDefaultValue
}

func generateBrokerCPU(broker v1beta1.Broker, kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger) string {
	brokerConfig, err := broker.GetBrokerConfig(kafkaClusterSpec)
	if err != nil {
		log.V(warnLevel).Info("could not get cpu resource limits falling back to default value")
		return storageConfigCPUDefaultValue
	}

	return strconv.Itoa(int(brokerConfig.GetResources().Limits.Cpu().ScaledValue(-2)))
}

func generateBrokerDisks(brokerState v1beta1.Broker, kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger) (map[string]string, error) {
	storageConfigs := make(map[string]v1beta1.StorageConfig)

	// Get disks from the BrokerConfigGroup if it's in use
	if brokerState.BrokerConfigGroup != "" {
		if b, ok := kafkaClusterSpec.BrokerConfigGroups[brokerState.BrokerConfigGroup]; ok {
			for _, c := range b.StorageConfigs {
				storageConfigs[c.MountPath] = c
			}
		}
	}

	// Get disks from the BrokerConfig itself
	if brokerState.BrokerConfig != nil {
		for _, c := range brokerState.BrokerConfig.StorageConfigs {
			storageConfigs[c.MountPath] = c
		}
	}

	// Generate log dir configuration
	logDirs := make(map[string]string, len(storageConfigs))
	for path, conf := range storageConfigs {
		size := parseMountPathWithSize(conf)
		log.V(1).Info(fmt.Sprintf("broker log.dir %s size in MB: %d", path, size), "brokerId", brokerState.Id)

		if size < MinLogDirSizeInMB {
			return nil, errors.Errorf("broker log.dir %s size is %dMB which is less than the minimum %dMB",
				path, size, MinLogDirSizeInMB)
		}

		logDir := util.StorageConfigKafkaMountPath(path)
		logDirs[logDir] = fmt.Sprintf("%d", size)
	}

	return logDirs, nil
}

func parseMountPathWithSize(storage v1beta1.StorageConfig) int64 {
	var q *resource.Quantity
	if storage.PvcSpec != nil {
		q = util.QuantityPointer(storage.PvcSpec.Resources.Requests["storage"])
	} else if storage.EmptyDir != nil {
		q = storage.EmptyDir.SizeLimit
	}

	var tmpDec = inf.NewDec(0, 0)
	tmpDec.Round(q.AsDec(), -1*inf.Scale(resource.Mega), inf.RoundDown)

	return resource.NewQuantity(tmpDec.UnscaledBig().Int64(), q.Format).Value()
}
