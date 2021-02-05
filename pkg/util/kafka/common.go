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

package kafka

import (
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	properties "github.com/banzaicloud/kafka-operator/properties/pkg"
	"github.com/go-logr/logr"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

const (
	// AllBrokerServiceTemplate template for Kafka headless service
	AllBrokerServiceTemplate = "%s-all-broker"
	// HeadlessServiceTemplate template for Kafka headless service
	HeadlessServiceTemplate = "%s-headless"

	// property name in the ConfigMap's Data field for the broker configuration
	ConfigPropertyName            = "broker-config"
	securityProtocolMapConfigName = "listener.security.protocol.map"
)

// PerBrokerConfigs configurations will not trigger rolling upgrade when updated
var PerBrokerConfigs = []string{
	// currently hardcoded in configmap.go
	"ssl.client.auth",

	// listener related config change will trigger rolling upgrade anyways due to pod spec change
	"listeners",
	"advertised.listeners",

	securityProtocolMapConfigName,
}

// LabelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func LabelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}

// commonAclString is the raw representation of an ACL allowing Describe on a Topic
var commonAclString = "User:%s,Topic,%s,%s,Describe,Allow,*"

// createAclString is the raw representation of an ACL allowing Create on a Topic
var createAclString = "User:%s,Topic,%s,%s,Create,Allow,*"

// writeAclString is the raw representation of an ACL allowing Write on a Topic
var writeAclString = "User:%s,Topic,%s,%s,Write,Allow,*"

// readAclString is the raw representation of an ACL allowing Read on a Topic
var readAclString = "User:%s,Topic,%s,%s,Read,Allow,*"

// readGroupAclString is the raw representation of an ACL allowing Read on ConsumerGroups
var readGroupAclString = "User:%s,Group,LITERAL,*,Read,Allow,*"

// GrantsToACLStrings converts a user DN and a list of topic grants to raw strings
// for a CR status
func GrantsToACLStrings(dn string, grants []v1alpha1.UserTopicGrant) []string {
	acls := make([]string, 0)
	for _, x := range grants {
		if x.PatternType == "" {
			x.PatternType = v1alpha1.KafkaPatternTypeDefault
		}
		patternType := strings.ToUpper(string(x.PatternType))
		cmn := fmt.Sprintf(commonAclString, dn, patternType, x.TopicName)
		if !util.StringSliceContains(acls, cmn) {
			acls = append(acls, cmn)
		}
		switch x.AccessType {
		case v1alpha1.KafkaAccessTypeRead:
			readAcl := fmt.Sprintf(readAclString, dn, patternType, x.TopicName)
			readGroupAcl := fmt.Sprintf(readGroupAclString, dn)
			for _, y := range []string{readAcl, readGroupAcl} {
				if !util.StringSliceContains(acls, y) {
					acls = append(acls, y)
				}
			}
		case v1alpha1.KafkaAccessTypeWrite:
			createAcl := fmt.Sprintf(createAclString, dn, patternType, x.TopicName)
			writeAcl := fmt.Sprintf(writeAclString, dn, patternType, x.TopicName)
			for _, y := range []string{createAcl, writeAcl} {
				if !util.StringSliceContains(acls, y) {
					acls = append(acls, y)
				}
			}
		}
	}
	return acls
}

func ShouldRefreshOnlyPerBrokerConfigs(currentConfigs, desiredConfigs *properties.Properties, log logr.Logger) bool {
	// Get the diff of the configuration
	configDiff := currentConfigs.Diff(desiredConfigs)

	// Return if there is no drift in the configuration
	if len(configDiff) == 0 {
		return true
	}

	log.V(1).Info("configs have been changed", "configs", configDiff)

	if diff, ok := configDiff[securityProtocolMapConfigName]; ok {
		if listenersSecurityProtocolChanged(diff[0].Value(), diff[1].Value()) {
			return false
		}
	}

	for _, perBrokerConfig := range PerBrokerConfigs {
		delete(configDiff, perBrokerConfig)
	}

	return len(configDiff) == 0
}

// Security protocol cannot be updated for existing listener
// a rolling upgrade should be triggered in this case
func listenersSecurityProtocolChanged(current, desired string) bool {
	// added or deleted config is ok
	if current == "" || desired == "" {
		return false
	}
	currentConfig := newListenerSecurityProtocolMap(current)
	desiredConfig := newListenerSecurityProtocolMap(desired)

	if len(currentConfig) != len(desiredConfig) {
		return true
	}

	for dKey, dConf := range desiredConfig {
		if cConf, ok := currentConfig[dKey]; ok && cConf != dConf {
			return true
		}
	}
	return false
}

type listenerSecurityProtocolMap map[string]string

func newListenerSecurityProtocolMap(s string) listenerSecurityProtocolMap {
	listenerSecProtoMap := make(listenerSecurityProtocolMap)

	for _, listenerConfig := range strings.Split(s, ",") {
		listenerProto := strings.SplitN(listenerConfig, ":", 2)
		// listenerProto must have 2 parts and it is considered as invalid if it does not.
		if len(listenerProto) != 2 {
			continue
		}
		listenerSecProtoMap[strings.TrimSpace(listenerProto[0])] = strings.TrimSpace(listenerProto[1])
	}

	return listenerSecProtoMap
}

const (
	// BrokerHostnameTemplate defines the hostname template for Kafka brokers in the following format:
	// 	<KAFKA_CLUSTER_NAME>-<BROKER_ID>
	BrokerHostnameTemplate = "%s-%d"
	// BrokerHostnameTemplate defines the domain template for Kafka brokers in the following format:
	// 	<K8S_NAMESPACE>.svc.<K8S_CLUSTER_DOMAIN>
	ServiceDomainNameTemplate = "%s.svc.%s"
)

func GetClusterServiceDomainName(cluster *v1beta1.KafkaCluster) string {
	return fmt.Sprintf(ServiceDomainNameTemplate, cluster.Namespace, cluster.Spec.GetKubernetesClusterDomain())
}

func GetBrokerServiceFqdn(cluster *v1beta1.KafkaCluster, broker *v1beta1.Broker) string {
	hostname := fmt.Sprintf(BrokerHostnameTemplate, cluster.Name, broker.Id)
	svcDomainName := GetClusterServiceDomainName(cluster)
	return fmt.Sprintf("%s.%s", hostname, svcDomainName)
}

func GetClusterServiceFqdn(cluster *v1beta1.KafkaCluster) string {
	tmpl := AllBrokerServiceTemplate
	if cluster.Spec.HeadlessServiceEnabled {
		tmpl = HeadlessServiceTemplate
	}
	return fmt.Sprintf("%s.%s",
		fmt.Sprintf(tmpl, cluster.Name),
		GetClusterServiceDomainName(cluster))
}

func GetBootstrapServers(cluster *v1beta1.KafkaCluster) (string, error) {
	return getBootstrapServers(cluster, false)
}

func GetBootstrapServersService(cluster *v1beta1.KafkaCluster) (string, error) {
	return getBootstrapServers(cluster, true)
}

func getBootstrapServers(cluster *v1beta1.KafkaCluster, useService bool) (string, error) {
	var listener v1beta1.InternalListenerConfig
	var bootstrapServersList []string

	for _, lc := range cluster.Spec.ListenersConfig.InternalListeners {
		if lc.UsedForInnerBrokerCommunication && !lc.UsedForControllerCommunication {
			listener = lc
			break
		}
	}

	if listener.Name == "" {
		return "", errors.New("no suitable listener found for using as Kafka bootstrap server configuration")
	}

	if useService {
		bootstrapServersList = append(bootstrapServersList,
			fmt.Sprintf("%s:%d", GetClusterServiceFqdn(cluster), listener.ContainerPort))
	} else {
		for _, broker := range cluster.Spec.Brokers {
			fqdn := GetBrokerServiceFqdn(cluster, &broker)
			bootstrapServersList = append(bootstrapServersList,
				fmt.Sprintf("%s:%d", fqdn, listener.ContainerPort))
		}
	}
	return strings.Join(bootstrapServersList, ","), nil
}

// GatherBrokerConfigIfAvailable return the brokerConfig for a specific Id if available
func GatherBrokerConfigIfAvailable(kafkaClusterSpec v1beta1.KafkaClusterSpec, brokerId int) (*v1beta1.BrokerConfig, error) {
	brokerConfig := &v1beta1.BrokerConfig{}
	brokerIdPresent := false
	var requiredBroker v1beta1.Broker
	// This check is used in case of broker delete. In case of broker delete there is some time when the CC removes the broker
	// gracefully which means we have to generate the port for that broker as well. At that time the status contains
	// but the broker spec does not contain the required config values.
	for _, broker := range kafkaClusterSpec.Brokers {
		if int(broker.Id) == brokerId {
			brokerIdPresent = true
			requiredBroker = broker
			break
		}
	}
	if brokerIdPresent {
		var err error
		brokerConfig, err = util.GetBrokerConfig(requiredBroker, kafkaClusterSpec)
		if err != nil {
			return nil, err
		}
	}
	return brokerConfig, nil
}
