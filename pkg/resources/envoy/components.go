package envoy

import (
	"errors"
	"fmt"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoybootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/ghodss/yaml"
	"github.com/gogo/protobuf/jsonpb"
	ptypes "github.com/gogo/protobuf/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"time"
)

var log = logf.Log.WithName("envoy-components-builder")

// LoadBalancerForEnvoy return a Loadbalancer service for Envoy
func LoadBalancerForEnvoy(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.Service {

	exposedPorts := getExposedServicePorts(kc.Spec.Listeners.ExternalListener, kc.Spec.Brokers)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envoy-loadbalancer",
			Namespace: kc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "envoy"},
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports:    exposedPorts,
		},
	}
	return service
}

func DeploymentForEnvoy(kc *banzaicloudv1alpha1.KafkaCluster) *appsv1.Deployment {

	exposedPorts := getExposedContainerPorts(kc.Spec.Listeners.ExternalListener, kc.Spec.Brokers)
	volumes := []corev1.Volume{
		{
			Name: "envoy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "envoy-config"},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "envoy-config",
			MountPath: "/etc/envoy",
			ReadOnly:  true,
		},
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envoy",
			Namespace: kc.Namespace,
			Labels:    map[string]string{"app": "envoy"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "envoy"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "envoy"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "envoy",
							Image:           "banzaicloud/envoy:01",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: append(exposedPorts, []corev1.ContainerPort{
								{Name: "envoy-admin", ContainerPort: 9901}}...),
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func ConfigMapForEnvoy(kc *banzaicloudv1alpha1.KafkaCluster) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envoy-config",
			Namespace: kc.Namespace,
			Labels:    map[string]string{"app": "envoy"},
		},
		Data: map[string]string{"envoy.yaml": generateEnvoyConfig(kc)},
	}
	return configMap
}

func generateEnvoyConfig(kc *banzaicloudv1alpha1.KafkaCluster) string {
	//TODO support multiple external listener by removing [0]
	adminConfig := envoybootstrap.Admin{
		AccessLogPath: "/tmp/admin_access.log",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: 9901,
					},
				},
			},
		},
	}

	var listeners []envoyapi.Listener
	var clusters []envoyapi.Cluster

	for brokerId := int32(0); brokerId < kc.Spec.Brokers; brokerId++ {
		listeners = append(listeners, envoyapi.Listener{
			Address: core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Address: "0.0.0.0",
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: uint32(kc.Spec.Listeners.ExternalListener[0].ExternalStartingPort + brokerId),
						},
					},
				},
			},
			FilterChains: []listener.FilterChain{
				{
					Filters: []listener.Filter{
						{
							Name: util.TCPProxy,
							ConfigType: &listener.Filter_Config{
								Config: &ptypes.Struct{
									Fields: map[string]*ptypes.Value{
										"stat_prefix": {Kind: &ptypes.Value_StringValue{StringValue: fmt.Sprintf("broker_tcp-%d", brokerId)}},
										"cluster":     {Kind: &ptypes.Value_StringValue{StringValue: fmt.Sprintf("broker-%d", brokerId)}},
									},
								},
							},
						},
					},
				},
			},
		})

		clusters = append(clusters, envoyapi.Cluster{
			Name:                 fmt.Sprintf("broker-%d", brokerId),
			ConnectTimeout:       250 * time.Millisecond,
			Type:                 envoyapi.Cluster_STRICT_DNS,
			LbPolicy:             envoyapi.Cluster_ROUND_ROBIN,
			Http2ProtocolOptions: &core.Http2ProtocolOptions{},
			Hosts: []*core.Address{
				{
					Address: &core.Address_SocketAddress{
						SocketAddress: &core.SocketAddress{
							Address: fmt.Sprintf("%s-%d.%s-headless.%s.svc.cluster.local", kc.Name, brokerId, kc.Name, kc.Namespace),
							PortSpecifier: &core.SocketAddress_PortValue{
								PortValue: uint32(kc.Spec.Listeners.ExternalListener[0].ContainerPort),
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

func getExposedServicePorts(extListeners []banzaicloudv1alpha1.ExternalListenerConfig, brokers int32) []corev1.ServicePort {
	var exposedPorts []corev1.ServicePort

	for _, eListener := range extListeners {
		switch eListener.Type {
		case "plaintext":
			for brokerSize := int32(0); brokerSize < brokers; brokerSize++ {
				exposedPorts = append(exposedPorts, corev1.ServicePort{
					Name: fmt.Sprintf("broker-%d", brokerSize),
					Port: eListener.ExternalStartingPort + brokerSize,
				})
			}
		case "tls":
			log.Error(errors.New("TLS listener type is not supported yet"), "not supported")
		case "both":
			log.Error(errors.New("both listener type is not supported yet"), "not supported")
		}
	}
	return exposedPorts
}

func getExposedContainerPorts(extListeners []banzaicloudv1alpha1.ExternalListenerConfig, brokers int32) []corev1.ContainerPort {
	var exposedPorts []corev1.ContainerPort

	for _, eListener := range extListeners {
		switch eListener.Type {
		case "plaintext":
			for brokerSize := int32(0); brokerSize < brokers; brokerSize++ {
				exposedPorts = append(exposedPorts, corev1.ContainerPort{
					Name:          fmt.Sprintf("broker-%d", brokerSize),
					ContainerPort: eListener.ExternalStartingPort + brokerSize,
				})
			}
		case "tls":
			log.Error(errors.New("TLS listener type is not supported yet"), "not supported")
		case "both":
			log.Error(errors.New("both listener type is not supported yet"), "not supported")
		}
	}
	return exposedPorts
}
