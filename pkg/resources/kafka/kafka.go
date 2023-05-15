// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"sort"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiutil "github.com/banzaicloud/koperator/api/util"

	"github.com/banzaicloud/k8s-objectmatcher/patch"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
	"github.com/banzaicloud/koperator/pkg/jmxextractor"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/pki"
	"github.com/banzaicloud/koperator/pkg/resources"
	"github.com/banzaicloud/koperator/pkg/resources/templates"
	"github.com/banzaicloud/koperator/pkg/scale"
	"github.com/banzaicloud/koperator/pkg/util"
	certutil "github.com/banzaicloud/koperator/pkg/util/cert"
	envoyutils "github.com/banzaicloud/koperator/pkg/util/envoy"
	istioingressutils "github.com/banzaicloud/koperator/pkg/util/istioingress"
	"github.com/banzaicloud/koperator/pkg/util/kafka"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"
)

// brokerReconcilePriority lower value represents higher priority for a broker to be reconciled
type brokerReconcilePriority int

const (
	componentName         = "kafka"
	brokerConfigTemplate  = "%s-config"
	brokerStorageTemplate = "%s-%d-storage-%d-"

	brokerConfigMapVolumeMount = "broker-config"
	kafkaDataVolumeMount       = "kafka-data"

	serverKeystorePath   = "/var/run/secrets/java.io/keystores/server"
	clientKeystoreVolume = "client-ks-files"
	clientKeystorePath   = "/var/run/secrets/java.io/keystores/client"

	listenerSSLCertVolumeNameTemplate  = "listener-%s-certs"
	listenerServerKeyStorePathTemplate = "%s/%s"

	jmxVolumePath      = "/opt/jmx-exporter/"
	jmxVolumeName      = "jmx-jar-data"
	MetricsHealthCheck = "/-/healthy"
	MetricsPort        = 9020

	// missingBrokerDownScaleRunningPriority the priority is used  for missing brokers where there is an incomplete downscale operation
	missingBrokerDownScaleRunningPriority brokerReconcilePriority = iota
	// newBrokerReconcilePriority the priority used  for brokers that were just added to the cluster used to define its priority in the reconciliation order
	newBrokerReconcilePriority
	// missingBrokerReconcilePriority the priority used for missing brokers used to define its priority in the reconciliation order
	missingBrokerReconcilePriority
	// nonControllerBrokerReconcilePriority the priority used for running non-controller brokers used to define its priority in the reconciliation order
	nonControllerBrokerReconcilePriority
	// controllerBrokerReconcilePriority the priority used for controller broker used to define its priority in the reconciliation order
	controllerBrokerReconcilePriority
)

// Reconciler implements the Component Reconciler
type Reconciler struct {
	resources.Reconciler
	kafkaClientProvider kafkaclient.Provider
}

