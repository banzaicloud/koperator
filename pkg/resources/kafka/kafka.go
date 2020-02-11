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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	"github.com/banzaicloud/kafka-operator/pkg/pki"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	certutil "github.com/banzaicloud/kafka-operator/pkg/util/cert"
	ccutils "github.com/banzaicloud/kafka-operator/pkg/util/cruisecontrol"
	envoyutils "github.com/banzaicloud/kafka-operator/pkg/util/envoy"
	istioingressutils "github.com/banzaicloud/kafka-operator/pkg/util/istioingress"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

const (
	componentName         = "kafka"
	brokerConfigTemplate  = "%s-config"
	brokerStorageTemplate = "%s-storage"

	brokerConfigMapVolumeMount = "broker-config"
	kafkaDataVolumeMount       = "kafka-data"

	serverKeystoreVolume = "server-ks-files"
	serverKeystorePath   = "/var/run/secrets/java.io/keystores/server"
	clientKeystoreVolume = "client-ks-files"
	clientKeystorePath   = "/var/run/secrets/java.io/keystores/client"

	jmxVolumePath = "/opt/jmx-exporter/"
	jmxVolumeName = "jmx-jar-data"
	metricsPort   = 9020
)

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
	Scheme *runtime.Scheme
}

// labelsForKafka returns the labels for selecting the resources
// belonging to the given kafka CR name.
func labelsForKafka(name string) map[string]string {
	return map[string]string{"app": "kafka", "kafka_cr": name}
}

