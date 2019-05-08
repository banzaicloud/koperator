package k8sutil

import (
	"context"
	"strconv"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/goph/emperror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func updateCrWithNodeAffinity(current *corev1.Pod, cr *banzaicloudv1alpha1.KafkaCluster, client runtimeClient.Client) error {
	nodeZoneAndRegion, err := determineNodeZoneAndRegion(current.Spec.NodeName, client)
	if err != nil {
		return emperror.WrapWith(err, "determining Node zone failed")
	}

	brokerConfigs := []banzaicloudv1alpha1.BrokerConfig{}

	for _, brokerConfig := range cr.Spec.BrokerConfigs{
		if strconv.Itoa(int(brokerConfig.Id)) == current.Labels["brokerId"] {
			nodeAffinity := &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key: zoneLabel,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{nodeZoneAndRegion.Zone},
								},
								{
									Key: regionLabel,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{nodeZoneAndRegion.Region},
								},
							},
						},
					},
				},
			}
			brokerConfig.NodeAffinity = nodeAffinity
		}
		brokerConfigs = append(brokerConfigs, brokerConfig)
	}
	cr.Spec.BrokerConfigs = brokerConfigs
	return updateCr(cr, client)
}

func AddNewBrokerToCr(brokerConfig *banzaicloudv1alpha1.BrokerConfig, crName string ,client runtimeClient.Client) error {
	cr, err := getCr(crName, client)
	if err != nil {
		return err
	}
	cr.Spec.BrokerConfigs = append(cr.Spec.BrokerConfigs, *brokerConfig)

	return updateCr(cr, client)
}

func RemoveBrokerFromCr(brokerId, crName string, client runtimeClient.Client) error {

	cr, err := getCr(crName, client)
	if err != nil {
		return err
	}

	tmpBrokers := cr.Spec.BrokerConfigs[:0]
	for _,broker := range cr.Spec.BrokerConfigs {
		if strconv.Itoa(int(broker.Id)) != brokerId {
			tmpBrokers = append(tmpBrokers, broker)
		}
	}
	cr.Spec.BrokerConfigs = tmpBrokers
	return updateCr(cr, client)
}

//func AddPvToSpecificBroker(brokerId string, crName string, pvc *corev1.PersistentVolumeClaimSpec) error {
//
//}

func getCr(name string, client runtimeClient.Client) (*banzaicloudv1alpha1.KafkaCluster, error) {
	cr := &banzaicloudv1alpha1.KafkaCluster{}

	err := client.Get(context.TODO(),types.NamespacedName{Name: name, Namespace: ""}, cr)
	if err != nil {
		return nil, emperror.WrapWith(err, "could not get cr from k8s", "crName", name)
	}
	return cr, nil
}

func updateCr(cr *banzaicloudv1alpha1.KafkaCluster, client runtimeClient.Client) error {
	err := client.Update(context.TODO(), cr)
	if err != nil {
		return err
	}
	return nil
}