// New creates a new reconciler for Kafka
func New(client client.Client, directClient client.Reader, cluster *v1beta1.KafkaCluster, kafkaClientProvider kafkaclient.Provider) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client:       client,
			DirectClient: directClient,
			KafkaCluster: cluster,
		},
		kafkaClientProvider: kafkaClientProvider,
	}
}
func getCreatedPvcForBroker(
	ctx context.Context,
	c client.Reader,
	brokerID int32,
	storageConfigs []v1beta1.StorageConfig,
	namespace, crName string) ([]corev1.PersistentVolumeClaim, error) {
	foundPvcList := &corev1.PersistentVolumeClaimList{}
	matchingLabels := client.MatchingLabels(
		apiutil.MergeLabels(
			apiutil.LabelsForKafka(crName),
			map[string]string{v1beta1.BrokerIdLabelKey: fmt.Sprintf("%d", brokerID)},
		),
	)
	err := c.List(ctx, foundPvcList, client.ListOption(client.InNamespace(namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return nil, err
	}

	var missing []string
	for i := range storageConfigs {
		if storageConfigs[i].PvcSpec == nil {
			continue
		}

		found := false
		for j := range foundPvcList.Items {
			if storageConfigs[i].MountPath == foundPvcList.Items[j].GetAnnotations()["mountPath"] {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, storageConfigs[i].MountPath)
		}
	}
	if len(missing) > 0 {
		return nil, errors.NewWithDetails("broker mount paths missing persistent volume claim",
			v1beta1.BrokerIdLabelKey, brokerID, "mount paths", missing)
	}
	sort.Slice(foundPvcList.Items, func(i, j int) bool {
		return foundPvcList.Items[i].Name < foundPvcList.Items[j].Name
	})
	return foundPvcList.Items, nil
}

func getLoadBalancerIP(foundLBService *corev1.Service) (string, error) {
	if len(foundLBService.Status.LoadBalancer.Ingress) == 0 {
		return "", errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("loadbalancer ingress is not created waiting"), "trying")
	}

	if foundLBService.Status.LoadBalancer.Ingress[0].Hostname == "" && foundLBService.Status.LoadBalancer.Ingress[0].IP == "" {
		return "", errorfactory.New(errorfactory.LoadBalancerIPNotReady{}, errors.New("loadbalancer hostname and IP has not been created yet - waiting"), "trying")
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
//
//gocyclo:ignore
//nolint:funlen
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName, "clusterName", r.KafkaCluster.Name, "clusterNamespace", r.KafkaCluster.Namespace)

	log.V(1).Info("Reconciling")

	ctx := context.Background()
	if err := k8sutil.UpdateBrokerConfigurationBackup(r.Client, r.KafkaCluster); err != nil {
		log.Error(err, "failed to update broker configuration backup")
	}

	if r.KafkaCluster.Spec.HeadlessServiceEnabled {
		// reconcile headless service
		o := r.headlessService()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	} else {
		// reconcile all-broker service
		o := r.allBrokerService()
		err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	// Handle PDB
	if r.KafkaCluster.Spec.DisruptionBudget.Create {
		o, err := r.podDisruptionBudget(log)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to compute podDisruptionBudget")
		}
		err = k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	// Handle Pod delete
	err := r.reconcileKafkaPodDelete(ctx, log)
	if err != nil {
		return errors.WrapIf(err, "failed to reconcile resource")
	}

	extListenerStatuses, err := r.createExternalListenerStatuses(log)
	if err != nil {
		return errors.WrapIf(err, "could not update status for external listeners")
	}
	intListenerStatuses, controllerIntListenerStatuses := k8sutil.CreateInternalListenerStatuses(r.KafkaCluster, extListenerStatuses)
	err = k8sutil.UpdateListenerStatuses(ctx, r.Client, r.KafkaCluster, intListenerStatuses, extListenerStatuses)
	if err != nil {
		return errors.WrapIf(err, "failed to update listener statuses")
	}

	// Setup the PKI if using SSL
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil {
		// reconcile the PKI
		if err := pki.GetPKIManager(r.Client, r.KafkaCluster, v1beta1.PKIBackendProvided).ReconcilePKI(ctx, extListenerStatuses); err != nil {
			return err
		}
	}

	// We need to grab names for servers and client in case user is enabling ACLs
	// That way we can continue to manage topics and users
	clientPass, serverPasses, superUsers, err := r.getPasswordKeysAndSuperUsers()
	if err != nil {
		return err
	}

	brokersVolumes := make(map[string][]*corev1.PersistentVolumeClaim, len(r.KafkaCluster.Spec.Brokers))
	for _, broker := range r.KafkaCluster.Spec.Brokers {
		brokerConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
		if err != nil {
			return errors.WrapIf(err, "failed to reconcile resource")
		}

		var brokerVolumes []*corev1.PersistentVolumeClaim
		for index, storage := range brokerConfig.StorageConfigs {
			if storage.PvcSpec == nil && storage.EmptyDir == nil {
				return errors.WrapIfWithDetails(err,
					"invalid storage config, either 'pvcSpec' or 'emptyDir` has to be set",
					v1beta1.BrokerIdLabelKey, broker.Id, "mountPath", storage.MountPath)
			}
			if storage.PvcSpec == nil {
				continue
			}
			o, err := r.pvc(broker.Id, index, storage)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to generate resource", "resources", "PersistentVolumeClaim")
			}
			brokerVolumes = append(brokerVolumes, o)
		}
		if len(brokerVolumes) > 0 {
			brokersVolumes[strconv.Itoa(int(broker.Id))] = brokerVolumes
		}
	}
	if len(brokersVolumes) > 0 {
		err := r.reconcileKafkaPvc(ctx, log, brokersVolumes)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resources", "PersistentVolumeClaim")
		}
	}

	var brokerPods corev1.PodList
	matchingLabels := client.MatchingLabels(apiutil.LabelsForKafka(r.KafkaCluster.Name))
	err = r.Client.List(ctx, &brokerPods, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return errors.WrapIf(err, "failed to list broker pods that belong to Kafka cluster")
	}

	runningBrokers := make(map[string]struct{})
	for _, b := range brokerPods.Items {
		brokerID := b.GetLabels()[v1beta1.BrokerIdLabelKey]
		runningBrokers[brokerID] = struct{}{}
	}

	controllerID, err := r.determineControllerId()
	if err != nil {
		log.Error(err, "could not find controller broker")
	}

	var pvcList corev1.PersistentVolumeClaimList
	err = r.Client.List(ctx, &pvcList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return errors.WrapIf(err, "failed to list broker pvcs that belong to Kafka cluster")
	}

	boundPersistentVolumeClaims := make(map[string]struct{})
	for _, pvc := range pvcList.Items {
		brokerID := pvc.GetLabels()[v1beta1.BrokerIdLabelKey]
		if pvc.Status.Phase == corev1.ClaimBound {
			boundPersistentVolumeClaims[brokerID] = struct{}{}
		}
	}

	reorderedBrokers := reorderBrokers(runningBrokers, boundPersistentVolumeClaims, r.KafkaCluster.Spec.Brokers, r.KafkaCluster.Status.BrokersState, controllerID, log)
	allBrokerDynamicConfigSucceeded := true
	for _, broker := range reorderedBrokers {
		brokerConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
		if err != nil {
			return errors.WrapIf(err, "failed to reconcile resource")
		}

		var configMap *corev1.ConfigMap
		if r.KafkaCluster.Spec.RackAwareness == nil {
			configMap = r.configMap(broker.Id, brokerConfig, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPasses, clientPass, superUsers, log)
			err := k8sutil.Reconcile(log, r.Client, configMap, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", configMap.GetObjectKind().GroupVersionKind())
			}
		} else if brokerState, ok := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(broker.Id))]; ok {
			if brokerState.RackAwarenessState != "" {
				configMap = r.configMap(broker.Id, brokerConfig, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPasses, clientPass, superUsers, log)
				err := k8sutil.Reconcile(log, r.Client, configMap, r.KafkaCluster)
				if err != nil {
					return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", configMap.GetObjectKind().GroupVersionKind())
				}
			}
		}

		pvcs, err := getCreatedPvcForBroker(ctx, r.Client, broker.Id, brokerConfig.StorageConfigs, r.KafkaCluster.Namespace, r.KafkaCluster.Name)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to list PVC's")
		}

		if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
			o := r.service(broker.Id, brokerConfig)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		}
		o := r.pod(broker.Id, brokerConfig, pvcs, log)
		err = r.reconcileKafkaPod(log, o.(*corev1.Pod), brokerConfig)
		if err != nil {
			return err
		}
		if err = r.updateStatusWithDockerImageAndVersion(broker.Id, brokerConfig, log); err != nil {
			return err
		}
		// If dynamic configs can not be set then let the loop continue to the next broker,
		// after the loop we return error. This solves that case when other brokers could get healthy,
		// but the loop exits too soon because dynamic configs can not be set.
		err = r.reconcilePerBrokerDynamicConfig(broker.Id, brokerConfig, configMap, log)
		if err != nil {
			log.Error(err, "setting dynamic configs has failed", v1beta1.BrokerIdLabelKey, broker.Id)
			allBrokerDynamicConfigSucceeded = false
		}
	}

	if !allBrokerDynamicConfigSucceeded {
		// re-reconcile to retry setting the dynamic configs
		return errors.NewWithDetails("setting dynamic configs for some brokers has failed",
			"component", componentName,
			"clusterName", r.KafkaCluster.Name,
			"clusterNamespace", r.KafkaCluster.Namespace)
	}

	if err = r.reconcileClusterWideDynamicConfig(); err != nil {
		return err
	}

	// in case HeadlessServiceEnabled is changed, delete the service that was created by the previous
	// reconcile flow. The services must be deleted at the end of the reconcile flow after the new services
	// were created and broker configurations reflecting the new services otherwise the Kafka brokers
	// won't be reachable by koperator.
	if r.KafkaCluster.Spec.HeadlessServiceEnabled {
		log.V(1).Info("deleting non-headless services for all of the brokers")

		if err := r.deleteNonHeadlessServices(ctx); err != nil {
			return errors.WrapIfWithDetails(err, "failed to delete non-headless services for all of the brokers",
				"component", componentName,
				"clusterName", r.KafkaCluster.Name,
				"clusterNamespace", r.KafkaCluster.Namespace)
		}
	} else {
		// delete headless service
		log.V(1).Info("deleting headless service")
		if err := r.deleteHeadlessService(); err != nil {
			return errors.WrapIfWithDetails(err, "failed to delete headless service",
				"component", componentName,
				"clusterName", r.KafkaCluster.Name,
				"clusterNamespace", r.KafkaCluster.Namespace)
		}
	}

	log.V(1).Info("Reconciled")

	return nil
}