// New creates a new reconciler for Kafka
func New(client client.Client, scheme *runtime.Scheme, cluster *v1beta1.KafkaCluster) *Reconciler {
	return &Reconciler{
		Scheme: scheme,
		Reconciler: resources.Reconciler{
			Client:       client,
			KafkaCluster: cluster,
		},
	}
}
func getCreatedPvcForBroker(c client.Client, brokerID int32, namespace, crName string) ([]corev1.PersistentVolumeClaim, error) {
	foundPvcList := &corev1.PersistentVolumeClaimList{}
	matchingLabels := client.MatchingLabels(
		util.MergeLabels(
			labelsForKafka(crName),
			map[string]string{"brokerId": fmt.Sprintf("%d", brokerID)},
		),
	)
	err := c.List(context.TODO(), foundPvcList, client.ListOption(client.InNamespace(namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return nil, err
	}
	if len(foundPvcList.Items) == 0 {
		return nil, fmt.Errorf("no persistentvolume found for broker %d", brokerID)
	}
	return foundPvcList.Items, nil
}

func getLoadBalancerIP(client client.Client, namespace, ingressController, crName string, log logr.Logger) (string, error) {
	foundLBService := &corev1.Service{}
	var iControllerServiceName string
	if ingressController == istioingressutils.IngressControllerName {
		iControllerServiceName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplate, crName)
	} else if ingressController == envoyutils.IngressControllerName {
		iControllerServiceName = envoyutils.EnvoyServiceName
	}

	err := client.Get(context.TODO(), types.NamespacedName{Name: iControllerServiceName, Namespace: namespace}, foundLBService)
	if err != nil {
		return "", err
	}

	if len(foundLBService.Status.LoadBalancer.Ingress) == 0 {
		return "", errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("loadbalancer is not created waiting"), "trying")
	}

	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" && foundLBService.Status.LoadBalancer.Ingress[0].IP == "" {
		time.Sleep(20 * time.Second)
		return "", errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("loadbalancer is not created waiting"), "trying")
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
	log = log.WithValues("component", componentName, "clusterName", r.KafkaCluster.Name, "clusterNamespace", r.KafkaCluster.Namespace)

	log.V(1).Info("Reconciling")

	if r.KafkaCluster.Spec.HeadlessServiceEnabled {
		o := r.headlessService()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	} else {
		o := r.allBrokerService()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}
	// Handle Pod delete
	podList := &corev1.PodList{}
	matchingLabels := client.MatchingLabels(labelsForKafka(r.KafkaCluster.Name))

	err := r.Client.List(context.TODO(), podList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return errors.WrapIf(err, "failed to reconcile resource")
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

		if !arePodsAlreadyDeleted(deletedBrokers, log) {
			if r.KafkaCluster.Status.BrokersState[generateBrokerIdsFromPodSlice(deletedBrokers)[0]].GracefulActionState.CruiseControlState != v1beta1.GracefulUpdateRunning &&
				r.KafkaCluster.Status.BrokersState[generateBrokerIdsFromPodSlice(deletedBrokers)[0]].GracefulActionState.CruiseControlState != v1beta1.GracefulDownscaleSucceeded {
				uTaskId, taskStartTime, err := scale.DownsizeCluster(strings.Join(generateBrokerIdsFromPodSlice(deletedBrokers), ","),
					r.KafkaCluster.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
				if err != nil {
					log.Info(fmt.Sprintf("cruise control communication error during downscaling broker(s) id(s): %s", strings.Join(generateBrokerIdsFromPodSlice(deletedBrokers), ",")))
					return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, fmt.Sprintf("broker(s) id(s): %s", strings.Join(generateBrokerIdsFromPodSlice(deletedBrokers), ",")))
				}
				err = k8sutil.UpdateBrokerStatus(r.Client, generateBrokerIdsFromPodSlice(deletedBrokers), r.KafkaCluster,
					v1beta1.GracefulActionState{CruiseControlTaskId: uTaskId, CruiseControlState: v1beta1.GracefulUpdateRunning,
						TaskStarted: taskStartTime}, log)
				if err != nil {
					return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(generateBrokerIdsFromPodSlice(deletedBrokers), ","))
				}
			}
			if r.KafkaCluster.Status.BrokersState[generateBrokerIdsFromPodSlice(deletedBrokers)[0]].GracefulActionState.CruiseControlState == v1beta1.GracefulUpdateRunning {
				err = r.checkCCTaskState(generateBrokerIdsFromPodSlice(deletedBrokers),
					r.KafkaCluster.Status.BrokersState[generateBrokerIdsFromPodSlice(deletedBrokers)[0]], v1beta1.GracefulDownscaleSucceeded, log)
				if err != nil {
					return err
				}
			}
		}

		for _, broker := range deletedBrokers {
			if broker.ObjectMeta.DeletionTimestamp != nil {
				log.Info(fmt.Sprintf("Broker %s is already on terminating state", broker.Labels["brokerId"]))
				continue
			}
			err = r.Client.Delete(context.TODO(), &broker)
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete broker", "id", broker.Labels["brokerId"])
			}
			err = r.Client.Delete(context.TODO(), &corev1.ConfigMap{ObjectMeta: templates.ObjectMeta(fmt.Sprintf(brokerConfigTemplate+"-%s", r.KafkaCluster.Name, broker.Labels["brokerId"]), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete configmap for broker", "id", broker.Labels["brokerId"])
			}
			if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
				err = r.Client.Delete(context.TODO(), &corev1.Service{ObjectMeta: templates.ObjectMeta(fmt.Sprintf("%s-%s", r.KafkaCluster.Name, broker.Labels["brokerId"]), labelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
				if err != nil {
					return errors.WrapIfWithDetails(err, "could not delete service for broker", "id", broker.Labels["brokerId"])
				}
			}
			for _, volume := range broker.Spec.Volumes {
				if strings.HasPrefix(volume.Name, kafkaDataVolumeMount) {
					err = r.Client.Delete(context.TODO(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
						Name:      volume.PersistentVolumeClaim.ClaimName,
						Namespace: r.KafkaCluster.Namespace,
					}})
					if err != nil {
						return errors.WrapIfWithDetails(err, "could not delete pvc for broker", "id", broker.Labels["brokerId"])
					}
				}
			}
			err = k8sutil.DeleteStatus(r.Client, broker.Labels["brokerId"], r.KafkaCluster, log)
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete status for broker", "id", broker.Labels["brokerId"])
			}
		}
	}

	lbIPs := make([]string, 0)

	if r.KafkaCluster.Spec.ListenersConfig.ExternalListeners != nil {
		// TODO: This is a hack that needs to be banished when the time is right.
		// Currently we only support one external listener but this will be fixed
		// sometime in the future
		if r.KafkaCluster.Spec.ListenersConfig.ExternalListeners[0].HostnameOverride != "" {
			// first element of slice will be used for external advertised listener
			lbIPs = append(lbIPs, r.KafkaCluster.Spec.ListenersConfig.ExternalListeners[0].HostnameOverride)
		}
		lbIP, err := getLoadBalancerIP(r.Client, r.KafkaCluster.Namespace, r.KafkaCluster.Spec.GetIngressController(), r.KafkaCluster.Name, log)
		if err != nil {
			return err
		}
		lbIPs = append(lbIPs, lbIP)
	}
	//TODO remove after testing
	//lBIp := "192.168.0.1"

	// Setup the PKI if using SSL
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil {
		// reconcile the PKI
		if err := pki.GetPKIManager(r.Client, r.KafkaCluster).ReconcilePKI(context.TODO(), log, r.Scheme, lbIPs); err != nil {
			return err
		}
	}

	// We need to grab names for servers and client in case user is enabling ACLs
	// That way we can continue to manage topics and users
	serverPass, clientPass, superUsers, err := r.getServerAndClientDetails()
	if err != nil {
		return err
	}

	for _, broker := range r.KafkaCluster.Spec.Brokers {
		brokerConfig, err := util.GetBrokerConfig(broker, r.KafkaCluster.Spec)
		if err != nil {
			return errors.WrapIf(err, "failed to reconcile resource")
		}
		for _, storage := range brokerConfig.StorageConfigs {
			o := r.pvc(broker.Id, storage, log)
			err := r.reconcileKafkaPvc(log, o.(*corev1.PersistentVolumeClaim))
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}

		}
		if r.KafkaCluster.Spec.RackAwareness == nil {
			o := r.configMap(broker.Id, brokerConfig, lbIPs, serverPass, clientPass, superUsers, log)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		} else {
			if brokerState, ok := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(broker.Id))]; ok {
				if brokerState.RackAwarenessState != "" {
					o := r.configMap(broker.Id, brokerConfig, lbIPs, serverPass, clientPass, superUsers, log)
					err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
					if err != nil {
						return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
					}
				}
			}
		}

		pvcs, err := getCreatedPvcForBroker(r.Client, broker.Id, r.KafkaCluster.Namespace, r.KafkaCluster.Name)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to list PVC's")
		}

		if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
			o := r.service(broker.Id, log)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
		o := r.pod(broker.Id, brokerConfig, pvcs, log)
		err = r.reconcileKafkaPod(log, o.(*corev1.Pod))
		if err != nil {
			return err
		}
		if err = r.reconcilePerBrokerDynamicConfig(broker.Id, brokerConfig, log); err != nil {
			return err
		}
	}

	if err = r.reconcileClusterWideDynamicConfig(log); err != nil {
		return err
	}

	log.V(1).Info("Reconciled")

	return nil
}

