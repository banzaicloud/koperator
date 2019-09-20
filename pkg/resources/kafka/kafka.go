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
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/certutil"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/kafkautil"
	"github.com/banzaicloud/kafka-operator/pkg/resources"
	"github.com/banzaicloud/kafka-operator/pkg/resources/envoy"
	"github.com/banzaicloud/kafka-operator/pkg/resources/templates"
	"github.com/banzaicloud/kafka-operator/pkg/scale"
	"github.com/banzaicloud/kafka-operator/pkg/util"
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
	log = log.WithValues("component", componentName)

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
	matchingLabels := client.MatchingLabels{
		"kafka_cr": r.KafkaCluster.Name,
	}

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
		for _, broker := range deletedBrokers {
			err = scale.DownsizeCluster(broker.Labels["brokerId"], broker.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
			if err != nil {
				log.Error(err, "graceful downscale failed.")
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
	lBIp := ""

	if r.KafkaCluster.Spec.ListenersConfig.ExternalListeners != nil {
		lBIp, err = getLoadBalancerIP(r.Client, r.KafkaCluster.Namespace, log)
		if err != nil {
			return err
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
			return errors.WrapIfWithDetails(err, "failed to get controller secret, maybe still creating...")
		}
		for _, key := range []string{"clientCert", "peerCert"} {
			su, err := certutil.DecodeCertificate(controllerSecret.Data[key])
			if err != nil {
				return errors.WrapIfWithDetails(err, "Failed to decode our client certificate")
			}
			superUsers = append(superUsers, su.Subject.String())
		}
	}

	for _, broker := range r.KafkaCluster.Spec.Brokers {
		brokerConfig := util.GetBrokerConfig(broker, r.KafkaCluster.Spec)
		for _, storage := range brokerConfig.StorageConfigs {
			o := r.pvc(broker.Id, storage, log)
			err := r.reconcileKafkaPVC(log, o.(*corev1.PersistentVolumeClaim))
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}

		}
		if r.KafkaCluster.Spec.RackAwareness == nil {
			o := r.configMap(broker.Id, brokerConfig, lBIp, superUsers, log)
			err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
			if err != nil {
				return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
			}
		} else {
			if brokerState, ok := r.KafkaCluster.Status.BrokersState[strconv.Itoa(int(broker.Id))]; ok {
				if brokerState.RackAwarenessState == banzaicloudv1alpha1.Configured {
					o := r.configMap(broker.Id, brokerConfig, lBIp, superUsers, log)
					err := k8sutil.Reconcile(log, r.Client, o, r.KafkaCluster)
					if err != nil {
						return errors.WrapIfWithDetails(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
					}
				}
			}
		}

		pvcs, err := getCreatedPVCForBroker(r.Client, broker.Id, r.KafkaCluster.Namespace, r.KafkaCluster.Name)
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
			//TODO handle error properly (baluchicken)
			return err
		}
	}

	if err = r.reconcileClusterWideDynamicConfig(log); err != nil {
		//TODO handle error properly (baluchicken)
		return err
	}

	log.V(1).Info("Reconciled")

	return nil
}

func (r *Reconciler) reconcilePerBrokerDynamicConfig(brokerId int32, brokerConfig *banzaicloudv1alpha1.BrokerConfig, log logr.Logger) error {
	kClient, err := kafkautil.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer kClient.Close()

	parsedBrokerConfig := util.ParsePropertiesFormat(brokerConfig.Config)

	// Calling DescribePerBrokerConfig with empty slice will return all config for that broker including the default ones
	if parsedBrokerConfig != nil {

		brokerConfigKeys := []string{}
		for key := range parsedBrokerConfig {
			brokerConfigKeys = append(brokerConfigKeys, key)
		}
		configIdentical := true

		response, err := kClient.DescribePerBrokerConfig(brokerId, brokerConfigKeys)
		if err != nil {
			//TODO handle error properly (baluchicken)
			return err
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
				//TODO handle error properly (baluchicken)
				return err
			}
		}
	}
	return nil
}

func (r *Reconciler) reconcileClusterWideDynamicConfig(log logr.Logger) error {
	kClient, err := kafkautil.NewFromCluster(r.Client, r.KafkaCluster)
	if err != nil {
		return errorfactory.New(errorfactory.BrokersUnreachable{}, err, "could not connect to kafka brokers")
	}
	defer kClient.Close()

	currentConfig, err := kClient.DescribeClusterWideConfig()
	if err != nil {
		//TODO handle error properly (baluchicken)
		return err
	}
	parsedClusterWideConfig := util.ParsePropertiesFormat(r.KafkaCluster.Spec.ClusterWideConfig)

	if len(currentConfig) != len(parsedClusterWideConfig) {
		err = kClient.AlterClusterWideConfig(util.ConvertMapStringToMapStringPointer(parsedClusterWideConfig))
		if err != nil {
			//TODO handle error properly (baluchicken)
			return err
		}
	}

	return nil
}

