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
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
	zookeeperutils "github.com/banzaicloud/kafka-operator/pkg/util/zookeeper"
	properties "github.com/banzaicloud/kafka-operator/properties/pkg"

	corev1 "k8s.io/api/core/v1"
)

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
	sslConf := generateSSLConfig(&r.KafkaCluster.Spec.ListenersConfig, clientPass, log)
	if sslConf.Len() != 0 {
		ccConfig.Merge(sslConf)
	}

	ccConfig.Sort()

	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			fmt.Sprintf(configAndVolumeNameTemplate, r.KafkaCluster.Name),
			util.MergeLabels(ccLabelSelector(r.KafkaCluster.Name), r.KafkaCluster.Labels),
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

func generateSSLConfig(l *v1beta1.ListenersConfig, clientPass string, log logr.Logger) *properties.Properties {
	sslConf := properties.NewProperties()

	if l.SSLSecrets != nil && util.IsSSLEnabledForInternalCommunication(l.InternalListeners) {
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

// generateCapacityConfig generates a CC capacity config with default values or returns the manually overridden value if it exists
func GenerateCapacityConfig(kafkaCluster *v1beta1.KafkaCluster, log logr.Logger, config *corev1.ConfigMap) string {
	log.Info("Generating capacity config")

	// If there is already a config added manually, use that one
	if kafkaCluster.Spec.CruiseControlConfig.CapacityConfig != "" {
		return kafkaCluster.Spec.CruiseControlConfig.CapacityConfig
	}
	// During cluster downscale the CR does not contain data for brokers being downscaled which is
	// required to generate the proper capacity json for CC so we are reusing the old one.
	// We can only remove brokers from capacity config when they were removed (pods deleted) from CC as well.
	if config != nil {
		if data, ok := config.Data["capacity.json"]; ok {
			return data
		}
	}

	capacityConfig := CapacityConfig{}

	for _, broker := range kafkaCluster.Spec.Brokers {
		brokerCapacity := BrokerCapacity{
			BrokerID: fmt.Sprintf("%d", broker.Id),
			Capacity: Capacity{
				DISK:  generateBrokerDisks(broker, kafkaCluster.Spec, log),
				CPU:   generateBrokerCPU(broker, kafkaCluster.Spec, log),
				NWIN:  generateBrokerNetworkIn(broker, kafkaCluster.Spec, log),
				NWOUT: generateBrokerNetworkOut(broker, kafkaCluster.Spec, log),
			},
			Doc: "Capacity unit used for disk is in MB, cpu is in percentage, network throughput is in KB.",
		}

		log.Info("The following brokerCapacity was generated", "brokerCapacity", brokerCapacity)

		capacityConfig.BrokerCapacities = append(capacityConfig.BrokerCapacities, brokerCapacity)
	}

	result, err := json.MarshalIndent(capacityConfig, "", "    ")
	if err != nil {
		log.Error(err, "Could not marshal cruise control capacity config")
	}
	log.Info(fmt.Sprintf("Generated capacity config was successful with values: %s", result))

	return string(result)
}

func generateBrokerNetworkIn(broker v1beta1.Broker, kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger) string {
	brokerConfig, err := util.GetBrokerConfig(broker, kafkaClusterSpec)
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
	brokerConfig, err := util.GetBrokerConfig(broker, kafkaClusterSpec)
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
	brokerConfig, err := util.GetBrokerConfig(broker, kafkaClusterSpec)
	if err != nil {
		log.V(warnLevel).Info("could not get cpu resource limits falling back to default value")
		return storageConfigCPUDefaultValue
	}

	return strconv.Itoa(int(brokerConfig.GetResources().Limits.Cpu().ScaledValue(-2)))
}

func generateBrokerDisks(brokerState v1beta1.Broker, kafkaClusterSpec v1beta1.KafkaClusterSpec, log logr.Logger) map[string]string {
	brokerDisks := map[string]string{}

	// Get disks from the BrokerConfigGroup if it's in use
	if brokerState.BrokerConfigGroup != "" {
		brokerConfigGroup := kafkaClusterSpec.BrokerConfigGroups[brokerState.BrokerConfigGroup]
		parseMountPathWithSize(brokerConfigGroup, log, brokerState, brokerDisks)
	}

	//Get disks from the BrokerConfig itself
	if brokerState.BrokerConfig != nil {
		parseMountPathWithSize(*brokerState.BrokerConfig, log, brokerState, brokerDisks)
	}

	return brokerDisks
}

func parseMountPathWithSize(brokerConfigGroup v1beta1.BrokerConfig, log logr.Logger, brokerState v1beta1.Broker, brokerDisks map[string]string) {
	for _, storageConfig := range brokerConfigGroup.StorageConfigs {
		int64Value, isConvertible := util.QuantityPointer(storageConfig.PvcSpec.Resources.Requests["storage"]).AsInt64()
		if isConvertible == false {
			log.Info("Could not convert 'storage' quantity to Int64 in brokerConfig for broker",
				"brokerId", brokerState.Id)
		}

		brokerDisks[storageConfig.MountPath+"/kafka"] = strconv.FormatInt(int64Value, 10)
	}
}