func generateBrokerIdsFromPodSlice(pods []corev1.Pod) []string {
	ids := make([]string, len(pods))
	for i, broker := range pods {
		ids[i] = broker.Labels["brokerId"]
	}
	return ids
}

func arePodsAlreadyDeleted(pods []corev1.Pod, log logr.Logger) bool {
	for _, broker := range pods {
		if broker.ObjectMeta.DeletionTimestamp == nil {
			return false
		}
		log.Info(fmt.Sprintf("Broker %s is already on terminating state", broker.Labels["brokerId"]))
	}
	return true
}

func (r *Reconciler) getServerAndClientDetails() (string, string, []string, error) {
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets == nil {
		return "", "", []string{}, nil
	}
	serverName := types.NamespacedName{Name: fmt.Sprintf(pkicommon.BrokerServerCertTemplate, r.KafkaCluster.Name), Namespace: r.KafkaCluster.Namespace}
	serverSecret := &corev1.Secret{}
	if err := r.Client.Get(context.TODO(), serverName, serverSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", "", nil, errorfactory.New(errorfactory.ResourceNotReady{}, err, "server secret not ready")
		}
		return "", "", nil, errors.WrapIfWithDetails(err, "failed to get server secret")
	}
	serverPass := string(serverSecret.Data[v1alpha1.PasswordKey])

	clientName := types.NamespacedName{Name: fmt.Sprintf(pkicommon.BrokerControllerTemplate, r.KafkaCluster.Name), Namespace: r.KafkaCluster.Namespace}
	clientSecret := &corev1.Secret{}
	if err := r.Client.Get(context.TODO(), clientName, clientSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", "", nil, errorfactory.New(errorfactory.ResourceNotReady{}, err, "client secret not ready")
		}
		return "", "", nil, errors.WrapIfWithDetails(err, "failed to get client secret")
	}
	clientPass := string(clientSecret.Data[v1alpha1.PasswordKey])

	superUsers := make([]string, 0)
	for _, secret := range []*corev1.Secret{serverSecret, clientSecret} {
		cert, err := certutil.DecodeCertificate(secret.Data[corev1.TLSCertKey])
		if err != nil {
			return "", "", nil, errors.WrapIfWithDetails(err, "failed to decode certificate")
		}
		superUsers = append(superUsers, cert.Subject.String())
	}

	return serverPass, clientPass, superUsers, nil
}

