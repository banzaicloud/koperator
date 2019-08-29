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
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"emperror.dev/emperror"
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/certutil"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	componentName = "kafka"
	// AllBrokerServiceTemplate template for Kafka headless service
	AllBrokerServiceTemplate = "%s-all-broker"
	// HeadlessServiceTemplate template for Kafka headless service
	HeadlessServiceTemplate = "%s-headless"
	brokerConfigTemplate    = "%s-config"
	brokerStorageTemplate   = "%s-storage"

	brokerConfigMapVolumeMount    = "broker-config"
	modbrokerConfigMapVolumeMount = "broker-modconfig"
	kafkaDataVolumeMount          = "kafka-data"
	keystoreVolume                = "ks-files"
	keystoreVolumePath            = "/var/run/secrets/java.io/keystores"
	pemFilesVolume                = "pem-files"
	jmxVolumePath                 = "/opt/jmx-exporter/"
	jmxVolumeName                 = "jmx-jar-data"
	metricsPort                   = 9020

	//jaasConfig  = "jaas-config"
	//scramSecret = "scram-secret"
)

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
}

// labelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}

// New creates a new reconciler for Kafka
func New(client client.Client, cluster *banzaicloudv1alpha1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}

func getCreatedPVCForBroker(c client.Client, brokerID int32, namespace, crName string) ([]corev1.PersistentVolumeClaim, error) {
	foundPVCList := &corev1.PersistentVolumeClaimList{}
	matchingLabels := client.MatchingLabels{
		"kafka_cr": crName,
		"brokerId": fmt.Sprintf("%d", brokerID),
	}
	err := c.List(context.TODO(), foundPVCList, client.ListOption(client.InNamespace(namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return nil, err
	}
	if len(foundPVCList.Items) == 0 {
		return nil, fmt.Errorf("no persistentvolume found for broker %d", brokerID)
	}
	return foundPVCList.Items, nil
}

func getLoadBalancerIP(client client.Client, namespace string, log logr.Logger) (string, error) {
	foundLBService := &corev1.Service{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: envoy.EnvoyServiceName, Namespace: namespace}, foundLBService)
	if err != nil {
		return "", err
	}

	if len(foundLBService.Status.LoadBalancer.Ingress) == 0 {
		return "", errors.New("loadbalancer is not created waiting")
	}

	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" && foundLBService.Status.LoadBalancer.Ingress[0].IP == "" {
		time.Sleep(20 * time.Second)
		return "", errors.New("loadbalancer is not ready waiting")
	}
	var loadBalancerExternalAddress string
	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" {
		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].IP
	} else {
		loadBalancerExternalAddress = foundLBService.Status.LoadBalancer.Ingress[0].Hostname
	}
	return loadBalancerExternalAddress, nil
}

