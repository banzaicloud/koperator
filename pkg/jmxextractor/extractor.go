// Copyright © 2021 Banzai Cloud
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

package jmxextractor

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/util/kafka"

	"github.com/go-logr/logr"
)

const (
	headlessServiceJMXTemplate = "http://%s-%d." + kafka.HeadlessServiceTemplate + ".%s.svc.%s:%d"
	serviceJMXTemplate         = "http://%s-%d.%s.svc.%s:%d"
	versionRegexGroup          = "version"
)

var newJMXExtractor = createNewJMXExtractor
var jmxMetricRegex = regexp.MustCompile(
	fmt.Sprintf(`kafka_server_app_info_version{broker_id=\"[0-9]+\",version=\"(?P<%s>[0-9]+.[0-9]+.[0-9]+)\"`, versionRegexGroup))

type JMXExtractor interface {
	ExtractDockerImageAndVersion(brokerId int32, brokerConfig *v1beta1.BrokerConfig,
		clusterImage string, headlessServiceEnabled bool) (*v1beta1.KafkaVersion, error)
}

type jmxExtractor struct {
	JMXExtractor
	clusterNamespace        string
	kubernetesClusterDomain string
	clusterName             string
	log                     logr.Logger
}

func createNewJMXExtractor(namespace, kubernetesClusterDomain, clusterName string, log logr.Logger) JMXExtractor {
	return &jmxExtractor{
		clusterNamespace:        namespace,
		kubernetesClusterDomain: kubernetesClusterDomain,
		clusterName:             clusterName,
		log:                     log,
	}
}
func createMockJMXExtractor(namespace, kubernetesClusterDomain, clusterName string, log logr.Logger) JMXExtractor {
	return &mockJmxExtractor{}
}

func NewJMXExtractor(namespace, kubernetesClusterDomain, clusterName string, log logr.Logger) JMXExtractor {
	return newJMXExtractor(namespace, kubernetesClusterDomain, clusterName, log)
}

func NewMockJMXExtractor() {
	newJMXExtractor = createMockJMXExtractor
}

func (exp *jmxExtractor) ExtractDockerImageAndVersion(brokerId int32, brokerConfig *v1beta1.BrokerConfig,
	clusterImage string, headlessServiceEnabled bool) (*v1beta1.KafkaVersion, error) {
	var requestURL string
	if headlessServiceEnabled {
		requestURL =
			fmt.Sprintf(headlessServiceJMXTemplate,
				exp.clusterName, brokerId, exp.clusterName, exp.clusterNamespace,
				exp.kubernetesClusterDomain, 9020)
	} else {
		requestURL = fmt.Sprintf(serviceJMXTemplate, exp.clusterName, brokerId,
			exp.clusterNamespace, exp.kubernetesClusterDomain, 9020)
	}
	rsp, err := http.Get(requestURL)
	if err != nil {
		exp.log.Error(err, fmt.Sprintf("error during talking to broker-%d", brokerId))
		return nil, errorfactory.New(errorfactory.BrokersNotReady{}, err, "unable to talk to ...")
	}
	defer func() {
		closeErr := rsp.Body.Close()
		if closeErr != nil {
			exp.log.Error(closeErr, "could not close client")
		}
	}()

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	index := jmxMetricRegex.SubexpIndex(versionRegexGroup)
	var version string
	if index > -1 {
		if metrics := jmxMetricRegex.FindStringSubmatch(string(body)); len(metrics) > index {
			version = metrics[index]
		}
	}

	brokerImage := util.GetBrokerImage(brokerConfig, clusterImage)
	return &v1beta1.KafkaVersion{Version: version, Image: brokerImage}, nil
}