func (r *Reconciler) reconcileKafkaPodDelete(ctx context.Context, log logr.Logger) error {
	podList := &corev1.PodList{}
	err := r.Client.List(context.TODO(), podList,
		client.InNamespace(r.KafkaCluster.Namespace),
		client.MatchingLabels(apiutil.LabelsForKafka(r.KafkaCluster.Name)),
	)
	if err != nil {
		return errors.WrapIf(err, "failed to reconcile resource")
	}

	brokerIDsFromSpec := make(map[string]bool, len(r.KafkaCluster.Spec.Brokers))
	for _, broker := range r.KafkaCluster.Spec.Brokers {
		brokerIDsFromSpec[strconv.Itoa(int(broker.Id))] = true
	}

	podsDeletedFromSpec := make([]corev1.Pod, 0, len(podList.Items))
	brokerIDsDeletedFromSpec := make(map[string]bool)
	for _, pod := range podList.Items {
		id, ok := pod.Labels[v1beta1.BrokerIdLabelKey]
		if !ok {
			continue
		}
		if _, ok = brokerIDsFromSpec[id]; !ok {
			brokerIDsDeletedFromSpec[id] = true
			podsDeletedFromSpec = append(podsDeletedFromSpec, pod)
		}
	}

	if len(podsDeletedFromSpec) > 0 {
		if !arePodsAlreadyDeleted(podsDeletedFromSpec, log) {
			cruiseControlURL := scale.CruiseControlURLFromKafkaCluster(r.KafkaCluster)
			// FIXME: we should reuse the context of the Kafka Controller
			cc, err := scale.NewCruiseControlScaler(context.TODO(), scale.CruiseControlURLFromKafkaCluster(r.KafkaCluster))
			if err != nil {
				return errorfactory.New(errorfactory.CruiseControlNotReady{}, err,
					"failed to initialize Cruise Control Scaler", "cruise control url", cruiseControlURL)
			}

			brokerStates := []scale.KafkaBrokerState{
				scale.KafkaBrokerNew,
				scale.KafkaBrokerAlive,
				scale.KafkaBrokerDemoted,
				scale.KafkaBrokerBadDisks,
			}
			availableBrokers, err := cc.BrokersWithState(ctx, brokerStates...)
			if err != nil {
				log.Error(err, "failed to get the list of available brokers from Cruise Control")
				return errorfactory.New(errorfactory.CruiseControlNotReady{}, err,
					"failed to get the list of available brokers from Cruise Control")
			}

			brokersToUpdateInStatus := make([]string, 0, len(availableBrokers))
			for _, brokerID := range availableBrokers {
				if _, ok := brokerIDsDeletedFromSpec[brokerID]; ok {
					brokersToUpdateInStatus = append(brokersToUpdateInStatus, brokerID)
				}
			}

			if len(brokersToUpdateInStatus) == 0 {
				log.Info("no brokers found in Cruise Control which marked for deletion. No action is needed")
			} else {
				var brokersPendingGracefulDownscale []string

				for _, brokerID := range brokersToUpdateInStatus {
					if brokerState, ok := r.KafkaCluster.Status.BrokersState[brokerID]; ok {
						ccState := brokerState.GracefulActionState.CruiseControlState
						if ccState == v1beta1.GracefulUpscaleSucceeded || ccState == v1beta1.GracefulUpscaleRequired {
							brokersPendingGracefulDownscale = append(brokersPendingGracefulDownscale, brokerID)
						}
					}
				}

				if len(brokersPendingGracefulDownscale) > 0 {
					err = k8sutil.UpdateBrokerStatus(r.Client, brokersPendingGracefulDownscale, r.KafkaCluster,
						v1beta1.GracefulActionState{
							CruiseControlState: v1beta1.GracefulDownscaleRequired,
						}, log)
					if err != nil {
						return errors.WrapIfWithDetails(err, "could not update status for broker(s)", "id(s)",
							strings.Join(brokersPendingGracefulDownscale, ","))
					}
				}
			}
		}

		for _, broker := range podsDeletedFromSpec {
			broker := broker
			if broker.ObjectMeta.DeletionTimestamp != nil {
				log.Info(fmt.Sprintf("Broker %s is already on terminating state", broker.Labels[v1beta1.BrokerIdLabelKey]))
				continue
			}

			if brokerState, ok := r.KafkaCluster.Status.BrokersState[broker.Labels[v1beta1.BrokerIdLabelKey]]; ok &&
				brokerState.GracefulActionState.CruiseControlState != v1beta1.GracefulDownscaleSucceeded &&
				brokerState.GracefulActionState.CruiseControlState != v1beta1.GracefulUpscaleRequired {
				if brokerState.GracefulActionState.CruiseControlState == v1beta1.GracefulDownscaleRunning {
					log.Info("cc task is still running for broker", v1beta1.BrokerIdLabelKey, broker.Labels[v1beta1.BrokerIdLabelKey], "CruiseControlOperationReference", brokerState.GracefulActionState.CruiseControlOperationReference)
				}
				continue
			}

			err = r.Client.Delete(context.TODO(), &broker)
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete broker", "id", broker.Labels[v1beta1.BrokerIdLabelKey])
			}
			log.Info("broker pod deleted", v1beta1.BrokerIdLabelKey, broker.Labels[v1beta1.BrokerIdLabelKey], "pod", broker.GetName())
			configMapName := fmt.Sprintf(brokerConfigTemplate+"-%s", r.KafkaCluster.Name, broker.Labels[v1beta1.BrokerIdLabelKey])
			err = r.Client.Delete(context.TODO(), &corev1.ConfigMap{ObjectMeta: templates.ObjectMeta(configMapName, apiutil.LabelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// can happen when broker was not fully initialized and now is deleted
					log.Info(fmt.Sprintf("ConfigMap for Broker %s not found. Continue", broker.Labels[v1beta1.BrokerIdLabelKey]))
				} else {
					return errors.WrapIfWithDetails(err, "could not delete configmap for broker", "id", broker.Labels[v1beta1.BrokerIdLabelKey])
				}
			} else {
				log.V(1).Info("configMap for broker deleted", "configMap name", configMapName, v1beta1.BrokerIdLabelKey, broker.Labels[v1beta1.BrokerIdLabelKey])
			}
			if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
				serviceName := fmt.Sprintf("%s-%s", r.KafkaCluster.Name, broker.Labels[v1beta1.BrokerIdLabelKey])
				err = r.Client.Delete(context.TODO(), &corev1.Service{ObjectMeta: templates.ObjectMeta(serviceName, apiutil.LabelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
				if err != nil {
					if apierrors.IsNotFound(err) {
						// can happen when broker was not fully initialized and now is deleted
						log.Info(fmt.Sprintf("Service for Broker %s not found. Continue", broker.Labels[v1beta1.BrokerIdLabelKey]))
					}
					return errors.WrapIfWithDetails(err, "could not delete service for broker", "id", broker.Labels[v1beta1.BrokerIdLabelKey])
				}
				log.V(1).Info("service for broker deleted", "service name", serviceName, v1beta1.BrokerIdLabelKey, broker.Labels[v1beta1.BrokerIdLabelKey])
			}
			for _, volume := range broker.Spec.Volumes {
				if strings.HasPrefix(volume.Name, kafkaDataVolumeMount) {
					err = r.Client.Delete(context.TODO(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
						Name:      volume.PersistentVolumeClaim.ClaimName,
						Namespace: r.KafkaCluster.Namespace,
					}})
					if err != nil {
						if apierrors.IsNotFound(err) {
							// can happen when broker was not fully initialized and now is deleted
							log.Info(fmt.Sprintf("PVC for Broker %s not found. Continue", broker.Labels[v1beta1.BrokerIdLabelKey]))
						}
						return errors.WrapIfWithDetails(err, "could not delete pvc for broker", "id", broker.Labels[v1beta1.BrokerIdLabelKey])
					}
					log.V(1).Info("pvc for broker deleted", "pvc name", volume.PersistentVolumeClaim.ClaimName, v1beta1.BrokerIdLabelKey, broker.Labels[v1beta1.BrokerIdLabelKey])
				}
			}
			err = k8sutil.DeleteBrokerStatus(r.Client, broker.Labels[v1beta1.BrokerIdLabelKey], r.KafkaCluster, log)
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete status for broker", "id", broker.Labels[v1beta1.BrokerIdLabelKey])
			}
		}
	}
	return nil
}

func arePodsAlreadyDeleted(pods []corev1.Pod, log logr.Logger) bool {
	for _, broker := range pods {
		if broker.ObjectMeta.DeletionTimestamp == nil {
			return false
		}
		log.Info(fmt.Sprintf("Broker %s is already on terminating state", broker.Labels[v1beta1.BrokerIdLabelKey]))
	}
	return true
}
func (r *Reconciler) getClientPasswordKeyAndUser() (string, string, error) {
	var clientPass, CN string
	if r.KafkaCluster.Spec.IsClientSSLSecretPresent() {
		// Use that secret as default which has autogenerated for clients by us
		clientNamespacedName := types.NamespacedName{Name: fmt.Sprintf(pkicommon.BrokerControllerTemplate, r.KafkaCluster.Name), Namespace: r.KafkaCluster.Namespace}
		if r.KafkaCluster.Spec.GetClientSSLCertSecretName() != "" {
			clientNamespacedName = types.NamespacedName{Name: r.KafkaCluster.Spec.GetClientSSLCertSecretName(), Namespace: r.KafkaCluster.Namespace}
		}
		clientSecret := &corev1.Secret{}
		if err := r.Client.Get(context.TODO(), clientNamespacedName, clientSecret); err != nil {
			if apierrors.IsNotFound(err) && r.KafkaCluster.Spec.GetClientSSLCertSecretName() == "" {
				return "", "", errorfactory.New(errorfactory.ResourceNotReady{}, err, "client secret not ready")
			}
			return "", "", errors.WrapIf(err, "failed to get client secret")
		}
		if err := certutil.CheckSSLCertSecret(clientSecret); err != nil {
			if r.KafkaCluster.Spec.GetClientSSLCertSecretName() != "" {
				return "", "", err
			}
			return "", "", errorfactory.New(errorfactory.ResourceNotReady{}, errors.Errorf("SSL JKS certificate has not generated properly yet into secret: %s", clientSecret.Name), "checking secret data fields")
		}

		tlsCert, err := certutil.ParseKeyStoreToTLSCertificate(clientSecret.Data[v1alpha1.TLSJKSKeyStore], clientSecret.Data[v1alpha1.PasswordKey])
		if err != nil {
			return "", "", errors.WrapIfWithDetails(err, "failed to decode certificate", "secretName", clientSecret.Name)
		}
		CN = tlsCert.Leaf.Subject.String()
		clientPass = string(clientSecret.Data[v1alpha1.PasswordKey])
	}
	return clientPass, CN, nil
}