func (r *Reconciler) reconcilePerBrokerDynamicConfig(brokerId int32, brokerConfig *v1beta1.BrokerConfig, log logr.Logger) error {
	kClient, err := kafkaclient.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer func() {
		if err := kClient.Close(); err != nil {
			log.Error(err, "could not close client")
		}
	}()

	parsedBrokerConfig := util.ParsePropertiesFormat(brokerConfig.Config)

	// Calling DescribePerBrokerConfig with empty slice will return all config for that broker including the default ones
	if len(parsedBrokerConfig) > 0 {

		brokerConfigKeys := []string{}
		for key := range parsedBrokerConfig {
			brokerConfigKeys = append(brokerConfigKeys, key)
		}
		configIdentical := true

		response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not describe broker config", "brokerId", brokerId)
		}

		if len(response) == 0 {
			configIdentical = false
		}
		for _, conf := range response {
			if val, ok := parsedBrokerConfig[conf.Name]; ok {
				if val != conf.Value {
					configIdentical = false
					break
				}
			}
		}
		if !configIdentical {
			err := kClient.AlterPerBrokerConfig(brokerId, util.ConvertMapStringToMapStringPointer(parsedBrokerConfig))
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not alter broker config", "brokerId", brokerId)
			}
		}
	}
	return nil
}

func (r *Reconciler) reconcileClusterWideDynamicConfig(log logr.Logger) error {
	kClient, err := kafkaclient.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer func() {
		if err := kClient.Close(); err != nil {
			log.Error(err, "could not close client")
		}
	}()

	configIdentical := true

	currentConfig, err := kClient.DescribeClusterWideConfig()
	if err != nil {
		return errors.WrapIf(err, "could not describe cluster wide broker config")
	}
	parsedClusterWideConfig := util.ParsePropertiesFormat(r.KafkaCluster.Spec.ClusterWideConfig)
	if len(currentConfig) != len(parsedClusterWideConfig) {
		configIdentical = false
	}
	for _, conf := range currentConfig {
		if val, ok := parsedClusterWideConfig[conf.Name]; ok {
			if val != conf.Value {
				configIdentical = false
				break
			}
		}
	}
	if !configIdentical {
		err = kClient.AlterClusterWideConfig(util.ConvertMapStringToMapStringPointer(parsedClusterWideConfig))
		if err != nil {
			return errors.WrapIf(err, "could not alter cluster wide broker config")
		}
	}

	return nil
}

