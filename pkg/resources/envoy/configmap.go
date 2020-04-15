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

package envoy

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/duration"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	ptypesstruct "github.com/golang/protobuf/ptypes/struct"
	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) configMap(log logr.Logger) runtime.Object {
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(envoyVolumeAndConfigName, labelSelector, r.KafkaCluster),
		Data:       map[string]string{"envoy.yaml": generateEnvoyConfig(r.KafkaCluster, log)},
	}
	return configMap
}

func generateEnvoyConfig(kc *v1beta1.KafkaCluster, log logr.Logger) string {
	//TODO support multiple external listener by removing [0] (baluchicken)
	adminConfig := envoybootstrap.Admin{
		AccessLogPath: "/tmp/admin_access.log",
		Address: &envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: 9901,
					},
				},
			},
		},
	}

	var listeners []*envoyapi.Listener
	var clusters []*envoyapi.Cluster

	for _, broker := range kc.Spec.Brokers {
		listeners = append(listeners, &envoyapi.Listener{
			Address: &envoycore.Address{
				Address: &envoycore.Address_SocketAddress{
					SocketAddress: &envoycore.SocketAddress{
						Address: "0.0.0.0",
						PortSpecifier: &envoycore.SocketAddress_PortValue{
							PortValue: uint32(kc.Spec.ListenersConfig.ExternalListeners[0].ExternalStartingPort + broker.Id),
						},
					},
				},
			},
			FilterChains: []*envoylistener.FilterChain{
				{
					Filters: []*envoylistener.Filter{
						{
							Name: wellknown.TCPProxy,
							ConfigType: &envoylistener.Filter_Config{
								Config: &ptypesstruct.Struct{
									Fields: map[string]*ptypesstruct.Value{
										"stat_prefix": {Kind: &ptypesstruct.Value_StringValue{StringValue: fmt.Sprintf("broker_tcp-%d", broker.Id)}},
										"cluster":     {Kind: &ptypesstruct.Value_StringValue{StringValue: fmt.Sprintf("broker-%d", broker.Id)}},
									},
								},
							},
						},
					},
				},
			},
		})

		clusters = append(clusters, &envoyapi.Cluster{
			Name:                 fmt.Sprintf("broker-%d", broker.Id),
			ConnectTimeout:       &duration.Duration{Seconds: 1},
			ClusterDiscoveryType: &envoyapi.Cluster_Type{Type: envoyapi.Cluster_STRICT_DNS},
			LbPolicy:             envoyapi.Cluster_ROUND_ROBIN,
			Http2ProtocolOptions: &envoycore.Http2ProtocolOptions{},
			Hosts: []*envoycore.Address{
				{
					Address: &envoycore.Address_SocketAddress{
						SocketAddress: &envoycore.SocketAddress{
							Address: fmt.Sprintf("%s-%d.%s-headless.%s.svc.cluster.local", kc.Name, broker.Id, kc.Name, kc.Namespace),
							PortSpecifier: &envoycore.SocketAddress_PortValue{
								PortValue: uint32(kc.Spec.ListenersConfig.ExternalListeners[0].ContainerPort),
							},
						},
					},
				},
			},
		})
	}

	config := envoybootstrap.Bootstrap_StaticResources{
		Listeners: listeners,
		Clusters:  clusters,
	}
	generatedConfig := envoybootstrap.Bootstrap{
		Admin:           &adminConfig,
		StaticResources: &config,
	}
	marshaller := &jsonpb.Marshaler{}
	marshalledProtobufConfig, err := marshaller.MarshalToString(&generatedConfig)
	if err != nil {
		log.Error(err, "could not marshall envoy config")
		return ""
	}

	marshalledConfig, err := yaml.JSONToYAML([]byte(marshalledProtobufConfig))
	if err != nil {
		log.Error(err, "could not convert config from Json to Yaml")
		return ""
	}
	return string(marshalledConfig)
}