func getListenerSSLCertSecret(client client.Reader, commonSpec v1beta1.CommonListenerSpec, clusterName string, clusterNamespace string) (*corev1.Secret, error) {
	// Use default SSL cert secret
	secretNamespacedName := types.NamespacedName{Name: fmt.Sprintf(pkicommon.BrokerServerCertTemplate, clusterName), Namespace: clusterNamespace}
	if commonSpec.GetServerSSLCertSecretName() != "" {
		secretNamespacedName = types.NamespacedName{Name: commonSpec.GetServerSSLCertSecretName(), Namespace: clusterNamespace}
	}
	serverSecret := &corev1.Secret{}
	if err := client.Get(context.TODO(), secretNamespacedName, serverSecret); err != nil {
		if apierrors.IsNotFound(err) && commonSpec.GetServerSSLCertSecretName() == "" {
			return nil, errorfactory.New(errorfactory.ResourceNotReady{}, err, "server secret not ready")
		}
		return nil, errors.WrapIfWithDetails(err, "failed to get server secret")
	}
	// Check secret data fields
	if err := certutil.CheckSSLCertSecret(serverSecret); err != nil {
		if commonSpec.GetServerSSLCertSecretName() != "" {
			return nil, err
		}
		return nil, errorfactory.New(errorfactory.ResourceNotReady{}, errors.Errorf("SSL JKS certificate has not generated properly yet into secret: %s", serverSecret.Name), "checking secret data fields")
	}

	return serverSecret, nil
}

func (r *Reconciler) getServerPasswordKeysAndUsers() (map[string]string, []string, error) {
	// pair contains the listenerName and the corresponding  JKS cert password
	pair := make(map[string]string)

	var CNList []string
	// globKeyPass contains the generated ssl keystore password.
	var globKeyPass string
	var err error
	serverSecret := &corev1.Secret{}
	for _, iListener := range r.KafkaCluster.Spec.ListenersConfig.InternalListeners {
		if iListener.Type == v1beta1.SecurityProtocolSSL {
			// This implementation logic gets the generated ssl secret only once even
			// if multiple listener use the generated one, because they share the same.
			if globKeyPass == "" || iListener.GetServerSSLCertSecretName() != "" {
				// get the appropriate secret: the generated as default or the custom if its specified
				serverSecret, err = getListenerSSLCertSecret(r.Client, iListener.CommonListenerSpec, r.KafkaCluster.Name, r.KafkaCluster.Namespace)
				if err != nil {
					return nil, nil, err
				}

				pair[iListener.Name] = string(serverSecret.Data[v1alpha1.PasswordKey])
			}

			// Set the globKeyPass if there is no custom server cert present
			if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && iListener.GetServerSSLCertSecretName() == "" {
				if globKeyPass == "" {
					globKeyPass = string(serverSecret.Data[v1alpha1.PasswordKey])
				}
				pair[iListener.Name] = globKeyPass
			}
			// We need to grab names for servers and client in case user is enabling ACLs
			// That way we can continue to manage topics and users
			// We put these Common Names from certificates into the superusers kafka broker config
			if iListener.UsedForControllerCommunication || iListener.UsedForInnerBrokerCommunication {
				tlsCert, err := certutil.ParseKeyStoreToTLSCertificate(serverSecret.Data[v1alpha1.TLSJKSKeyStore], serverSecret.Data[v1alpha1.PasswordKey])
				if err != nil {
					return nil, nil, errors.WrapIfWithDetails(err, fmt.Sprintf("failed to decode certificate, secretName: %s", serverSecret.Name))
				}
				CNList = append(CNList, tlsCert.Leaf.Subject.String())
			}
		}
	}
	// Same as at the internalListeners except we dont need to collect Common Names from certificates.
	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		if eListener.Type == v1beta1.SecurityProtocolSSL {
			if globKeyPass == "" || eListener.GetServerSSLCertSecretName() != "" {
				serverSecret, err = getListenerSSLCertSecret(r.Client, eListener.CommonListenerSpec, r.KafkaCluster.Name, r.KafkaCluster.Namespace)
				if err != nil {
					return nil, nil, err
				}
				pair[eListener.Name] = string(serverSecret.Data[v1alpha1.PasswordKey])
			}

			if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil && eListener.GetServerSSLCertSecretName() == "" {
				if globKeyPass == "" {
					globKeyPass = string(serverSecret.Data[v1alpha1.PasswordKey])
				}
				pair[eListener.Name] = globKeyPass
			}
		}
	}
	return pair, CNList, nil
}
func (r *Reconciler) getPasswordKeysAndSuperUsers() (clientPass string, serverPasses map[string]string, superUsers []string, err error) {
	serverPasses, superUsers, err = r.getServerPasswordKeysAndUsers()
	if err != nil {
		return "", nil, nil, err
	}
	clientPass, superUser, err := r.getClientPasswordKeyAndUser()
	if err != nil {
		return "", nil, nil, err
	}
	if superUser != "" {
		superUsers = append(superUsers, superUser)
	}
	return clientPass, serverPasses, superUsers, nil
}

func (r *Reconciler) reconcileKafkaPod(log logr.Logger, desiredPod *corev1.Pod, bConfig *v1beta1.BrokerConfig) error {
	currentPod := desiredPod.DeepCopy()
	desiredType := reflect.TypeOf(desiredPod)

	log = log.WithValues("kind", desiredType)
	log.V(1).Info("searching with label because name is empty", v1beta1.BrokerIdLabelKey, desiredPod.Labels[v1beta1.BrokerIdLabelKey])

	podList := &corev1.PodList{}

	matchingLabels := client.MatchingLabels(
		apiutil.MergeLabels(
			apiutil.LabelsForKafka(r.KafkaCluster.Name),
			map[string]string{v1beta1.BrokerIdLabelKey: desiredPod.Labels[v1beta1.BrokerIdLabelKey]},
		),
	)
	err := r.Client.List(context.TODO(), podList, client.InNamespace(currentPod.Namespace), matchingLabels)
	if err != nil && len(podList.Items) == 0 {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
	}
	switch {
	case len(podList.Items) == 0:
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPod); err != nil {
			return errors.WrapIf(err, "could not apply last state to annotation")
		}

		if err := r.Client.Create(context.TODO(), desiredPod); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		// Update status what externalListener configs are in use
		var externalConfigNames v1beta1.ExternalListenerConfigNames
		if len(bConfig.BrokerIngressMapping) > 0 {
			externalConfigNames = bConfig.BrokerIngressMapping
		} else {
			for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
				if eListener.Config != nil {
					externalConfigNames = append(externalConfigNames, eListener.Config.DefaultIngressConfig)
				}
			}
		}
		statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels[v1beta1.BrokerIdLabelKey]},
			r.KafkaCluster, externalConfigNames, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating status for resource failed", "kind", desiredType)
		}
		// Update status to Config InSync because broker is configured to go
		statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels[v1beta1.BrokerIdLabelKey]}, r.KafkaCluster, v1beta1.ConfigInSync, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating status for resource failed", "kind", desiredType)
		}
		// Update status to per-broker Config InSync because broker is configured to go
		log.V(1).Info("setting per broker config status to in sync")
		statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels[v1beta1.BrokerIdLabelKey]}, r.KafkaCluster, v1beta1.PerBrokerConfigInSync, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating per broker config status for resource failed", "kind", desiredType)
		}

		if val, hasBrokerState := r.KafkaCluster.Status.BrokersState[desiredPod.Labels[v1beta1.BrokerIdLabelKey]]; hasBrokerState {
			ccState := val.GracefulActionState.CruiseControlState
			if ccState != v1beta1.GracefulUpscaleSucceeded && !ccState.IsDownscale() {
				gracefulActionState := v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded}

				if r.KafkaCluster.Status.CruiseControlTopicStatus == v1beta1.CruiseControlTopicReady {
					gracefulActionState = v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleRequired}
				}
				statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels[v1beta1.BrokerIdLabelKey]}, r.KafkaCluster, gracefulActionState, log)
				if statusErr != nil {
					return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "could not update broker graceful action state")
				}
			}
		}
		log.Info("resource created")
		return nil
	case len(podList.Items) == 1:
		currentPod = podList.Items[0].DeepCopy()
		brokerId := currentPod.Labels[v1beta1.BrokerIdLabelKey]
		if _, ok := r.KafkaCluster.Status.BrokersState[brokerId]; ok {
			if currentPod.Spec.NodeName == "" {
				log.Info(fmt.Sprintf("pod for brokerId %s does not scheduled to node yet", brokerId))
			} else if r.KafkaCluster.Spec.RackAwareness != nil {
				rackAwarenessState, err := k8sutil.UpdateCrWithRackAwarenessConfig(currentPod, r.KafkaCluster, r.Client, r.DirectClient)
				if err != nil {
					return err
				}
				statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{brokerId}, r.KafkaCluster, rackAwarenessState, log)
				if statusErr != nil {
					return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating status for resource failed", "kind", desiredType)
				}
			}
		} else {
			return errorfactory.New(errorfactory.InternalError{}, errors.New("reconcile failed"), fmt.Sprintf("could not find status for the given broker id, %s", brokerId))
		}
	default:
		return errorfactory.New(errorfactory.TooManyResources{}, errors.New("reconcile failed"), "more then one matching pod found", "labels", matchingLabels)
	}
	err = r.handleRollingUpgrade(log, desiredPod, currentPod, desiredType)
	if err != nil {
		return errors.Wrap(err, "could not handle rolling upgrade")
	}
	return nil
}