// Reconcile implements the reconcile logic for Kafka
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.V(1).Info("Reconciling")

	if r.KafkaCluster.Spec.HeadlessServiceEnabled {
		for _, res := range []resources.Resource{
			r.headlessService,
		} {
			o := res()
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
	} else {
		for _, res := range []resources.Resource{
			r.allBrokerService,
		} {
			o := res()
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
	}
	// Handle Pod delete
	podList := &corev1.PodList{}
	matchingLabels := client.MatchingLabels{
		"kafka_cr": r.KafkaCluster.Name,
	}

	err := r.Client.List(context.TODO(), podList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return emperror.Wrap(err, "failed to reconcile resource")
	}
	if len(podList.Items) > len(r.KafkaCluster.Spec.Brokers) {
		deletedBrokers := make([]corev1.Pod, 0)
	OUTERLOOP:
		for _, pod := range podList.Items {
			for _, broker := range r.KafkaCluster.Spec.Brokers {
				if pod.Labels["brokerId"] == fmt.Sprintf("%d", broker.Id) {
					continue OUTERLOOP
				}
			}
			deletedBrokers = append(deletedBrokers, pod)
		}
		for _, broker := range deletedBrokers {
			err = scale.DownsizeCluster(broker.Labels["brokerId"], broker.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
			if err != nil {
				log.Error(err, "graceful downscale failed.")
			}
			err = r.Client.Delete(context.TODO(), &broker)
			if err != nil {
				return emperror.WrapWith(err, "could not delete broker", "id", broker.Labels["brokerId"])
			}
			err = r.Client.Delete(context.TODO(), &corev1.ConfigMap{ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate+"-%s", r.KafkaCluster.Name, broker.Labels["brokerId"]), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
			if err != nil {
				return emperror.WrapWith(err, "could not delete configmap for broker", "id", broker.Labels["brokerId"])
			}
			if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
				err = r.Client.Delete(context.TODO(), &corev1.Service{ObjectMeta: templates.ObjectMeta(fmt.Sprintf("%s-%s", r.KafkaCluster.Name, broker.Labels["brokerId"]), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
				if err != nil {
					return emperror.WrapWith(err, "could not delete service for broker", "id", broker.Labels["brokerId"])
				}
			}
			for _, volume := range broker.Spec.Volumes {
				if strings.HasPrefix(volume.Name, kafkaDataVolumeMount) {
					err = r.Client.Delete(context.TODO(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
						Name:      volume.PersistentVolumeClaim.ClaimName,
						Namespace: r.KafkaCluster.Namespace,
					}})
					if err != nil {
						return emperror.WrapWith(err, "could not delete pvc for broker", "id", broker.Labels["brokerId"])
					}
				}
			}
			err = k8sutil.DeleteStatus(r.Client, broker.Labels["brokerId"], r.KafkaCluster, log)
			if err != nil {
				return emperror.WrapWith(err, "could not delete status for broker", "id", broker.Labels["brokerId"])
			}
		}
	}
	lBIp := ""

	if r.KafkaCluster.Spec.ListenersConfig.ExternalListeners != nil {
		lBIp, err = getLoadBalancerIP(r.Client, r.KafkaCluster.Namespace, log)
		if err != nil {
			return emperror.WrapWith(err, "failed to get loadbalancerIP maybe still creating...")
		}
	}
	//TODO remove after testing
	//lBIp := "192.168.0.1"

	// We need to grab names for servers and client in case user is enabling ACLs
	// That way we can continue to manage topics and users
	superUsers := make([]string, 0)
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil {
		controllerSecret := &corev1.Secret{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      r.KafkaCluster.Spec.ListenersConfig.SSLSecrets.TLSSecretName,
			Namespace: r.KafkaCluster.Namespace,
		}, controllerSecret)
		if err != nil {
			return emperror.WrapWith(err, "failed to get controller secret, maybe still creating...")
		}
		for _, key := range []string{"clientCert", "peerCert"} {
			su, err := certutil.DecodeCertificate(controllerSecret.Data[key])
			if err != nil {
				return emperror.WrapWith(err, "Failed to decode our client certificate")
			}
			superUsers = append(superUsers, su.Subject.String())
		}
	}

	for _, broker := range r.KafkaCluster.Spec.Brokers {
		for _, storage := range broker.BrokerConfig.StorageConfigs {
			for _, res := range []resources.ResourceWithBrokerAndStorage{
				r.pvc,
			} {
				o := res(broker, storage, log)
				err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
				if err != nil {
					return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
				}
			}
		}
		if r.KafkaCluster.Spec.RackAwareness == nil {
			for _, res := range []resources.ResourceWithBrokerAndString{
				r.configMapPod,
			} {
				o := res(broker, lBIp, superUsers, log)
				err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
				if err != nil {
					return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
				}
			}
		} else {
			if brokerState, ok := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(broker.Id))]; ok {
				if brokerState.RackAwarenessState == banzaicloudv1alpha1.Configured {
					for _, res := range []resources.ResourceWithBrokerAndString{
						r.configMapPod,
					} {
						o := res(broker, lBIp, superUsers, log)
						err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
						if err != nil {
							return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
						}
					}
				}
			}
		}

		pvcs, err := getCreatedPVCForBroker(r.Client, broker.Id, r.KafkaCluster.Namespace, r.KafkaCluster.Name)
		if err != nil {
			return emperror.WrapWith(err, "failed to list PVC's")
		}

		if !r.KafkaCluster.Spec.HeadlessServiceEnabled {

			for _, res := range []resources.ResourceWithBrokerAndLog{
				r.service,
			} {
				o := res(broker, log)
				err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
				if err != nil {
					return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
				}
			}
		}

		for _, res := range []resources.ResourceWithBrokerAndVolume{
			r.pod,
		} {
			o := res(broker, pvcs, log)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}