func (r *Reconciler) reconcileKafkaPod(log logr.Logger, desiredPod *corev1.Pod) error {
	currentPod := desiredPod.DeepCopy()
	desiredType := reflect.TypeOf(desiredPod)

	log = log.WithValues("kind", desiredType)
	log.V(1).Info("searching with label because name is empty", "brokerId", desiredPod.Labels["brokerId"])

	podList := &corev1.PodList{}

	matchingLabels := client.MatchingLabels(
		util.MergeLabels(
			labelsForKafka(r.KafkaCluster.Name),
			map[string]string{"brokerId": desiredPod.Labels["brokerId"]},
		),
	)
	err := r.Client.List(context.TODO(), podList, client.InNamespace(currentPod.Namespace), matchingLabels)
	if err != nil && len(podList.Items) == 0 {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
	}
	if len(podList.Items) == 0 {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPod); err != nil {
			return errors.WrapIf(err, "could not apply last state to annotation")
		}
		if err := r.Client.Create(context.TODO(), desiredPod); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		// Update status to Config InSync because broker is configured to go
		statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels["brokerId"]}, r.KafkaCluster, v1beta1.ConfigInSync, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating status for resource failed", "kind", desiredType)
		}
		if val, ok := r.KafkaCluster.Status.BrokersState[desiredPod.Labels["brokerId"]]; ok && val.GracefulActionState.CruiseControlState != v1beta1.GracefulUpscaleSucceeded {
			gracefulActionState := v1beta1.GracefulActionState{ErrorMessage: "CruiseControl not yet ready", CruiseControlState: v1beta1.GracefulUpscaleSucceeded}

			if r.KafkaCluster.Status.CruiseControlTopicStatus == v1beta1.CruiseControlTopicReady {
				gracefulActionState = v1beta1.GracefulActionState{ErrorMessage: "", CruiseControlState: v1beta1.GracefulUpdateRequired}
			}
			statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels["brokerId"]}, r.KafkaCluster, gracefulActionState, log)
			if statusErr != nil {
				return errorfactory.New(errorfactory.StatusUpdateError{}, err, "could not update broker graceful action state")
			}
		}

		log.Info("resource created")
		return nil
	} else if len(podList.Items) == 1 {
		currentPod = podList.Items[0].DeepCopy()
		brokerId := currentPod.Labels["brokerId"]
		if _, ok := r.KafkaCluster.Status.BrokersState[brokerId]; ok {
			if r.KafkaCluster.Spec.RackAwareness != nil {
				rackAwarenessState, err := k8sutil.UpdateCrWithRackAwarenessConfig(currentPod, r.KafkaCluster, r.Client)
				if err != nil {
					return err
				}
				statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{brokerId}, r.KafkaCluster, rackAwarenessState, log)
				if statusErr != nil {
					return errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating status for resource failed", "kind", desiredType)
				}
			}
			if currentPod.Status.Phase == corev1.PodRunning && r.KafkaCluster.Status.BrokersState[brokerId].GracefulActionState.CruiseControlState == v1beta1.GracefulUpdateRequired {
				//trigger add broker in CC
				uTaskId, taskStartTime, scaleErr := scale.UpScaleCluster(desiredPod.Labels["brokerId"], desiredPod.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
				if scaleErr != nil {
					log.Info(fmt.Sprintf("cruise control communication error during upscaling broker id: %s", brokerId))
					return errorfactory.New(errorfactory.CruiseControlNotReady{}, scaleErr, fmt.Sprintf("broker id: %s", brokerId))
				}
				statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{brokerId}, r.KafkaCluster,
					v1beta1.GracefulActionState{CruiseControlTaskId: uTaskId, CruiseControlState: v1beta1.GracefulUpdateRunning,
						TaskStarted: taskStartTime}, log)
				if statusErr != nil {
					return errors.WrapIfWithDetails(err, "could not update status for broker", "id", brokerId)
				}
			}
			if r.KafkaCluster.Status.BrokersState[brokerId].GracefulActionState.CruiseControlState == v1beta1.GracefulUpdateRunning {
				err = r.checkCCTaskState([]string{brokerId}, r.KafkaCluster.Status.BrokersState[brokerId], v1beta1.GracefulUpscaleSucceeded, log)
				if err != nil {
					return err
				}
			}
		} else {
			return errorfactory.New(errorfactory.InternalError{}, errors.New("reconcile failed"), fmt.Sprintf("could not find status for the given broker id, %s", brokerId))
		}
	} else {
		return errorfactory.New(errorfactory.TooManyResources{}, errors.New("reconcile failed"), "more then one matching pod found", "labels", matchingLabels)
	}
	// TODO check if this err == nil check necessary (baluchicken)
	if err == nil {
		//Since toleration does not support patchStrategy:"merge,retainKeys", we need to add all toleration from the current pod if the toleration is set in the CR
		if len(desiredPod.Spec.Tolerations) > 0 {
			desiredPod.Spec.Tolerations = append(desiredPod.Spec.Tolerations, currentPod.Spec.Tolerations...)
			uniqueTolerations := []corev1.Toleration{}
			keys := make(map[corev1.Toleration]bool)
			for _, t := range desiredPod.Spec.Tolerations {
				if _, value := keys[t]; !value {
					keys[t] = true
					uniqueTolerations = append(uniqueTolerations, t)
				}
			}
			desiredPod.Spec.Tolerations = uniqueTolerations
		}
		// Check if the resource actually updated
		patchResult, err := patch.DefaultPatchMaker.Calculate(currentPod, desiredPod)
		if err != nil {
			log.Error(err, "could not match objects", "kind", desiredType)
		} else if patchResult.IsEmpty() {
			if !k8sutil.IsPodContainsTerminatedContainer(currentPod) && r.KafkaCluster.Status.BrokersState[currentPod.Labels["brokerId"]].ConfigurationState == v1beta1.ConfigInSync {
				log.V(1).Info("resource is in sync")
				return nil
			}
		} else {
			log.V(1).Info("resource diffs",
				"patch", string(patchResult.Patch),
				"current", string(patchResult.Current),
				"modified", string(patchResult.Modified),
				"original", string(patchResult.Original))
		}

		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPod); err != nil {
			return errors.WrapIf(err, "could not apply last state to annotation")
		}

		if !k8sutil.IsPodContainsTerminatedContainer(currentPod) {

			if r.KafkaCluster.Status.State != v1beta1.KafkaClusterRollingUpgrading {
				if err := k8sutil.UpdateCRStatus(r.Client, r.KafkaCluster, v1beta1.KafkaClusterRollingUpgrading, log); err != nil {
					return errorfactory.New(errorfactory.StatusUpdateError{}, err, "setting state to rolling upgrade failed")
				}
			}

			if r.KafkaCluster.Status.State == v1beta1.KafkaClusterRollingUpgrading {
				// Check if any kafka pod is in terminating or pending state
				podList := &corev1.PodList{}
				matchingLabels := client.MatchingLabels(labelsForKafka(r.KafkaCluster.Name))
				err := r.Client.List(context.TODO(), podList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
				if err != nil {
					return errors.WrapIf(err, "failed to reconcile resource")
				}
				for _, pod := range podList.Items {
					if k8sutil.IsMarkedForDeletion(pod.ObjectMeta) {
						return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("pod is still terminating"), "rolling upgrade in progress")
					}
					if k8sutil.IsPodContainsPendingContainer(&pod) {
						return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("pod is still creating"), "rolling upgrade in progress")
					}
				}

				errorCount := r.KafkaCluster.Status.RollingUpgrade.ErrorCount

				kClient, err := kafkaclient.NewFromCluster(r.Client, r.KafkaCluster)
				if err != nil {
					return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
				}
				defer func() {
					if err := kClient.Close(); err != nil {
						log.Error(err, "could not close client")
					}
				}()
				offlineReplicaCount, err := kClient.OfflineReplicaCount()
				if err != nil {
					return errors.WrapIf(err, "health check failed")
				}
				replicasInSync, err := kClient.AllReplicaInSync()
				if err != nil {
					return errors.WrapIf(err, "health check failed")
				}

				if offlineReplicaCount > 0 || !replicasInSync {
					errorCount++
				}
				if errorCount >= r.KafkaCluster.Spec.RollingUpgradeConfig.FailureThreshold {
					return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("cluster is not healthy"), "rolling upgrade in progress")
				}
			}
		}

		err = r.Client.Delete(context.TODO(), currentPod)
		if err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "deleting resource failed", "kind", desiredType)
		}
	}
	return nil

}

