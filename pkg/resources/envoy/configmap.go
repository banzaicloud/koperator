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

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/duration"
	ptypesstruct "github.com/golang/protobuf/ptypes/struct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	kafkautils "github.com/banzaicloud/kafka-operator/pkg/util/kafka"
)

func (r *Reconciler) configMap(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	_ v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	var configMapName string
	if ingressConfigName == util.IngressConfigGlobalName {
		configMapName = fmt.Sprintf(envoyVolumeAndConfigName, extListener.Name, r.KafkaCluster.GetName())
	} else {
		configMapName = fmt.Sprintf(envoyVolumeAndConfigNameWithScope, extListener.Name,
			ingressConfigName, r.KafkaCluster.GetName())
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			configMapName,
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), extListener.Name), r.KafkaCluster),
		Data: map[string]string{"envoy.yaml": GenerateEnvoyConfig(r.KafkaCluster, extListener, ingressConfigName,
			defaultIngressConfigName, log)},
	}
	return configMap
}

func generateAddressValue(kc *v1beta1.KafkaCluster, brokerId int) string {
	if kc.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s-%d.%s-headless.%s.svc.%s", kc.Name, brokerId, kc.Name, kc.Namespace, kc.Spec.GetKubernetesClusterDomain())
	}
	//ClusterIP services are in use
	return fmt.Sprintf("%s-%d.%s.svc.%s", kc.Name, brokerId, kc.Namespace, kc.Spec.GetKubernetesClusterDomain())
}

func generateAnyCastAddressValue(kc *v1beta1.KafkaCluster) string {
	if kc.Spec.HeadlessServiceEnabled {
		return fmt.Sprintf("%s-headless.%s.svc.%s", kc.GetName(), kc.GetNamespace(), kc.Spec.GetKubernetesClusterDomain())
	}
	//ClusterIP services are in use
	return fmt.Sprintf(
		kafkautils.AllBrokerServiceTemplate+".%s.svc.%s", kc.GetName(), kc.GetNamespace(), kc.Spec.GetKubernetesClusterDomain())
}

func GenerateEnvoyConfig(kc *v1beta1.KafkaCluster, elistener v1beta1.ExternalListenerConfig,
	ingressConfigName, defaultIngressConfigName string, log logr.Logger) string {

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

	for _, brokerId := range util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log) {
		var err error
		brokerConfig := &v1beta1.BrokerConfig{}
		brokerIdPresent := false
		var requiredBroker v1beta1.Broker
		// This check is used in case of broker delete. In case of broker delete there is some time when the CC removes the broker
		// gracefully which means we have to generate the port for that broker as well. At that time the status contains
		// but the broker spec does not contain the required config values.
		for _, broker := range kc.Spec.Brokers {
			if int(broker.Id) == brokerId {
				brokerIdPresent = true
				requiredBroker = broker
				break
			}
		}
		if brokerIdPresent {
			brokerConfig, err = util.GetBrokerConfig(requiredBroker, kc.Spec)
			if err != nil {
				log.Error(err, "could not determine brokerConfig")
				continue
			}
		}
		if (len(brokerConfig.BrokerIdBindings) == 0 && ingressConfigName == defaultIngressConfigName) ||
			util.StringSliceContains(brokerConfig.BrokerIdBindings, ingressConfigName) {
			listeners = append(listeners, &envoyapi.Listener{
				Address: &envoycore.Address{
					Address: &envoycore.Address_SocketAddress{
						SocketAddress: &envoycore.SocketAddress{
							Address: "0.0.0.0",
							PortSpecifier: &envoycore.SocketAddress_PortValue{
								PortValue: uint32(elistener.ExternalStartingPort + int32(brokerId)),
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
											"stat_prefix": {Kind: &ptypesstruct.Value_StringValue{StringValue: fmt.Sprintf("broker_tcp-%d", brokerId)}},
											"cluster":     {Kind: &ptypesstruct.Value_StringValue{StringValue: fmt.Sprintf("broker-%d", brokerId)}},
										},
									},
								},
							},
						},
					},
				},
			})

			clusters = append(clusters, &envoyapi.Cluster{
				Name:                 fmt.Sprintf("broker-%d", brokerId),
				ConnectTimeout:       &duration.Duration{Seconds: 1},
				ClusterDiscoveryType: &envoyapi.Cluster_Type{Type: envoyapi.Cluster_STRICT_DNS},
				LbPolicy:             envoyapi.Cluster_ROUND_ROBIN,
				Http2ProtocolOptions: &envoycore.Http2ProtocolOptions{},
				Hosts: []*envoycore.Address{
					{
						Address: &envoycore.Address_SocketAddress{
							SocketAddress: &envoycore.SocketAddress{
								Address: generateAddressValue(kc, brokerId),
								PortSpecifier: &envoycore.SocketAddress_PortValue{
									PortValue: uint32(elistener.ContainerPort),
								},
							},
						},
					},
				},
			})
		}
	}
	// Create an any cast broker access point
	listeners = append(listeners, &envoyapi.Listener{
		Address: &envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: uint32(elistener.GetAnyCastPort()),
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
									"stat_prefix": {Kind: &ptypesstruct.Value_StringValue{StringValue: allBrokerEnvoyConfigName}},
									"cluster":     {Kind: &ptypesstruct.Value_StringValue{StringValue: allBrokerEnvoyConfigName}},
								},
							},
						},
					},
				},
			},
		},
	})

	clusters = append(clusters, &envoyapi.Cluster{
		Name:                 allBrokerEnvoyConfigName,
		ConnectTimeout:       &duration.Duration{Seconds: 1},
		ClusterDiscoveryType: &envoyapi.Cluster_Type{Type: envoyapi.Cluster_STRICT_DNS},
		LbPolicy:             envoyapi.Cluster_ROUND_ROBIN,
		Http2ProtocolOptions: &envoycore.Http2ProtocolOptions{},
		Hosts: []*envoycore.Address{
			{
				Address: &envoycore.Address_SocketAddress{
					SocketAddress: &envoycore.SocketAddress{
						Address: generateAnyCastAddressValue(kc),
						PortSpecifier: &envoycore.SocketAddress_PortValue{
							PortValue: uint32(elistener.ContainerPort),
						},
					},
				},
			},
		},
	})

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
