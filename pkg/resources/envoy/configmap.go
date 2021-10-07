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

	envoyaccesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoylistener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoystdoutaccesslog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/stream/v3"
	envoyhttphealthcheck "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/health_check/v3"
	envoyhcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoytcpproxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoytypes "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/kafka"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/util"
	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
	kafkautils "github.com/banzaicloud/koperator/pkg/util/kafka"
)

func (r *Reconciler) configMap(log logr.Logger, extListener v1beta1.ExternalListenerConfig,
	ingressConfig v1beta1.IngressConfig, ingressConfigName, defaultIngressConfigName string) runtime.Object {
	eListenerLabelName := util.ConstructEListenerLabelName(ingressConfigName, extListener.Name)

	var configMapName string = util.GenerateEnvoyResourceName(envoyutils.EnvoyVolumeAndConfigName, envoyutils.EnvoyVolumeAndConfigNameWithScope,
		extListener, ingressConfig, ingressConfigName, r.KafkaCluster.GetName())

	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(
			configMapName,
			labelsForEnvoyIngress(r.KafkaCluster.GetName(), eListenerLabelName), r.KafkaCluster),
		Data: map[string]string{"envoy.yaml": GenerateEnvoyConfig(r.KafkaCluster, extListener, ingressConfig,
			ingressConfigName, defaultIngressConfigName, log)},
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

func generateEnvoyHealthCheckListener(ingressConfig v1beta1.IngressConfig, log logr.Logger) *envoylistener.Listener {
	// health-check http listener
	stdoutAccessLog := &envoystdoutaccesslog.StdoutAccessLog{}
	pbstStdoutAccessLog, err := anypb.New(stdoutAccessLog)
	if err != nil {
		log.Error(err, "could not marshall envoy health-check stdoutAccessLog config")
		return nil
	}
	healtCheckConfig := &envoyhttphealthcheck.HealthCheck{
		PassThroughMode: wrapperspb.Bool(false),
		ClusterMinHealthyPercentages: map[string]*envoytypes.Percent{
			envoyutils.AllBrokerEnvoyConfigName: {Value: float64(1)},
		},
		Headers: []*envoyroute.HeaderMatcher{
			{
				Name: ":path",
				HeaderMatchSpecifier: &envoyroute.HeaderMatcher_ExactMatch{
					ExactMatch: envoyutils.HealthCheckPath,
				},
			},
		},
	}
	pbstHealthCheckConfig, err := anypb.New(healtCheckConfig)
	if err != nil {
		log.Error(err, "could not marshall envoy stdoutAccessLog config")
		return nil
	}

	healthCheckFilter := &envoyhcm.HttpConnectionManager{
		StatPrefix: fmt.Sprintf("%s-healthcheck", envoyutils.AllBrokerEnvoyConfigName),
		RouteSpecifier: &envoyhcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoyroute.RouteConfiguration{
				Name: "local",
				VirtualHosts: []*envoyroute.VirtualHost{
					{
						Name:    "localhost",
						Domains: []string{"*"},
						Routes: []*envoyroute.Route{
							{
								Match: &envoyroute.RouteMatch{
									PathSpecifier: &envoyroute.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &envoyroute.Route_Redirect{
									Redirect: &envoyroute.RedirectAction{
										PathRewriteSpecifier: &envoyroute.RedirectAction_PathRedirect{
											PathRedirect: envoyutils.HealthCheckPath,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		HttpFilters: []*envoyhcm.HttpFilter{
			{
				Name: wellknown.HealthCheck,
				ConfigType: &envoyhcm.HttpFilter_TypedConfig{
					TypedConfig: pbstHealthCheckConfig,
				},
			},
			{
				Name: wellknown.Router,
			},
		},
		AccessLog: []*envoyaccesslog.AccessLog{
			{
				Name: "envoy.access_loggers.stdout",
				ConfigType: &envoyaccesslog.AccessLog_TypedConfig{
					TypedConfig: pbstStdoutAccessLog,
				},
			},
		},
	}
	pbstHealthCheckFilter, err := anypb.New(healthCheckFilter)
	if err != nil {
		log.Error(err, "could not marshall envoy healthCheckFilter config")
		return nil
	}
	return &envoylistener.Listener{
		Address: &envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: uint32(ingressConfig.EnvoyConfig.GetEnvoyHealthCheckPort()),
					},
				},
			},
		},
		FilterChains: []*envoylistener.FilterChain{
			{
				Filters: []*envoylistener.Filter{
					{
						Name: wellknown.HTTPConnectionManager,
						ConfigType: &envoylistener.Filter_TypedConfig{
							TypedConfig: pbstHealthCheckFilter,
						},
					},
				},
			},
		},
		SocketOptions: getKeepAliveSocketOptions(),
	}
}

// GenerateEnvoyConfig generate envoy configuration file
func GenerateEnvoyConfig(kc *v1beta1.KafkaCluster, elistener v1beta1.ExternalListenerConfig, ingressConfig v1beta1.IngressConfig,
	ingressConfigName, defaultIngressConfigName string, log logr.Logger) string {
	adminConfig := envoybootstrap.Admin{
		Address: &envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: uint32(ingressConfig.EnvoyConfig.GetEnvoyAdminPort()),
					},
				},
			},
		},
	}

	var listeners []*envoylistener.Listener
	var clusters []*envoycluster.Cluster

	for _, brokerId := range util.GetBrokerIdsFromStatusAndSpec(kc.Status.BrokersState, kc.Spec.Brokers, log) {
		brokerConfig, err := kafkautils.GatherBrokerConfigIfAvailable(kc.Spec, brokerId)
		if err != nil {
			log.Error(err, "could not determine brokerConfig")
			continue
		}
		if util.ShouldIncludeBroker(brokerConfig, kc.Status, brokerId, defaultIngressConfigName, ingressConfigName) {
			// TCP_Proxy filter configuration
			tcpProxy := &envoytcpproxy.TcpProxy{
				StatPrefix:         fmt.Sprintf("broker_tcp-%d", brokerId),
				MaxConnectAttempts: &wrapperspb.UInt32Value{Value: 2},
				IdleTimeout:        &durationpb.Duration{Seconds: 560},
				ClusterSpecifier: &envoytcpproxy.TcpProxy_Cluster{
					Cluster: fmt.Sprintf("broker-%d", brokerId),
				},
			}
			pbstTcpProxy, err := anypb.New(tcpProxy)
			if err != nil {
				log.Error(err, "could not marshall envoy tcp_proxy config")
				return ""
			}
			listeners = append(listeners, &envoylistener.Listener{

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
								ConfigType: &envoylistener.Filter_TypedConfig{
									TypedConfig: pbstTcpProxy,
								},
							},
						},
					},
				},
				SocketOptions: getKeepAliveSocketOptions(),
			})

			clusters = append(clusters, &envoycluster.Cluster{
				Name:           fmt.Sprintf("broker-%d", brokerId),
				ConnectTimeout: &durationpb.Duration{Seconds: 1},
				UpstreamConnectionOptions: &envoycluster.UpstreamConnectionOptions{
					TcpKeepalive: getTcpKeepalive(),
				},
				ClusterDiscoveryType: &envoycluster.Cluster_Type{Type: envoycluster.Cluster_STRICT_DNS},
				LbPolicy:             envoycluster.Cluster_ROUND_ROBIN,
				// disable circuit breaking:
				// https://www.envoyproxy.io/docs/envoy/latest/faq/load_balancing/disable_circuit_breaking
				CircuitBreakers: &envoycluster.CircuitBreakers{
					Thresholds: []*envoycluster.CircuitBreakers_Thresholds{
						{
							Priority:           envoycore.RoutingPriority_DEFAULT,
							MaxConnections:     &wrapperspb.UInt32Value{Value: 1_000_000_000},
							MaxPendingRequests: &wrapperspb.UInt32Value{Value: 1_000_000_000},
							MaxRequests:        &wrapperspb.UInt32Value{Value: 1_000_000_000},
							MaxRetries:         &wrapperspb.UInt32Value{Value: 1_000_000_000},
						},
						{
							Priority:           envoycore.RoutingPriority_HIGH,
							MaxConnections:     &wrapperspb.UInt32Value{Value: 1_000_000_000},
							MaxPendingRequests: &wrapperspb.UInt32Value{Value: 1_000_000_000},
							MaxRequests:        &wrapperspb.UInt32Value{Value: 1_000_000_000},
							MaxRetries:         &wrapperspb.UInt32Value{Value: 1_000_000_000},
						},
					},
				},
				LoadAssignment: &envoyendpoint.ClusterLoadAssignment{
					ClusterName: fmt.Sprintf("broker-%d", brokerId),
					Endpoints: []*envoyendpoint.LocalityLbEndpoints{{
						LbEndpoints: []*envoyendpoint.LbEndpoint{{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: &envoycore.Address{
										Address: &envoycore.Address_SocketAddress{
											SocketAddress: &envoycore.SocketAddress{
												Protocol: envoycore.SocketAddress_TCP,
												Address:  generateAddressValue(kc, brokerId),
												PortSpecifier: &envoycore.SocketAddress_PortValue{
													PortValue: uint32(elistener.ContainerPort),
												},
											},
										},
									},
								},
							},
						}},
					}},
				},
			})
		}
	}
	// Create an any cast broker access point

	// TCP_Proxy filter configuration
	tcpProxy := &envoytcpproxy.TcpProxy{
		StatPrefix:         envoyutils.AllBrokerEnvoyConfigName,
		IdleTimeout:        &durationpb.Duration{Seconds: 560},
		MaxConnectAttempts: &wrapperspb.UInt32Value{Value: 2},
		ClusterSpecifier: &envoytcpproxy.TcpProxy_Cluster{
			Cluster: envoyutils.AllBrokerEnvoyConfigName,
		},
	}
	pbstTcpProxy, err := anypb.New(tcpProxy)
	if err != nil {
		log.Error(err, "could not marshall envoy tcp_proxy config")
		return ""
	}
	listeners = append(listeners, &envoylistener.Listener{
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
						ConfigType: &envoylistener.Filter_TypedConfig{
							TypedConfig: pbstTcpProxy,
						},
					},
				},
			},
		},
		SocketOptions: getKeepAliveSocketOptions(),
	})

	// health-check http listener
	healthCheckListener := generateEnvoyHealthCheckListener(ingressConfig, log)
	if healthCheckListener == nil {
		return ""
	}
	listeners = append(listeners, healthCheckListener)

	clusters = append(clusters, &envoycluster.Cluster{
		Name:           envoyutils.AllBrokerEnvoyConfigName,
		ConnectTimeout: &durationpb.Duration{Seconds: 1},
		UpstreamConnectionOptions: &envoycluster.UpstreamConnectionOptions{
			TcpKeepalive: getTcpKeepalive(),
		},
		IgnoreHealthOnHostRemoval: true,
		HealthChecks: []*envoycore.HealthCheck{
			{
				Interval:           &durationpb.Duration{Seconds: 5},
				Timeout:            &durationpb.Duration{Seconds: 1},
				NoTrafficInterval:  &durationpb.Duration{Seconds: 5},
				UnhealthyInterval:  &durationpb.Duration{Seconds: 2},
				IntervalJitter:     &durationpb.Duration{Seconds: 1},
				UnhealthyThreshold: wrapperspb.UInt32(2),
				HealthyThreshold:   wrapperspb.UInt32(1),
				EventLogPath:       "/dev/stdout",
				HealthChecker: &envoycore.HealthCheck_HttpHealthCheck_{
					HttpHealthCheck: &envoycore.HealthCheck_HttpHealthCheck{
						Path: kafka.MetricsHealthCheck,
					},
				},
			},
		},
		ClusterDiscoveryType: &envoycluster.Cluster_Type{Type: envoycluster.Cluster_STRICT_DNS},
		LbPolicy:             envoycluster.Cluster_ROUND_ROBIN,
		// disable circuit breakingL:
		// https://www.envoyproxy.io/docs/envoy/latest/faq/load_balancing/disable_circuit_breaking
		CircuitBreakers: &envoycluster.CircuitBreakers{
			Thresholds: []*envoycluster.CircuitBreakers_Thresholds{
				{
					Priority:           envoycore.RoutingPriority_DEFAULT,
					MaxConnections:     &wrapperspb.UInt32Value{Value: 1_000_000_000},
					MaxPendingRequests: &wrapperspb.UInt32Value{Value: 1_000_000_000},
					MaxRequests:        &wrapperspb.UInt32Value{Value: 1_000_000_000},
					MaxRetries:         &wrapperspb.UInt32Value{Value: 1_000_000_000},
				},
				{
					Priority:           envoycore.RoutingPriority_HIGH,
					MaxConnections:     &wrapperspb.UInt32Value{Value: 1_000_000_000},
					MaxPendingRequests: &wrapperspb.UInt32Value{Value: 1_000_000_000},
					MaxRequests:        &wrapperspb.UInt32Value{Value: 1_000_000_000},
					MaxRetries:         &wrapperspb.UInt32Value{Value: 1_000_000_000},
				},
			},
		},
		LoadAssignment: &envoyendpoint.ClusterLoadAssignment{
			ClusterName: envoyutils.AllBrokerEnvoyConfigName,
			Endpoints: []*envoyendpoint.LocalityLbEndpoints{{
				LbEndpoints: []*envoyendpoint.LbEndpoint{{
					HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
						Endpoint: &envoyendpoint.Endpoint{
							HealthCheckConfig: &envoyendpoint.Endpoint_HealthCheckConfig{
								PortValue: uint32(kafka.MetricsPort),
							},
							Address: &envoycore.Address{
								Address: &envoycore.Address_SocketAddress{
									SocketAddress: &envoycore.SocketAddress{
										Protocol: envoycore.SocketAddress_TCP,
										Address:  generateAnyCastAddressValue(kc),
										PortSpecifier: &envoycore.SocketAddress_PortValue{
											PortValue: uint32(elistener.ContainerPort),
										},
									},
								},
							},
						},
					},
				}},
			}},
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
	marshaller := &protojson.MarshalOptions{}
	marshalledProtobufConfig, err := marshaller.Marshal(&generatedConfig)
	if err != nil {
		log.Error(err, "could not marshall envoy config")
		return ""
	}

	marshalledConfig, err := yaml.JSONToYAML(marshalledProtobufConfig)
	if err != nil {
		log.Error(err, "could not convert config from Json to Yaml")
		return ""
	}
	return string(marshalledConfig)
}

func getTcpKeepalive() *envoycore.TcpKeepalive {
	return &envoycore.TcpKeepalive{
		KeepaliveProbes:   wrapperspb.UInt32(3),
		KeepaliveTime:     wrapperspb.UInt32(30),
		KeepaliveInterval: wrapperspb.UInt32(30),
	}
}

func getKeepAliveSocketOptions() []*envoycore.SocketOption {
	return []*envoycore.SocketOption{
		// enable socket keep-alive
		{
			// SOL_SOCKET = 1
			Level: 1,
			// SO_KEEPALIVE = 9
			Name:  9,
			Value: &envoycore.SocketOption_IntValue{IntValue: 1},
			State: envoycore.SocketOption_STATE_PREBIND,
		},
		// configure keep alive idle, interval and count
		{
			// IPPROTO_TCP = 6
			Level: 6,
			// TCP_KEEPIDLE = 4
			Name:  4,
			Value: &envoycore.SocketOption_IntValue{IntValue: 30},
			State: envoycore.SocketOption_STATE_PREBIND,
		},
		{
			// IPPROTO_TCP = 6
			Level: 6,
			// TCP_KEEPINTVL = 5
			Name:  5,
			Value: &envoycore.SocketOption_IntValue{IntValue: 30},
			State: envoycore.SocketOption_STATE_PREBIND,
		},
		{
			// IPPROTO_TCP = 6
			Level: 6,
			// TCP_KEEPCNT = 6
			Name:  6,
			Value: &envoycore.SocketOption_IntValue{IntValue: 3},
			State: envoycore.SocketOption_STATE_PREBIND,
		},
	}
}
