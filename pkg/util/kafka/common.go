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
	"github.com/go-logr/logr"
	"strings"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

const (
	// AllBrokerServiceTemplate template for Kafka headless service
	AllBrokerServiceTemplate = "%s-all-broker"
	// HeadlessServiceTemplate template for Kafka headless service
	HeadlessServiceTemplate = "%s-headless"
)

const securityProtocolMapConfigName = "listener.security.protocol.map"

// these configurations will not trigger rolling upgrade when updated
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

// reacAclString is the raw representation of an ACL allowing Read on a Topic
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

func ShouldRefreshOnlyPerBrokerConfigs(currentConfigs, desiredConfigs map[string]string, log logr.Logger) bool {
	touchedConfigs := collectTouchedConfigs(currentConfigs, desiredConfigs, log)

	if _, ok := touchedConfigs[securityProtocolMapConfigName]; ok {
		if listenersSecurityProtocolChanged(currentConfigs, desiredConfigs) {
			return false
		}
	}

	for _, perBrokerConfig := range PerBrokerConfigs {
		delete(touchedConfigs, perBrokerConfig)
	}

	return len(touchedConfigs) == 0
}

// Security protocol cannot be updated for existing listener
// a rolling upgrade should be triggered in this case
func listenersSecurityProtocolChanged(currentConfigs, desiredConfigs map[string]string) bool {
	// added or deleted config is ok
	if currentConfigs[securityProtocolMapConfigName] == "" || desiredConfigs[securityProtocolMapConfigName] == "" {
		return false
	}
	currentListenerProtocolMap := make(map[string]string)
	for _, listenerConfig := range strings.Split(currentConfigs[securityProtocolMapConfigName], ",") {
		listenerProtocol := strings.Split(listenerConfig, ":")
		if len(listenerProtocol) != 2 {
			continue
		}
		currentListenerProtocolMap[strings.TrimSpace(listenerProtocol[0])] = strings.TrimSpace(listenerProtocol[1])
	}
	for _, listenerConfig := range strings.Split(desiredConfigs[securityProtocolMapConfigName], ",") {
		desiredListenerProtocol := strings.Split(listenerConfig, ":")
		if len(desiredListenerProtocol) != 2 {
			continue
		}
		if currentListenerProtocolValue, ok := currentListenerProtocolMap[strings.TrimSpace(desiredListenerProtocol[0])]; ok {
			if currentListenerProtocolValue != strings.TrimSpace(desiredListenerProtocol[1]) {
				return true
			}
		}
	}
	return false
}

// collects are the config keys that are either added, removed or updated
// between the current and the desired ConfigMap
func collectTouchedConfigs(currentConfigs, desiredConfigs map[string]string, log logr.Logger) map[string]struct{} {
	touchedConfigs := make(map[string]struct{})

	currentConfigsCopy := make(map[string]string)
	for k, v := range currentConfigs {
		currentConfigsCopy[k] = v
	}

	for configName, desiredValue := range desiredConfigs {
		if currentValue, ok := currentConfigsCopy[configName]; !ok || currentValue != desiredValue {
			// new or updated config
			touchedConfigs[configName] = struct{}{}
		}
		delete(currentConfigsCopy, configName)
	}

	for configName := range currentConfigsCopy {
		// deleted config
		touchedConfigs[configName] = struct{}{}
	}

	log.V(1).Info("configs have been changed", "configs", touchedConfigs)
	return touchedConfigs
}