func (r *Reconciler) updateStatusWithDockerImageAndVersion(brokerId int32, brokerConfig *v1beta1.BrokerConfig,
	log logr.Logger) error {
	jmxExp := jmxextractor.NewJMXExtractor(r.KafkaCluster.GetNamespace(),
		r.KafkaCluster.Spec.GetKubernetesClusterDomain(), r.KafkaCluster.GetName(), log)

	kafkaVersion, err := jmxExp.ExtractDockerImageAndVersion(brokerId, brokerConfig,
		r.KafkaCluster.Spec.GetClusterImage(), r.KafkaCluster.Spec.HeadlessServiceEnabled)
	if err != nil {
		return err
	}
	err = k8sutil.UpdateBrokerStatus(r.Client, []string{strconv.Itoa(int(brokerId))}, r.KafkaCluster,
		*kafkaVersion, log)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) handleRollingUpgrade(log logr.Logger, desiredPod, currentPod *corev1.Pod, desiredType reflect.Type) error {
	// Since toleration does not support patchStrategy:"merge,retainKeys",
	// we need to add all toleration from the current pod if the toleration is set in the CR
	if len(desiredPod.Spec.Tolerations) > 0 {
		desiredPod.Spec.Tolerations = append(desiredPod.Spec.Tolerations, currentPod.Spec.Tolerations...)
		uniqueTolerations := make([]corev1.Toleration, 0, len(desiredPod.Spec.Tolerations))
		keys := make(map[corev1.Toleration]bool)
		for _, t := range desiredPod.Spec.Tolerations {
			if _, value := keys[t]; !value {
				keys[t] = true
				uniqueTolerations = append(uniqueTolerations, t)
			}
		}
		desiredPod.Spec.Tolerations = uniqueTolerations
	}
	// Check if the resource actually updated or if labels match TaintedBrokersSelector
	patchResult, err := patch.DefaultPatchMaker.Calculate(currentPod, desiredPod)
	switch {
	case err != nil:
		log.Error(err, "could not match objects", "kind", desiredType)
	case r.isPodTainted(log, currentPod):
		log.Info("pod has tainted labels, deleting it", "pod", currentPod)
	case patchResult.IsEmpty():
		if !k8sutil.IsPodContainsTerminatedContainer(currentPod) &&
			r.KafkaCluster.Status.BrokersState[currentPod.Labels[v1beta1.BrokerIdLabelKey]].ConfigurationState == v1beta1.ConfigInSync &&
			!k8sutil.IsPodContainsEvictedContainer(currentPod) &&
			!k8sutil.IsPodContainsShutdownContainer(currentPod) {
			log.V(1).Info("resource is in sync")
			return nil
		}
	default:
		log.V(1).Info("kafka pod resource diffs",
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
			matchingLabels := client.MatchingLabels(apiutil.LabelsForKafka(r.KafkaCluster.Name))
			err := r.Client.List(context.TODO(), podList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
			if err != nil {
				return errors.WrapIf(err, "failed to reconcile resource")
			}
			for _, pod := range podList.Items {
				pod := pod
				if k8sutil.IsMarkedForDeletion(pod.ObjectMeta) {
					return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("pod is still terminating"), "rolling upgrade in progress")
				}
				if k8sutil.IsPodContainsPendingContainer(&pod) {
					return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("pod is still creating"), "rolling upgrade in progress")
				}
			}

			errorCount := r.KafkaCluster.Status.RollingUpgrade.ErrorCount

			kClient, close, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
			if err != nil {
				return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
			}
			defer close()
			allOfflineReplicas, err := kClient.AllOfflineReplicas()
			if err != nil {
				return errors.WrapIf(err, "health check failed")
			}
			if len(allOfflineReplicas) > 0 {
				log.Info("offline replicas", "IDs", allOfflineReplicas)
			}
			outOfSyncReplicas, err := kClient.OutOfSyncReplicas()
			if err != nil {
				return errors.WrapIf(err, "health check failed")
			}
			if len(outOfSyncReplicas) > 0 {
				log.Info("out-of-sync replicas", "IDs", outOfSyncReplicas)
			}

			impactedReplicas := make(map[int32]struct{})
			for _, brokerID := range allOfflineReplicas {
				impactedReplicas[brokerID] = struct{}{}
			}
			for _, brokerID := range outOfSyncReplicas {
				impactedReplicas[brokerID] = struct{}{}
			}

			errorCount += len(impactedReplicas)

			if errorCount >= r.KafkaCluster.Spec.RollingUpgradeConfig.FailureThreshold {
				return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("cluster is not healthy"), "rolling upgrade in progress")
			}
		}
	}

	err = r.Client.Delete(context.TODO(), currentPod)
	if err != nil {
		return errorfactory.New(errorfactory.APIFailure{}, err, "deleting resource failed", "kind", desiredType)
	}

	// Print terminated container's statuses
	if k8sutil.IsPodContainsTerminatedContainer(currentPod) {
		for _, containerState := range currentPod.Status.ContainerStatuses {
			if containerState.State.Terminated != nil {
				log.Info("terminated container for broker pod", "pod", currentPod.GetName(), v1beta1.BrokerIdLabelKey, currentPod.Labels[v1beta1.BrokerIdLabelKey],
					"containerName", containerState.Name, "exitCode", containerState.State.Terminated.ExitCode, "reason", containerState.State.Terminated.Reason)
			}
		}
	}
	log.Info("broker pod deleted", "pod", currentPod.GetName(), v1beta1.BrokerIdLabelKey, currentPod.Labels[v1beta1.BrokerIdLabelKey])
	return nil
}

// Checks for match between pod labels and TaintedBrokersSelector
func (r *Reconciler) isPodTainted(log logr.Logger, pod *corev1.Pod) bool {
	selector, err := metav1.LabelSelectorAsSelector(r.KafkaCluster.Spec.TaintedBrokersSelector)

	if err != nil {
		log.Error(err, "Invalid tainted brokers label selector")
		return false
	}

	if selector.Empty() {
		return false
	}
	return selector.Matches(labels.Set(pod.Labels))
}