func (r *Reconciler) reconcileKafkaPod(log logr.Logger, desiredPod *corev1.Pod) error {
	currentPod := desiredPod.DeepCopy()
	desiredType := reflect.TypeOf(desiredPod)

	log = log.WithValues("kind", desiredType)
	log.V(1).Info("searching with label because name is empty")

	podList := &corev1.PodList{}
	matchingLabels := client.MatchingLabels{
		"kafka_cr": r.KafkaCluster.Name,
		"brokerId": desiredPod.Labels["brokerId"],
	}
	err := r.Client.List(context.TODO(), podList, client.InNamespace(currentPod.Namespace), matchingLabels)
	if err != nil && len(podList.Items) == 0 {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
	}
	if len(podList.Items) == 0 {
		patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPod)
		if err := r.Client.Create(context.TODO(), desiredPod); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		// Update status to Config InSync because broker is configured to go
		statusErr := k8sutil.UpdateBrokerStatus(r.Client, desiredPod.Labels["brokerId"], r.KafkaCluster, banzaicloudv1alpha1.ConfigInSync, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating status for resource failed", "kind", desiredType)
		}
		gracefulActionState := banzaicloudv1alpha1.GracefulActionState{ErrorMessage: "", CruiseControlState: banzaicloudv1alpha1.GracefulUpdateNotRequired}

		if r.KafkaCluster.Status.CruiseControlTopicStatus == banzaicloudv1alpha1.CruiseControlTopicReady {
			gracefulActionState = banzaicloudv1alpha1.GracefulActionState{ErrorMessage: "", CruiseControlState: banzaicloudv1alpha1.GracefulUpdateRequired}
		}
		statusErr = k8sutil.UpdateBrokerStatus(r.Client, desiredPod.Labels["brokerId"], r.KafkaCluster, gracefulActionState, log)
		if statusErr != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, err, "could not update broker graceful action state")
		}
		if r.KafkaCluster.Spec.RackAwareness != nil {
			statusErr := k8sutil.UpdateBrokerStatus(r.Client, desiredPod.Labels["brokerId"], r.KafkaCluster, banzaicloudv1alpha1.WaitingForRackAwareness, log)
			if statusErr != nil {
				return errorfactory.New(errorfactory.StatusUpdateError{}, err, "could not update broker rack state")
			}
		}

		log.Info("resource created")
		return nil
	} else if len(podList.Items) == 1 {
		currentPod = podList.Items[0].DeepCopy()
		brokerId := currentPod.Labels["brokerId"]
		if brokerState, ok := r.KafkaCluster.Status.BrokersState[brokerId]; ok {
			if r.KafkaCluster.Spec.RackAwareness != nil && (brokerState.RackAwarenessState == banzaicloudv1alpha1.WaitingForRackAwareness || brokerState.RackAwarenessState == "") {
				err := k8sutil.UpdateCrWithRackAwarenessConfig(currentPod, r.KafkaCluster, r.Client)
				if err != nil {
					return err
				}
				statusErr := k8sutil.UpdateBrokerStatus(r.Client, brokerId, r.KafkaCluster, banzaicloudv1alpha1.Configured, log)
				if statusErr != nil {
					return errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating status for resource failed", "kind", desiredType)
				}
			}
			if currentPod.Status.Phase == corev1.PodRunning && brokerState.GracefulActionState.CruiseControlState == banzaicloudv1alpha1.GracefulUpdateRequired {
				scaleErr := scale.UpScaleCluster(desiredPod.Labels["brokerId"], desiredPod.Namespace, r.KafkaCluster.Spec.CruiseControlConfig.CruiseControlEndpoint, r.KafkaCluster.Name)
				if scaleErr != nil {
					log.Error(err, "graceful upscale failed, or cluster just started")
					statusErr := k8sutil.UpdateBrokerStatus(r.Client, brokerId, r.KafkaCluster,
						banzaicloudv1alpha1.GracefulActionState{ErrorMessage: scaleErr.Error(), CruiseControlState: banzaicloudv1alpha1.GracefulUpdateFailed}, log)
					if statusErr != nil {
						return errorfactory.New(errorfactory.StatusUpdateError{}, err, "could not update broker graceful action state")
					}
				} else {
					statusErr := k8sutil.UpdateBrokerStatus(r.Client, brokerId, r.KafkaCluster,
						banzaicloudv1alpha1.GracefulActionState{ErrorMessage: "", CruiseControlState: banzaicloudv1alpha1.GracefulUpdateSucceeded}, log)
					if statusErr != nil {
						return errorfactory.New(errorfactory.StatusUpdateError{}, err, "could not update broker graceful action state")
					}
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
		// Check if the resource actually updated
		patchResult, err := patch.DefaultPatchMaker.Calculate(currentPod, desiredPod)
		if err != nil {
			log.Error(err, "could not match objects", "kind", desiredType)
		} else if patchResult.IsEmpty() {
			if currentPod.Status.Phase != corev1.PodFailed && r.KafkaCluster.Status.BrokersState[currentPod.Labels["brokerId"]].ConfigurationState == banzaicloudv1alpha1.ConfigInSync {
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

		patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPod)

		if currentPod.Status.Phase != corev1.PodFailed {

			if r.KafkaCluster.Status.State != banzaicloudv1alpha1.KafkaClusterRollingUpgrading {
				if err := k8sutil.UpdateCRStatus(r.Client, r.KafkaCluster, banzaicloudv1alpha1.KafkaClusterRollingUpgrading, log); err != nil {
					return errorfactory.New(errorfactory.StatusUpdateError{}, err, "setting state to rolling upgrade failed")
				}
			}

			if r.KafkaCluster.Status.State == banzaicloudv1alpha1.KafkaClusterRollingUpgrading {
				// Check if any kafka pod is in terminating state
				podList := &corev1.PodList{}
				matchingLabels := client.MatchingLabels{
					"kafka_cr": r.KafkaCluster.Name,
					"app":      "kafka",
				}
				err := r.Client.List(context.TODO(), podList, client.ListOption(client.InNamespace(r.KafkaCluster.Namespace)), client.ListOption(matchingLabels))
				if err != nil {
					return errors.WrapIf(err, "failed to reconcile resource")
				}
				for _, pod := range podList.Items {
					if k8sutil.IsMarkedForDeletion(pod.ObjectMeta) {
						return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("pod is still terminating"), "rolling upgrade in progress")
					}
				}
				errorCount := r.KafkaCluster.Status.RollingUpgrade.ErrorCount

				kClient, err := kafkautil.NewFromCluster(r.Client, r.KafkaCluster)
				if err != nil {
					return err
				}
				defer kClient.Close()
				offlineReplicaCount, err := kClient.OfflineReplicaCount()
				if err != nil {
					//TODO fix error handling (baluchicken)
					return err
				}
				replicasInSync, err := kClient.AllReplicaInSync()
				if err != nil {
					//TODO fix error handling (baluchicken)
					return err
				}

				if offlineReplicaCount > 0 && !replicasInSync {
					errorCount++
				}
				if errorCount >= r.KafkaCluster.Spec.RollingUpgradeConfig.FailureThreshold {
					return errorfactory.New(errorfactory.ReconcileRollingUpgrade{}, errors.New("cluster is not healthy"), "rolling upgrade in progress")
				}
			}
		}

		err = k8sutil.UpdateCrWithNodeAffinity(currentPod, r.KafkaCluster, r.Client)
		if err != nil {
			return errorfactory.New(errorfactory.StatusUpdateError{}, err, "updating cr with node affinity failed")
		}
		err = r.Client.Delete(context.TODO(), currentPod)
		if err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "deleting resource failed", "kind", desiredType)
		}
	}
	return nil

}

