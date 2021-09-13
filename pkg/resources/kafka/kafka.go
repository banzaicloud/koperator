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
	"sort"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

const (
	componentName         = "kafka"
	brokerConfigTemplate  = "%s-config"
	brokerStorageTemplate = "%s-%d-storage-%d-"

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
	Scheme              *runtime.Scheme
	kafkaClientProvider kafkaclient.Provider
}

// New creates a new reconciler for Kafka
func New(client client.Client, directClient client.Reader, scheme *runtime.Scheme, cluster *v1beta1.KafkaCluster, kafkaClientProvider kafkaclient.Provider) *Reconciler {
	return &Reconciler{
		Scheme: scheme,
		Reconciler: resources.Reconciler{
			Client:       client,
			DirectClient: directClient,
			KafkaCluster: cluster,
		},
		kafkaClientProvider: kafkaClientProvider,
	}
}
func getCreatedPvcForBroker(c client.Client, brokerID int32, namespace, crName string) ([]corev1.PersistentVolumeClaim, error) {
	foundPvcList := &corev1.PersistentVolumeClaimList{}
	matchingLabels := client.MatchingLabels(
		util.MergeLabels(
			kafka.LabelsForKafka(crName),
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
func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName, "clusterName", r.KafkaCluster.Name, "clusterNamespace", r.KafkaCluster.Namespace)

	log.V(1).Info("Reconciling")

	//TODO remove this in a future release (baluchicken)
	// this is only required to keep backward compatibility
	var brokerIdWithDeprecatedStatus []string
	for bId, bStatus := range r.KafkaCluster.Status.BrokersState {
		if bStatus.GracefulActionState.CruiseControlState == "GracefulUpdateNotRequired" {
			brokerIdWithDeprecatedStatus = append(brokerIdWithDeprecatedStatus, bId)
		}
	}
	statusErr := k8sutil.UpdateBrokerStatus(r.Client, brokerIdWithDeprecatedStatus, r.KafkaCluster,
		v1beta1.GracefulActionState{CruiseControlState: v1beta1.GracefulUpscaleSucceeded}, log)

	if statusErr != nil {
		return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating deprecated status failed")
	}

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
	err := r.reconcileKafkaPodDelete(log)
	if err != nil {
		return errors.WrapIf(err, "failed to reconcile resource")
	}

	extListenerStatuses, err := r.createExternalListenerStatuses(log)
	if err != nil {
		return errors.WrapIf(err, "could not update status for external listeners")
	}
	intListenerStatuses, controllerIntListenerStatuses := k8sutil.CreateInternalListenerStatuses(r.KafkaCluster)
	err = k8sutil.UpdateListenerStatuses(context.Background(), r.Client, r.KafkaCluster, log, intListenerStatuses, extListenerStatuses)
	if err != nil {
		return errors.WrapIf(err, "failed to update listener statuses")
	}

	// Setup the PKI if using SSL
	if r.KafkaCluster.Spec.ListenersConfig.SSLSecrets != nil {
		// reconcile the PKI
		if err := pki.GetPKIManager(r.Client, r.KafkaCluster, v1beta1.PKIBackendProvided, log).ReconcilePKI(context.TODO(), log, r.Scheme, extListenerStatuses); err != nil {
			return err
		}
	}

	// We need to grab names for servers and client in case user is enabling ACLs
	// That way we can continue to manage topics and users
	serverPass, clientPass, superUsers, err := r.getServerAndClientDetails()
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
			o := r.pvc(broker.Id, index, storage, log)
			brokerVolumes = append(brokerVolumes, o.(*corev1.PersistentVolumeClaim))
		}
		if len(brokerVolumes) > 0 {
			brokersVolumes[strconv.Itoa(int(broker.Id))] = brokerVolumes
		}
	}
	if len(brokersVolumes) > 0 {
		err := r.reconcileKafkaPvc(log, brokersVolumes)
		if err != nil {
			return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resources", "PersistentVolumeClaim")
		}
	}

	reorderedBrokers := r.reorderBrokers(log, r.KafkaCluster.Spec.Brokers)
	for _, broker := range reorderedBrokers {
		brokerConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
		if err != nil {
			return errors.WrapIf(err, "failed to reconcile resource")
		}

		var configMap *corev1.ConfigMap
		if r.KafkaCluster.Spec.RackAwareness == nil {
			configMap = r.configMap(broker.Id, brokerConfig, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPass, clientPass, superUsers, log)
			err := k8sutil.Reconcile(log, r.Client, configMap, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", configMap.GetObjectKind().GroupVersionKind())
			}
		} else if brokerState, ok := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(broker.Id))]; ok {
			if brokerState.RackAwarenessState != "" {
				configMap = r.configMap(broker.Id, brokerConfig, extListenerStatuses, intListenerStatuses, controllerIntListenerStatuses, serverPass, clientPass, superUsers, log)
				err := k8sutil.Reconcile(log, r.Client, configMap, r.KafkaCluster)
				if err != nil {
					return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", configMap.GetObjectKind().GroupVersionKind())
				}
			}
		}

		pvcs, err := getCreatedPvcForBroker(r.Client, broker.Id, r.KafkaCluster.Namespace, r.KafkaCluster.Name)
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
		if err = r.reconcilePerBrokerDynamicConfig(broker.Id, brokerConfig, configMap, log); err != nil {
			return err
		}
	}

	if err = r.reconcileClusterWideDynamicConfig(); err != nil {
		return err
	}

	log.V(1).Info("Reconciled")

	return nil
}