//nolint:funlen
func (r *Reconciler) reconcileKafkaPvc(ctx context.Context, log logr.Logger, brokersDesiredPvcs map[string][]*corev1.PersistentVolumeClaim) error {
	brokersVolumesState := make(map[string]map[string]v1beta1.VolumeState)
	var brokerIds []string
	waitForDiskRemovalToFinish := false

	for brokerId, desiredPvcs := range brokersDesiredPvcs {
		desiredType := reflect.TypeOf(&corev1.PersistentVolumeClaim{})
		brokerVolumesState := make(map[string]v1beta1.VolumeState)

		pvcList := &corev1.PersistentVolumeClaimList{}

		matchingLabels := client.MatchingLabels(
			apiutil.MergeLabels(
				apiutil.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{v1beta1.BrokerIdLabelKey: brokerId},
			),
		)

		log = log.WithValues("kind", desiredType)

		err := r.Client.List(ctx, pvcList,
			client.InNamespace(r.KafkaCluster.GetNamespace()), matchingLabels)
		if err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
		}

		// Handle disk removal
		if len(pvcList.Items) > len(desiredPvcs) {
			for _, pvc := range pvcList.Items {
				foundInDesired := false
				existingMountPath := pvc.Annotations["mountPath"]

				for _, desiredPvc := range desiredPvcs {
					desiredMountPath := desiredPvc.Annotations["mountPath"]

					if existingMountPath == desiredMountPath {
						foundInDesired = true
						break
					}
				}

				if foundInDesired {
					continue
				}

				mountPathToRemove := existingMountPath
				if brokerState, ok := r.KafkaCluster.Status.BrokersState[brokerId]; ok {
					volumeStateStatus, found := brokerState.GracefulActionState.VolumeStates[mountPathToRemove]
					if !found {
						// If the state is not found, it means that the disk removal was done according to the disk removal succeeded branch
						log.Info("Disk removal was completed, waiting for Rolling Upgrade to remove PVC", "brokerId", brokerId, "mountPath", mountPathToRemove)
						continue
					}

					// Check the volume state
					ccVolumeState := volumeStateStatus.CruiseControlVolumeState
					switch {
					case ccVolumeState.IsDiskRemovalSucceeded():
						if err := r.Client.Delete(ctx, &pvc); err != nil {
							return errorfactory.New(errorfactory.APIFailure{}, err, "deleting resource failed", "kind", desiredType)
						}
						log.Info("resource deleted")
						err = k8sutil.DeleteVolumeStatus(r.Client, brokerId, mountPathToRemove, r.KafkaCluster, log)
						if err != nil {
							return errors.WrapIfWithDetails(err, "could not delete volume status for broker volume", "brokerId", brokerId, "mountPath", mountPathToRemove)
						}
					case ccVolumeState.IsDiskRemoval():
						log.Info("Graceful disk removal is in progress", "brokerId", brokerId, "mountPath", mountPathToRemove)
						waitForDiskRemovalToFinish = true
					case ccVolumeState.IsDiskRebalance():
						log.Info("Graceful disk rebalance is in progress, waiting to mark disk for removal", "brokerId", brokerId, "mountPath", mountPathToRemove)
						waitForDiskRemovalToFinish = true
					default:
						brokerVolumesState[mountPathToRemove] = v1beta1.VolumeState{CruiseControlVolumeState: v1beta1.GracefulDiskRemovalRequired}
						log.Info("Marked the volume for removal", "brokerId", brokerId, "mountPath", mountPathToRemove)
						waitForDiskRemovalToFinish = true
					}
				}
			}
		}

		for _, desiredPvc := range desiredPvcs {
			currentPvc := desiredPvc.DeepCopy()
			log.V(1).Info("searching with label because name is empty")

			err := r.Client.List(ctx, pvcList,
				client.InNamespace(r.KafkaCluster.GetNamespace()), matchingLabels)
			if err != nil {
				return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
			}

			mountPath := currentPvc.Annotations["mountPath"]
			// Creating the first PersistentVolume For Pod
			if len(pvcList.Items) == 0 {
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPvc); err != nil {
					return errors.WrapIf(err, "could not apply last state to annotation")
				}
				if err := r.Client.Create(ctx, desiredPvc); err != nil {
					return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
				}
				log.Info("resource created")
				continue
			}

			alreadyCreated := false
			for _, pvc := range pvcList.Items {
				if mountPath == pvc.Annotations["mountPath"] {
					currentPvc = pvc.DeepCopy()
					alreadyCreated = true
					// Checking pvc state, if bounded, so the broker has already restarted and the CC GracefulDiskRebalance has not happened yet,
					// then we make it happening with status update.
					// If disk removal was set, and the disk was added back, we also need to mark the volume for rebalance
					volumeState, found := r.KafkaCluster.Status.BrokersState[brokerId].GracefulActionState.VolumeStates[mountPath]
					if currentPvc.Status.Phase == corev1.ClaimBound &&
						(!found || volumeState.CruiseControlVolumeState.IsDiskRemoval()) {
						brokerVolumesState[mountPath] = v1beta1.VolumeState{CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired}
					}
					break
				}
			}

			if !alreadyCreated {
				// Creating the 2+ PersistentVolumes for Pod
				if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPvc); err != nil {
					return errors.WrapIf(err, "could not apply last state to annotation")
				}
				if err := r.Client.Create(ctx, desiredPvc); err != nil {
					return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
				}
				continue
			}
			if err == nil {
				if k8sutil.CheckIfObjectUpdated(log, desiredType, currentPvc, desiredPvc) {
					if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPvc); err != nil {
						return errors.WrapIf(err, "could not apply last state to annotation")
					}

					if isDesiredStorageValueInvalid(desiredPvc, currentPvc) {
						return errorfactory.New(errorfactory.InternalError{}, errors.New("could not modify pvc size"),
							"one can not reduce the size of a PVC", "kind", desiredType)
					}

					resReq := desiredPvc.Spec.Resources.Requests
					labels := desiredPvc.Labels
					desiredPvc = currentPvc.DeepCopy()
					desiredPvc.Spec.Resources.Requests = resReq
					desiredPvc.Labels = labels

					if err := r.Client.Update(ctx, desiredPvc); err != nil {
						return errorfactory.New(errorfactory.APIFailure{}, err, "updating resource failed", "kind", desiredType)
					}
					log.Info("resource updated")
				}
			}
		}

		if len(brokerVolumesState) > 0 {
			brokerIds = append(brokerIds, brokerId)
			brokersVolumesState[brokerId] = brokerVolumesState
		}
	}

	if len(brokersVolumesState) > 0 {
		err := k8sutil.UpdateBrokerStatus(r.Client, brokerIds, r.KafkaCluster, brokersVolumesState, log)
		if err != nil {
			return err
		}
	}

	if waitForDiskRemovalToFinish {
		return errorfactory.New(errorfactory.CruiseControlTaskRunning{}, errors.New("Disk removal pending"), "Disk removal pending")
	}

	return nil
}

// GetBrokersWithPendingOrRunningCCTask returns list of brokers that are either waiting for CC
// to start executing a broker task (add broker, remove broker, etc) or CC already running a task for it.
func GetBrokersWithPendingOrRunningCCTask(kafkaCluster *v1beta1.KafkaCluster) []int32 {
	var brokerIDs []int32
	for i := range kafkaCluster.Spec.Brokers {
		if state, ok := kafkaCluster.Status.BrokersState[strconv.Itoa(int(kafkaCluster.Spec.Brokers[i].Id))]; ok {
			if state.GracefulActionState.CruiseControlState.IsRequiredState() ||
				(state.GracefulActionState.CruiseControlOperationReference != nil && state.GracefulActionState.CruiseControlState.IsRunningState()) {
				brokerIDs = append(brokerIDs, kafkaCluster.Spec.Brokers[i].Id)
			} else {
				// Check if the volumes are rebalancing or removing
				for _, volumeState := range state.GracefulActionState.VolumeStates {
					ccVolumeState := volumeState.CruiseControlVolumeState
					if ccVolumeState.IsDiskRemoval() || ccVolumeState.IsDiskRebalance() {
						brokerIDs = append(brokerIDs, kafkaCluster.Spec.Brokers[i].Id)
					}
				}
			}
		}
	}
	return brokerIDs
}

func isDesiredStorageValueInvalid(desired, current *corev1.PersistentVolumeClaim) bool {
	return desired.Spec.Resources.Requests.Storage().Value() < current.Spec.Resources.Requests.Storage().Value()
}

