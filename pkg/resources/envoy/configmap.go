package envoy

//import (
//	"fmt"
//	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
//	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
//	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
//	"github.com/envoyproxy/go-control-plane/pkg/util"
//	"github.com/ghodss/yaml"
//	"github.com/go-logr/logr"
//	"github.com/gogo/protobuf/jsonpb"
//	"k8s.io/apimachinery/pkg/runtime"
//	"time"
//
//	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
//	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
//	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
//	ptypes "github.com/gogo/protobuf/types"
//	corev1 "k8s.io/api/core/v1"
//)
//
//func (r *Reconciler) configMap(log logr.Logger) runtime.Object {
//	configMap := &corev1.ConfigMap{
//		ObjectMeta: templates.ObjectMetaWithName(envoyVolumeAndConfigName, labelSelector, r.KafkaCluster),
//		Data:       map[string]string{"envoy.yaml": generateEnvoyConfig(r.KafkaCluster, log)},
//	}
//	return configMap
//}
//
//func generateEnvoyConfig(kc *banzaicloudv1alpha1.KafkaCluster, log logr.Logger) string {
//	//TODO support multiple external listener by removing [0]
//	adminConfig := envoybootstrap.Admin{
//		AccessLogPath: "/tmp/admin_access.log",
//		Address: &core.Address{
//			Address: &core.Address_SocketAddress{
//				SocketAddress: &core.SocketAddress{
//					Address: "0.0.0.0",
//					PortSpecifier: &core.SocketAddress_PortValue{
//						PortValue: 9901,
//					},
//				},
//			},
//		},
//	}
//
//	var listeners []envoyapi.Listener
//	var clusters []envoyapi.Cluster
//
//	for brokerId := int32(0); brokerId < kc.Spec.Brokers; brokerId++ {
//		listeners = append(listeners, envoyapi.Listener{
//			Address: core.Address{
//				Address: &core.Address_SocketAddress{
//					SocketAddress: &core.SocketAddress{
//						Address: "0.0.0.0",
//						PortSpecifier: &core.SocketAddress_PortValue{
//							PortValue: uint32(kc.Spec.Listeners.ExternalListener[0].ExternalStartingPort + brokerId),
//						},
//					},
//				},
//			},
//			FilterChains: []listener.FilterChain{
//				{
//					Filters: []listener.Filter{
//						{
//							Name: util.TCPProxy,
//							ConfigType: &listener.Filter_Config{
//								Config: &ptypes.Struct{
//									Fields: map[string]*ptypes.Value{
//										"stat_prefix": {Kind: &ptypes.Value_StringValue{StringValue: fmt.Sprintf("broker_tcp-%d", brokerId)}},
//										"cluster":     {Kind: &ptypes.Value_StringValue{StringValue: fmt.Sprintf("broker-%d", brokerId)}},
//									},
//								},
//							},
//						},
//					},
//				},
//			},
//		})
//
//		clusters = append(clusters, envoyapi.Cluster{
//			Name:                 fmt.Sprintf("broker-%d", brokerId),
//			ConnectTimeout:       250 * time.Millisecond,
//			Type:                 envoyapi.Cluster_STRICT_DNS,
//			LbPolicy:             envoyapi.Cluster_ROUND_ROBIN,
//			Http2ProtocolOptions: &core.Http2ProtocolOptions{},
//			Hosts: []*core.Address{
//				{
//					Address: &core.Address_SocketAddress{
//						SocketAddress: &core.SocketAddress{
//							Address: fmt.Sprintf("%s-%d.%s-headless.%s.svc.cluster.local", kc.Name, brokerId, kc.Name, kc.Namespace),
//							PortSpecifier: &core.SocketAddress_PortValue{
//								PortValue: uint32(kc.Spec.Listeners.ExternalListener[0].ContainerPort),
//							},
//						},
//					},
//				},
//			},
//		})
//	}
//
//	config := envoybootstrap.Bootstrap_StaticResources{
//		Listeners: listeners,
//		Clusters:  clusters,
//	}
//	generatedConfig := envoybootstrap.Bootstrap{
//		Admin:           &adminConfig,
//		StaticResources: &config,
//	}
//	marshaller := &jsonpb.Marshaler{}
//	marshalledProtobufConfig, err := marshaller.MarshalToString(&generatedConfig)
//	if err != nil {
//		log.Error(err, "could not marshall envoy config")
//		return ""
//	}
//
//	marshalledConfig, err := yaml.JSONToYAML([]byte(marshalledProtobufConfig))
//	if err != nil {
//		log.Error(err, "could not convert config from Json to Yaml")
//		return ""
//	}
//	return string(marshalledConfig)
//}