func (r *Reconciler) reconcileKafkaPodDelete(log logr.Logger) error {
	podList := &corev1.PodList{}
	matchingLabels := client.MatchingLabels(kafka.LabelsForKafka(r.KafkaCluster.Name))

	err := r.Client.List(context.TODO(), podList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
	if err != nil {
		return errors.WrapIf(err, "failed to reconcile resource")
	}

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

	if len(deletedBrokers) > 0 {
		if !arePodsAlreadyDeleted(deletedBrokers, log) {
			cc := scale.NewCruiseControlScaler(r.KafkaCluster.Namespace, r.KafkaCluster.Spec.GetKubernetesClusterDomain(), r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
			liveBrokers, err := cc.GetLiveKafkaBrokersFromCruiseControl(generateBrokerIdsFromPodSlice(deletedBrokers))

			if err != nil {
				log.Error(err, "could not query CC for ALIVE brokers")
				return errorfactory.New(errorfactory.CruiseControlNotReady{}, err, fmt.Sprintf("broker(s) id(s): %s", strings.Join(liveBrokers, ",")))
			}
			if len(liveBrokers) == 0 {
				log.Info("No alive broker found in CC. No need to decommission")
			} else {
				var brokersPendingGracefulDownscale []string

				for i := range liveBrokers {
					if brokerState, ok := r.KafkaCluster.Status.BrokersState[liveBrokers[i]]; ok {
						ccState := brokerState.GracefulActionState.CruiseControlState
						if ccState != v1beta1.GracefulDownscaleRunning && (ccState == v1beta1.GracefulUpscaleSucceeded ||
							ccState == v1beta1.GracefulUpscaleRequired) {
							brokersPendingGracefulDownscale = append(brokersPendingGracefulDownscale, liveBrokers[i])
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

		for _, broker := range deletedBrokers {
			broker := broker
			if broker.ObjectMeta.DeletionTimestamp != nil {
				log.Info(fmt.Sprintf("Broker %s is already on terminating state", broker.Labels["brokerId"]))
				continue
			}

			if brokerState, ok := r.KafkaCluster.Status.BrokersState[broker.Labels["brokerId"]]; ok &&
				brokerState.GracefulActionState.CruiseControlState != v1beta1.GracefulDownscaleSucceeded &&
				brokerState.GracefulActionState.CruiseControlState != v1beta1.GracefulUpscaleRequired &&
				broker.Status.Phase != corev1.PodPending {
				if brokerState.GracefulActionState.CruiseControlState == v1beta1.GracefulDownscaleRunning {
					log.Info("cc task is still running for broker", "brokerId", broker.Labels["brokerId"], "taskId", brokerState.GracefulActionState.CruiseControlTaskId)
				}
				continue
			}

			err = r.Client.Delete(context.TODO(), &broker)
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete broker", "id", broker.Labels["brokerId"])
			}
			log.Info("broker pod deleted", "brokerId", broker.Labels["brokerId"], "pod", broker.GetName())
			configMapName := fmt.Sprintf(brokerConfigTemplate+"-%s", r.KafkaCluster.Name, broker.Labels["brokerId"])
			err = r.Client.Delete(context.TODO(), &corev1.ConfigMap{ObjectMeta: templates.ObjectMeta(configMapName, kafka.LabelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// can happen when broker was not fully initialized and now is deleted
					log.Info(fmt.Sprintf("ConfigMap for Broker %s not found. Continue", broker.Labels["brokerId"]))
				} else {
					return errors.WrapIfWithDetails(err, "could not delete configmap for broker", "id", broker.Labels["brokerId"])
				}
			} else {
				log.V(1).Info("configMap for broker deleted", "configMap name", configMapName, "brokerId", broker.Labels["brokerId"])
			}
			if !r.KafkaCluster.Spec.HeadlessServiceEnabled {
				serviceName := fmt.Sprintf("%s-%s", r.KafkaCluster.Name, broker.Labels["brokerId"])
				err = r.Client.Delete(context.TODO(), &corev1.Service{ObjectMeta: templates.ObjectMeta(serviceName, kafka.LabelsForKafka(r.KafkaCluster.Name), r.KafkaCluster)})
				if err != nil {
					if apierrors.IsNotFound(err) {
						// can happen when broker was not fully initialized and now is deleted
						log.Info(fmt.Sprintf("Service for Broker %s not found. Continue", broker.Labels["brokerId"]))
					}
					return errors.WrapIfWithDetails(err, "could not delete service for broker", "id", broker.Labels["brokerId"])
				}
				log.V(1).Info("service for broker deleted", "service name", serviceName, "brokerId", broker.Labels["brokerId"])
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
							log.Info(fmt.Sprintf("PVC for Broker %s not found. Continue", broker.Labels["brokerId"]))
						}
						return errors.WrapIfWithDetails(err, "could not delete pvc for broker", "id", broker.Labels["brokerId"])
					}
					log.V(1).Info("pvc for broker deleted", "pvc name", volume.PersistentVolumeClaim.ClaimName, "brokerId", broker.Labels["brokerId"])
				}
			}
			err = k8sutil.DeleteStatus(r.Client, broker.Labels["brokerId"], r.KafkaCluster, log)
			if err != nil {
				return errors.WrapIfWithDetails(err, "could not delete status for broker", "id", broker.Labels["brokerId"])
			}
		}
	}
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

func (r *Reconciler) reconcileKafkaPod(log logr.Logger, desiredPod *corev1.Pod, bConfig *v1beta1.BrokerConfig) error {
	currentPod := desiredPod.DeepCopy()
	desiredType := reflect.TypeOf(desiredPod)

	log = log.WithValues("kind", desiredType)
	log.V(1).Info("searching with label because name is empty", "brokerId", desiredPod.Labels["brokerId"])

	podList := &corev1.PodList{}

	matchingLabels := client.MatchingLabels(
		util.MergeLabels(
			kafka.LabelsForKafka(r.KafkaCluster.Name),
			map[string]string{"brokerId": desiredPod.Labels["brokerId"]},
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
		statusErr := k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels["brokerId"]},
			r.KafkaCluster, externalConfigNames, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating status for resource failed", "kind", desiredType)
		}
		// Update status to Config InSync because broker is configured to go
		statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels["brokerId"]}, r.KafkaCluster, v1beta1.ConfigInSync, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating status for resource failed", "kind", desiredType)
		}
		// Update status to per-broker Config InSync because broker is configured to go
		log.V(1).Info("setting per broker config status to in sync")
		statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels["brokerId"]}, r.KafkaCluster, v1beta1.PerBrokerConfigInSync, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "updating per broker config status for resource failed", "kind", desiredType)
		}

		if val, ok := r.KafkaCluster.Status.BrokersState[desiredPod.Labels["brokerId"]]; ok && val.GracefulActionState.CruiseControlState != v1beta1.GracefulUpscaleSucceeded {
			gracefulActionState := v1beta1.GracefulActionState{ErrorMessage: "CruiseControl not yet ready", CruiseControlState: v1beta1.GracefulUpscaleSucceeded}

			if r.KafkaCluster.Status.CruiseControlTopicStatus == v1beta1.CruiseControlTopicReady {
				gracefulActionState = v1beta1.GracefulActionState{ErrorMessage: "", CruiseControlState: v1beta1.GracefulUpscaleRequired}
			}
			statusErr = k8sutil.UpdateBrokerStatus(r.Client, []string{desiredPod.Labels["brokerId"]}, r.KafkaCluster, gracefulActionState, log)
			if statusErr != nil {
				return errorfactory.New(errorfactory.StatusUpdateError{}, statusErr, "could not update broker graceful action state")
			}
		}
		log.Info("resource created")
		return nil
	case len(podList.Items) == 1:
		currentPod = podList.Items[0].DeepCopy()
		brokerId := currentPod.Labels["brokerId"]
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
	//Since toleration does not support patchStrategy:"merge,retainKeys",
	//we need to add all toleration from the current pod if the toleration is set in the CR
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
	// Check if the resource actually updated
	patchResult, err := patch.DefaultPatchMaker.Calculate(currentPod, desiredPod)
	switch {
	case err != nil:
		log.Error(err, "could not match objects", "kind", desiredType)
	case patchResult.IsEmpty():
		if !k8sutil.IsPodContainsTerminatedContainer(currentPod) &&
			r.KafkaCluster.Status.BrokersState[currentPod.Labels["brokerId"]].ConfigurationState == v1beta1.ConfigInSync &&
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
			matchingLabels := client.MatchingLabels(kafka.LabelsForKafka(r.KafkaCluster.Name))
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

	// Print terminated container's statuses
	if k8sutil.IsPodContainsTerminatedContainer(currentPod) {
		for _, containerState := range currentPod.Status.ContainerStatuses {
			if containerState.State.Terminated != nil {
				log.Info("terminated container for broker pod", "pod", currentPod.GetName(), "brokerId", currentPod.Labels["brokerId"],
					"containerName", containerState.Name, "exitCode", containerState.State.Terminated.ExitCode, "reason", containerState.State.Terminated.Reason)
			}
		}
	}
	log.Info("broker pod deleted", "pod", currentPod.GetName(), "brokerId", currentPod.Labels["brokerId"])
	return nil
}

func (r *Reconciler) reconcileKafkaPvc(log logr.Logger, brokersDesiredPvcs map[string][]*corev1.PersistentVolumeClaim) error {
	brokersVolumesState := make(map[string]map[string]v1beta1.VolumeState)
	var brokerIds []string

	for brokerId, desiredPvcs := range brokersDesiredPvcs {
		brokerVolumesState := make(map[string]v1beta1.VolumeState)

		pvcList := &corev1.PersistentVolumeClaimList{}

		matchingLabels := client.MatchingLabels(
			util.MergeLabels(
				kafka.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{"brokerId": brokerId},
			),
		)

		for _, desiredPvc := range desiredPvcs {
			currentPvc := desiredPvc.DeepCopy()
			desiredType := reflect.TypeOf(desiredPvc)
			log = log.WithValues("kind", desiredType)
			log.V(1).Info("searching with label because name is empty")

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
				continue
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
				brokerVolumesState[desiredPvc.Annotations["mountPath"]] = v1beta1.VolumeState{CruiseControlVolumeState: v1beta1.GracefulDiskRebalanceRequired}
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

					if err := r.Client.Update(context.TODO(), desiredPvc); err != nil {
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

	return nil
}

// GetBrokersWithPendingOrRunningCCTask returns list of brokers that are either waiting for CC
// to start executing a broker task (add broker, remove broker, etc) or CC already running a task for it.
func GetBrokersWithPendingOrRunningCCTask(kafkaCluster *v1beta1.KafkaCluster) []int32 {
	var brokerIDs []int32
	for i := range kafkaCluster.Spec.Brokers {
		if state, ok := kafkaCluster.Status.BrokersState[strconv.Itoa(int(kafkaCluster.Spec.Brokers[i].Id))]; ok {
			if state.GracefulActionState.CruiseControlState.IsRequiredState() ||
				(state.GracefulActionState.CruiseControlTaskId != "" && state.GracefulActionState.CruiseControlState.IsRunningState()) {
				brokerIDs = append(brokerIDs, kafkaCluster.Spec.Brokers[i].Id)
			} else {
				// Check if the volumes are rebalancing
				for _, volumeState := range state.GracefulActionState.VolumeStates {
					if volumeState.CruiseControlVolumeState == v1beta1.GracefulDiskRebalanceRequired ||
						(volumeState.CruiseControlTaskId != "" && volumeState.CruiseControlVolumeState == v1beta1.GracefulDiskRebalanceRunning) {
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

func (r *Reconciler) createExternalListenerStatuses(log logr.Logger) (map[string]v1beta1.ListenerStatusList, error) {
	extListenerStatuses := make(map[string]v1beta1.ListenerStatusList, len(r.KafkaCluster.Spec.ListenersConfig.ExternalListeners))
	for _, eListener := range r.KafkaCluster.Spec.ListenersConfig.ExternalListeners {
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
				brokerHost := host
				portNumber := eListener.ExternalStartingPort + broker.Id

				if eListener.GetAccessMethod() != corev1.ServiceTypeLoadBalancer {
					bConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
					if err != nil {
						return nil, err
					}
					if eListener.ExternalStartingPort == 0 {
						portNumber, err = r.getK8sAssignedNodeport(log, eListener.Name, broker.Id)
						if err != nil {
							return nil, err
						}
					}

					if externalIP, ok := bConfig.NodePortExternalIP[eListener.Name]; ok && externalIP != "" {
						// https://kubernetes.io/docs/concepts/services-networking/service/#external-ips
						// if specific external IP is set for the NodePort service incoming traffic will be received on Service port
						// and not nodeport
						portNumber = eListener.ContainerPort
					}
					if brokerHost == "" {
						brokerHost = bConfig.NodePortExternalIP[eListener.Name]
					} else {
						brokerHost = fmt.Sprintf("%s-%d-%s.%s%s", r.KafkaCluster.Name, broker.Id, eListener.Name, r.KafkaCluster.Namespace, brokerHost)
					}
				}
				brokerConfig, err := broker.GetBrokerConfig(r.KafkaCluster.Spec)
				if err != nil {
					return nil, err
				}
				if util.ShouldIncludeBroker(brokerConfig, r.KafkaCluster.Status, int(broker.Id), defaultControllerName, iConfigName) {
					listenerStatus := v1beta1.ListenerStatus{
						Name:    fmt.Sprintf("broker-%d", broker.Id),
						Address: fmt.Sprintf("%s:%d", brokerHost, portNumber),
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
		"brokerId", brokerId, "listenerName", eListenerName)
	nodePortSvc := &corev1.Service{}
	err := r.Client.Get(context.Background(),
		types.NamespacedName{Name: fmt.Sprintf(kafka.NodePortServiceTemplate,
			r.KafkaCluster.GetName(), brokerId, eListenerName),
			Namespace: r.KafkaCluster.GetNamespace()}, nodePortSvc)
	if err != nil {
		return 0, errors.WrapWithDetails(err, "could not get nodeport service",
			"brokerId", brokerId, "listenerName", eListenerName)
	}
	log.V(1).Info("nodeport service successfully found",
		"brokerId", brokerId, "listenerName", eListenerName)

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

func (r *Reconciler) reorderBrokers(log logr.Logger, brokers []v1beta1.Broker) []v1beta1.Broker {
	kClient, close, err := r.kafkaClientProvider.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		log.Info("could not create Kafka client, thus could not determine controller")
		return brokers
	}
	defer close()

	_, controllerID, err := kClient.DescribeCluster()
	if err != nil {
		log.Info("could not find controller broker")
		return brokers
	}

	var controllerBroker *v1beta1.Broker
	reorderedBrokers := make([]v1beta1.Broker, 0, len(brokers))
	for i := range brokers {
		if brokers[i].Id == controllerID {
			controllerBroker = &brokers[i]
		} else {
			reorderedBrokers = append(reorderedBrokers, brokers[i])
		}
	}
	if controllerBroker != nil {
		reorderedBrokers = append(reorderedBrokers, *controllerBroker)
	}
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