func (r *Reconciler) getBrokerHost(log logr.Logger, defaultHost string, broker v1beta1.Broker, eListener v1beta1.ExternalListenerConfig, iConfig v1beta1.IngressConfig) (string, error) {
	brokerHost := defaultHost
	portNumber := eListener.GetBrokerPort(broker.Id)

	if eListener.GetAccessMethod() != corev1.ServiceTypeLoadBalancer {
		bConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
		if err != nil {
			return "", err
		}
		if eListener.ExternalStartingPort == 0 {
			portNumber, err = r.getK8sAssignedNodeport(log, eListener.Name, broker.Id)
			if err != nil {
				return "", err
			}
		}

		if externalIP, ok := bConfig.NodePortExternalIP[eListener.Name]; ok && externalIP != "" {
			// https://kubernetes.io/docs/concepts/services-networking/service/#external-ips
			// if specific external IP is set for the NodePort service incoming traffic will be received on Service port
			// and not nodeport
			portNumber = eListener.ContainerPort
		}
		if brokerHost == "" {
			if bConfig.NodePortExternalIP[eListener.Name] == "" {
				brokerHost, err = r.getK8sAssignedNodeAddress(broker.Id, string(bConfig.NodePortNodeAddressType))
				if err != nil {
					log.Error(err, fmt.Sprintf("could not get the (%s) address of the broker's (ID: %d) node for external listener (%s) configuration",
						bConfig.NodePortNodeAddressType, broker.Id, eListener.Name))
				}
			} else {
				brokerHost = bConfig.NodePortExternalIP[eListener.Name]
			}
		} else {
			brokerHost = fmt.Sprintf("%s-%d-%s.%s%s", r.KafkaCluster.Name, broker.Id, eListener.Name, r.KafkaCluster.Namespace, brokerHost)
		}
	}
	if eListener.TLSEnabled() {
		brokerHost = iConfig.EnvoyConfig.GetBrokerHostname(broker.Id)
		if brokerHost == "" {
			return "", errors.New("brokerHostnameTemplate is not set in the ingress service settings")
		}
	}
	return fmt.Sprintf("%s:%d", brokerHost, portNumber), nil
}

func (r *Reconciler) createExternalListenerStatuses(log logr.Logger) (map[string]v1beta1.ListenerStatusList, error) {
	extListenerStatuses := make(map[string]v1beta1.ListenerStatusList, len(r.KafkaCluster.Spec.ListenersConfig.ExternalListeners))
	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
		// in case if external listener uses loadbalancer type of service and istioControlPlane is not specified than we skip this listener from status update. In this way this external listener will not be in the configmap.
		if eListener.GetAccessMethod() == corev1.ServiceTypeLoadBalancer && r.KafkaCluster.Spec.GetIngressController() == istioingressutils.IngressControllerName && r.KafkaCluster.Spec.IstioControlPlane == nil {
			continue
		}
		var host string
		var foundLBService *corev1.Service
		var err error
		ingressConfigs, defaultControllerName, err := util.GetIngressConfigs(r.KafkaCluster.Spec, eListener)
		if err != nil {
			return nil, err
		}
		listenerStatusList := make(v1beta1.ListenerStatusList, 0, len(r.KafkaCluster.Spec.Brokers)+1)
		for iConfigName, iConfig := range ingressConfigs {
			if !util.IsIngressConfigInUse(iConfigName, defaultControllerName, r.KafkaCluster, log) {
				continue
			}
			if iConfig.HostnameOverride != "" {
				host = iConfig.HostnameOverride
			} else if eListener.GetAccessMethod() == corev1.ServiceTypeLoadBalancer {
				foundLBService, err = getServiceFromExternalListener(r.Client, r.KafkaCluster, eListener.Name, iConfigName)
				if err != nil {
					return nil, errors.WrapIfWithDetails(err, "could not get service corresponding to the external listener", "externalListenerName", eListener.Name)
				}
				lbIP, err := getLoadBalancerIP(foundLBService)
				if err != nil {
					return nil, errors.WrapIfWithDetails(err, "could not extract IP from LoadBalancer service", "externalListenerName", eListener.Name)
				}
				host = lbIP
			}

			// optionally add all brokers service to the top of the list
			if eListener.GetAccessMethod() != corev1.ServiceTypeNodePort {
				if foundLBService == nil {
					foundLBService, err = getServiceFromExternalListener(r.Client, r.KafkaCluster, eListener.Name, iConfigName)
					if err != nil {
						return nil, errors.WrapIfWithDetails(err, "could not get service corresponding to the external listener", "externalListenerName", eListener.Name)
					}
				}
				var allBrokerPort int32 = 0
				for _, port := range foundLBService.Spec.Ports {
					if port.Name == "tcp-all-broker" {
						allBrokerPort = port.Port
						break
					}
				}
				if allBrokerPort == 0 {
					return nil, errors.NewWithDetails("could not find port with name tcp-all-broker", "externalListenerName", eListener.Name)
				}
				var anyBrokerStatusName string
				if iConfigName == util.IngressConfigGlobalName {
					anyBrokerStatusName = "any-broker"
				} else {
					anyBrokerStatusName = fmt.Sprintf("any-broker-%s", iConfigName)
				}
				listenerStatus := v1beta1.ListenerStatus{
					Name:    anyBrokerStatusName,
					Address: fmt.Sprintf("%s:%d", host, allBrokerPort),
				}
				listenerStatusList = append(listenerStatusList, listenerStatus)
			}

			for _, broker := range r.KafkaCluster.Spec.Brokers {
				brokerHostPort, err := r.getBrokerHost(log, host, broker, eListener, iConfig)
				if err != nil {
					return nil, errors.WrapIfWithDetails(err, "could not get brokerHost for external listener status", "brokerID", broker.Id)
				}

				brokerConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
				if err != nil {
					return nil, err
				}
				if util.ShouldIncludeBroker(brokerConfig, r.KafkaCluster.Status, int(broker.Id), defaultControllerName, iConfigName) {
					listenerStatus := v1beta1.ListenerStatus{
						Name:    fmt.Sprintf("broker-%d", broker.Id),
						Address: brokerHostPort,
					}
					listenerStatusList = append(listenerStatusList, listenerStatus)
				}
			}
		}
		// We have to sort the listener status list since the ingress config is a
		// map and we are using that for the generation
		sort.Sort(listenerStatusList)

		extListenerStatuses[eListener.Name] = listenerStatusList
	}
	return extListenerStatuses, nil
}

func (r *Reconciler) getK8sAssignedNodeport(log logr.Logger, eListenerName string, brokerId int32) (int32, error) {
	log.Info("determining automatically assigned nodeport",
		v1beta1.BrokerIdLabelKey, brokerId, "listenerName", eListenerName)
	nodePortSvc := &corev1.Service{}
	err := r.Client.Get(context.Background(),
		types.NamespacedName{Name: fmt.Sprintf(kafka.NodePortServiceTemplate,
			r.KafkaCluster.GetName(), brokerId, eListenerName),
			Namespace: r.KafkaCluster.GetNamespace()}, nodePortSvc)
	if err != nil {
		return 0, errors.WrapWithDetails(err, "could not get nodeport service",
			v1beta1.BrokerIdLabelKey, brokerId, "listenerName", eListenerName)
	}
	log.V(1).Info("nodeport service successfully found",
		v1beta1.BrokerIdLabelKey, brokerId, "listenerName", eListenerName)

	var nodePort int32
	for _, port := range nodePortSvc.Spec.Ports {
		if port.Name == fmt.Sprintf("broker-%d", brokerId) {
			nodePort = port.NodePort
			break
		}
	}
	if nodePort == 0 {
		return nodePort, errorfactory.New(errorfactory.ResourceNotReady{},
			errors.New("nodeport port number is not allocated"), "nodeport number is still 0")
	}
	return nodePort, nil
}