func (r *Reconciler) checkCCTaskState(brokerIds []string, brokerState v1beta1.BrokerState, cruiseControlState v1beta1.CruiseControlState, log logr.Logger) error {

	// check cc task status
	status, err := scale.GetCCTaskState(brokerState.GracefulActionState.CruiseControlTaskId,
		r.KafkaCluster.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
	if err != nil {
		log.Info(fmt.Sprintf("Cruise control communication error checking running task: %s", brokerState.GracefulActionState.CruiseControlTaskId))
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}
	if status == v1beta1.CruiseControlTaskNotFound || status == v1beta1.CruiseControlTaskCompletedWithError {
		// CC task failed or not found in CC,
		// reschedule it by marking broker CruiseControlState=GracefulUpdateRequired
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, r.KafkaCluster,
			v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpdateRequired,
				ErrorMessage: "Previous cc task status invalid",
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return errorfactory.New(errorfactory.CruiseControlTaskFailure{}, err, "CC task failed", fmt.Sprintf("cc task id: %s", brokerState.GracefulActionState.CruiseControlTaskId))
	}
	if status == v1beta1.CruiseControlTaskCompleted {
		// cc task completed successfully
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, r.KafkaCluster,
			v1beta1.GracefulActionState{CruiseControlState: cruiseControlState,
				TaskStarted:         brokerState.GracefulActionState.TaskStarted,
				CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		return nil
	}
	// cc task still in progress
	parsedTime, err := ccutils.ParseTimeStampToUnixTime(brokerState.GracefulActionState.TaskStarted)
	if err != nil {
		return errors.WrapIf(err, "could not parse timestamp")
	}
	if time.Now().Sub(parsedTime).Minutes() < r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlTaskSpec.GetDurationMinutes() {
		err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, r.KafkaCluster,
			v1beta1.GracefulActionState{TaskStarted: brokerState.GracefulActionState.TaskStarted,
				CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
				CruiseControlState:  brokerState.GracefulActionState.CruiseControlState,
			}, log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
		}
		log.Info(fmt.Sprintf("Cruise control task: %s is still running", brokerState.GracefulActionState.CruiseControlTaskId))
		return errorfactory.New(errorfactory.CruiseControlTaskRunning{}, errors.New("cc task is still running"), fmt.Sprintf("cc task id: %s", brokerState.GracefulActionState.CruiseControlTaskId))
	}
	// task timed out
	log.Info(fmt.Sprintf("Killing Cruise control task: %s", brokerState.GracefulActionState.CruiseControlTaskId))
	err = scale.KillCCTask(r.KafkaCluster.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
	if err != nil {
		return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, "cc communication error")
	}
	err = k8sutil.UpdateBrokerStatus(r.Client, brokerIds, r.KafkaCluster,
		v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpdateFailed,
			CruiseControlTaskId: brokerState.GracefulActionState.CruiseControlTaskId,
			ErrorMessage:        "Timed out waiting for the task to complete",
			TaskStarted:         brokerState.GracefulActionState.TaskStarted,
		}, log)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)", strings.Join(brokerIds, ","))
	}
	return errorfactory.New(errorfactory.CruiseControlTaskTimeout{}, errors.New("cc task timed out"), fmt.Sprintf("cc task id: %s", brokerState.GracefulActionState.CruiseControlTaskId))
}