func (r *Reconciler) reconcileKafkaPVC(log logr.Logger, desiredPVC *corev1.PersistentVolumeClaim) error {
	var currentPVC = desiredPVC.DeepCopy()
	desiredType := reflect.TypeOf(desiredPVC)
	log = log.WithValues("kind", desiredType)
	log.V(1).Info("searching with label because name is empty")

	pvcList := &corev1.PersistentVolumeClaimList{}
	matchingLabels := client.MatchingLabels{
		"kafka_cr": r.KafkaCluster.Name,
		"brokerId": desiredPVC.Labels["brokerId"],
	}
	err := r.Client.List(context.TODO(), pvcList,
		client.InNamespace(currentPVC.Namespace), matchingLabels)
	if err != nil && len(pvcList.Items) == 0 {
		return errorfactory.New(errorfactory.APIFailure{}, err, "getting resource failed", "kind", desiredType)
	}
	mountPath := currentPVC.Annotations["mountPath"]

	// Creating the first PersistentVolume For Pod
	if len(pvcList.Items) == 0 {
		patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPVC)
		if err := r.Client.Create(context.TODO(), desiredPVC); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		log.Info("resource created")
		return nil
	}
	alreadyCreated := false
	for _, pvc := range pvcList.Items {
		if mountPath == pvc.Annotations["mountPath"] {
			currentPVC = pvc.DeepCopy()
			alreadyCreated = true
			break
		}
	}
	if !alreadyCreated {
		// Creating the 2+ PersistentVolumes for Pod
		patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPVC)
		if err := r.Client.Create(context.TODO(), desiredPVC); err != nil {
			return errorfactory.New(errorfactory.APIFailure{}, err, "creating resource failed", "kind", desiredType)
		}
		return nil
	}
	if err == nil {
		if k8sutil.CheckIfObjectUpdated(log, desiredType, currentPVC, desiredPVC) {

			patch.DefaultAnnotator.SetLastAppliedAnnotation(desiredPVC)
			desiredPVC = currentPVC

			if err := r.Client.Update(context.TODO(), desiredPVC); err != nil {
				return errorfactory.New(errorfactory.APIFailure{}, err, "updating resource failed", "kind", desiredType)
			}
			log.Info("resource updated")
		}
	}
	return nil
}