func (r *Reconciler) getK8sAssignedNodeAddress(brokerId int32, nodeAddressType string) (string, error) {
	podList := &corev1.PodList{}
	if err := r.Client.List(context.TODO(), podList,
		client.InNamespace(r.KafkaCluster.Namespace),
		client.MatchingLabels(apiutil.MergeLabels(apiutil.LabelsForKafka(r.KafkaCluster.Name), map[string]string{v1beta1.BrokerIdLabelKey: strconv.Itoa(int(brokerId))})),
	); err != nil {
		return "", err
	}

	switch {
	case len(podList.Items) == 0:
		return "", fmt.Errorf("could not find pod with matching brokerId: %d in namespace '%s'", brokerId, r.KafkaCluster.Namespace)
	case len(podList.Items) > 1:
		return "", fmt.Errorf("multiple pods found with brokerId: %d in namespace '%s'", brokerId, r.KafkaCluster.Namespace)
	default:
		return r.getK8sNodeIP(podList.Items[0].Spec.NodeName, nodeAddressType)
	}
}

func (r *Reconciler) getK8sNodeIP(nodeName string, nodeAddressType string) (string, error) {
	node := &corev1.Node{}
	clientNamespacedName := types.NamespacedName{Name: nodeName, Namespace: r.KafkaCluster.Namespace}
	if err := r.Client.Get(context.TODO(), clientNamespacedName, node); err != nil {
		return "", err
	}

	addressMap := make(map[string]string)
	for _, address := range node.Status.Addresses {
		addressMap[string(address.Type)] = address.Address
	}

	if val, ok := addressMap[nodeAddressType]; ok && val != "" {
		return val, nil
	}

	return "", fmt.Errorf("could not find the node (%s)'s (%s) address", nodeName, nodeAddressType)
}

// determineControllerId returns the ID of the controller broker of the current cluster
func (r *Reconciler) determineControllerId() (int32, error) {
	kClient, close, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return -1, errors.WrapIf(err, "could not create Kafka client, thus could not determine controller")
	}
	defer close()

	_, controllerID, err := kClient.DescribeCluster()
	if err != nil {
		return -1, errors.WrapIf(err, "could not find controller broker")
	}

	return controllerID, nil
}

func getServiceFromExternalListener(client client.Client, cluster *v1beta1.KafkaCluster,
	eListenerName string, ingressConfigName string) (*corev1.Service, error) {
	foundLBService := &corev1.Service{}
	var iControllerServiceName string
	switch cluster.Spec.GetIngressController() {
	case istioingressutils.IngressControllerName:
		if ingressConfigName == util.IngressConfigGlobalName {
			iControllerServiceName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplate, eListenerName, cluster.GetName())
		} else {
			iControllerServiceName = fmt.Sprintf(istioingressutils.MeshGatewayNameTemplateWithScope, eListenerName, ingressConfigName, cluster.GetName())
		}
	case envoyutils.IngressControllerName:
		if ingressConfigName == util.IngressConfigGlobalName {
			iControllerServiceName = fmt.Sprintf(envoyutils.EnvoyServiceName, eListenerName, cluster.GetName())
		} else {
			iControllerServiceName = fmt.Sprintf(envoyutils.EnvoyServiceNameWithScope, eListenerName, ingressConfigName, cluster.GetName())
		}
	}

	err := client.Get(context.TODO(), types.NamespacedName{Name: iControllerServiceName, Namespace: cluster.GetNamespace()}, foundLBService)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "could not get LoadBalancer service", "serviceName", iControllerServiceName)
	}
	return foundLBService, nil
}

// reorderBrokers returns the KafkaCluster brokers list reordered for reconciliation such that:
//   - the controller broker is reconciled last
//   - prioritize missing broker pods where downscale operation has not been finished yet to give bigger chance to be scheduled and downscale operation to be continued
//   - prioritize upscale in order to allow upscaling the cluster even when there is a stuck RU
//   - prioritize missing broker pods to be able for escaping from offline partitions, not all replicas in sync which
//     could stall RU flow
func reorderBrokers(runningBrokers, boundPersistentVolumeClaims map[string]struct{}, desiredBrokers []v1beta1.Broker, brokersState map[string]v1beta1.BrokerState, controllerBrokerID int32, log logr.Logger) []v1beta1.Broker {
	brokersReconcilePriority := make(map[string]brokerReconcilePriority, len(desiredBrokers))
	missingBrokerDownScaleRunning := make(map[string]struct{})
	// logic for handling that case when a broker pod is removed before downscale operation completed
	for id, brokerState := range brokersState {
		_, running := runningBrokers[id]
		_, pvcPresent := boundPersistentVolumeClaims[id]
		ccState := brokerState.GracefulActionState.CruiseControlState
		if !running && (ccState == v1beta1.GracefulDownscaleRequired || ccState == v1beta1.GracefulDownscaleRunning) {
			log.Info("missing broker found with incomplete downscale operation", v1beta1.BrokerIdLabelKey, id)
			if pvcPresent {
				unfinishedBroker, err := util.GetBrokerFromBrokerConfigurationBackup(brokerState.ConfigurationBackup)
				if err != nil {
					log.Error(err, "unable to restore broker configuration from configuration backup", v1beta1.BrokerIdLabelKey, id)
					continue
				}
				log.Info("re-creating broker pod to continue downscale operation", v1beta1.BrokerIdLabelKey, id)
				missingBrokerDownScaleRunning[id] = struct{}{}
				desiredBrokers = append(desiredBrokers, unfinishedBroker)
			} else {
				log.Info("pvc is missing, unable to reconstruct missing broker pod", v1beta1.BrokerIdLabelKey, id)
			}
		}
	}

	for _, b := range desiredBrokers {
		brokerID := fmt.Sprintf("%d", b.Id)
		brokersReconcilePriority[brokerID] = nonControllerBrokerReconcilePriority

		if _, ok := missingBrokerDownScaleRunning[brokerID]; ok {
			brokersReconcilePriority[brokerID] = missingBrokerDownScaleRunningPriority
		} else if _, ok := brokersState[brokerID]; !ok {
			brokersReconcilePriority[brokerID] = newBrokerReconcilePriority
		} else if _, ok := runningBrokers[brokerID]; !ok {
			brokersReconcilePriority[brokerID] = missingBrokerReconcilePriority
		} else if b.Id == controllerBrokerID {
			brokersReconcilePriority[brokerID] = controllerBrokerReconcilePriority
		}
	}

	reorderedBrokers := make([]v1beta1.Broker, 0, len(desiredBrokers))
	// copy desiredBrokers from KafkaCluster CR for ordering
	reorderedBrokers = append(reorderedBrokers, desiredBrokers...)

	// sort brokers by weight
	sort.SliceStable(reorderedBrokers, func(i, j int) bool {
		brokerID1 := fmt.Sprintf("%d", reorderedBrokers[i].Id)
		brokerID2 := fmt.Sprintf("%d", reorderedBrokers[j].Id)

		return brokersReconcilePriority[brokerID1] < brokersReconcilePriority[brokerID2]
	})

	return reorderedBrokers
}

func generateServicePortForIListeners(listeners []v1beta1.InternalListenerConfig) []corev1.ServicePort {
	var usedPorts []corev1.ServicePort
	for _, iListener := range listeners {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name:       strings.ReplaceAll(iListener.GetListenerServiceName(), "_", ""),
			Port:       iListener.ContainerPort,
			TargetPort: intstr.FromInt(int(iListener.ContainerPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	return usedPorts
}

func generateServicePortForEListeners(listeners []v1beta1.ExternalListenerConfig) []corev1.ServicePort {
	var usedPorts []corev1.ServicePort
	for _, eListener := range listeners {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name:       eListener.GetListenerServiceName(),
			Protocol:   corev1.ProtocolTCP,
			Port:       eListener.ContainerPort,
			TargetPort: intstr.FromInt(int(eListener.ContainerPort)),
		})
	}
	return usedPorts
}

func generateServicePortForAdditionalPorts(containerPorts []corev1.ContainerPort) []corev1.ServicePort {
	var usedPorts []corev1.ServicePort
	for _, containerPort := range containerPorts {
		usedPorts = append(usedPorts, corev1.ServicePort{
			Name:       containerPort.Name,
			Protocol:   containerPort.Protocol,
			Port:       containerPort.ContainerPort,
			TargetPort: intstr.FromInt(int(containerPort.ContainerPort)),
		})
	}
	return usedPorts
}