func (r *Reconciler) reconcileKafkaPvc(log logr.Logger, desiredPvc *corev1.PersistentVolumeClaim) error {
	var currentPvc = desiredPvc.DeepCopy()
	desiredType := reflect.TypeOf(desiredPvc)
	log = log.WithValues("kind", desiredType)
	log.V(1).Info("searching with label because name is empty")

	pvcList := &corev1.PersistentVolumeClaimList{}

	matchingLabels := client.MatchingLabels(
		util.MergeLabels(
			labelsForKafka(r.KafkaCluster.Name),
			map[string]string{"brokerId": desiredPvc.Labels["brokerId"]},
		),
	)
	err := r.Client.List(context.TODO(), pvcList,
		client.InNamespace(currentPvc.Namespace), matchingLabels)
	if err != nil && len(pvcList.Items) == 0 {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
	}
	mountPath := currentPvc.Annotations["mountPath"]

	// Creating the first PersistentVolume For Pod
	if len(pvcList.Items) == 0 {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPvc); err != nil {
			return errors.WrapIf(err, "could not apply last state to annotation")
		}
		if err := r.Client.Create(context.TODO(), desiredPvc); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		log.Info("resource created")
		return nil
	}
	alreadyCreated := false
	for _, pvc := range pvcList.Items {
		if mountPath == pvc.Annotations["mountPath"] {
			currentPvc = pvc.DeepCopy()
			alreadyCreated = true
			break
		}
	}
	if !alreadyCreated {
		// Creating the 2+ PersistentVolumes for Pod
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPvc); err != nil {
			return errors.WrapIf(err, "could not apply last state to annotation")
		}
		if err := r.Client.Create(context.TODO(), desiredPvc); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		return nil
	}
	if err == nil {
		if k8sutil.CheckIfObjectUpdated(log, desiredType, currentPvc, desiredPvc) {

			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPvc); err != nil {
				return errors.WrapIf(err, "could not apply last state to annotation")
			}
			desiredPvc = currentPvc

			if err := r.Client.Update(context.TODO(), desiredPvc); err != nil {
				return errorfactory.New(errorfactory.APIFailure{}, err, "updating resource failed", "kind", desiredType)
			}
			log.Info("resource updated")
		}
	}
	return nil
}

// GetBrokersWithPendingOrRunningCCTask returns list of brokers that are either waiting for CC
// to start executing a broker task (add broker, remove broker, etc) or CC already running a task for it.
func GetBrokersWithPendingOrRunningCCTask(kafkaCluster *v1beta1.KafkaCluster) []int32 {
	var brokerIDs []int32
	for i := range kafkaCluster.Spec.Brokers {
		if state, ok := kafkaCluster.Status.BrokersState[strconv.Itoa(int(kafkaCluster.Spec.Brokers[i].Id))]; ok {
			if state.GracefulActionState.CruiseControlState == v1beta1.GracefulUpdateRequired || (state.GracefulActionState.CruiseControlTaskId != "" && state.GracefulActionState.CruiseControlState == v1beta1.GracefulUpdateRunning) {
				brokerIDs = append(brokerIDs, kafkaCluster.Spec.Brokers[i].Id)
			}
		}
	}
	return brokerIDs
}
