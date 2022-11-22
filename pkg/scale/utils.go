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

package scale

import (
	"fmt"
	"strconv"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func brokerIDsFromStringSlice(brokerIDs []string) ([]int32, error) {
	brokers := make([]int32, len(brokerIDs))
	for idx, id := range brokerIDs {
		bid, err := strconv.Atoi(id)
		if err != nil {
			return nil, err
		}
		brokers[idx] = int32(bid)
	}
	return brokers, nil
}

func CruiseControlURLFromKafkaCluster(instance *v1beta1.KafkaCluster) string {
	if instance == nil {
		return ""
	}
	return CruiseControlURL(
		instance.Namespace,
		instance.Spec.GetKubernetesClusterDomain(),
		instance.Spec.CruiseControlConfig.CruiseControlEndpoint,
		instance.Name,
	)
}

func CruiseControlURL(namespace, domain, endpoint, name string) string {
	var url string
	if endpoint != "" {
		url = endpoint
	} else {
		url = fmt.Sprintf("%s-cruisecontrol-svc.%s.svc.%s:8090", name, namespace, domain)
	}
	return cruiseControlURL(url, false)
}

func cruiseControlURL(endpoint string, secure bool) string {
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s", scheme, endpoint, "kafkacruisecontrol")
}

func stringSliceToMap(s []string) map[string]bool {
	brokerIDsMap := make(map[string]bool, len(s))
	for _, id := range s {
		brokerIDsMap[id] = true
	}
	return brokerIDsMap
}

func kafkaBrokerStatesToMap(states ...KafkaBrokerState) map[KafkaBrokerState]bool {
	statesMap := make(map[KafkaBrokerState]bool, len(states))
	for _, state := range states {
		statesMap[state] = true
	}
	return statesMap
}
