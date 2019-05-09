package currentalert

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/k8sutil"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func processAlert(alert *currentAlertStruct, client client.Client) error {

	sConfig := banzaicloudv1alpha1.StorageConfig{
		MountPath: "kafkacska",
		PVCSpec: &corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: util.StringPointer("storagetest"),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("1Gi"),
				},
			},
		},
	}

	err := k8sutil.AddPvToSpecificBroker("2","kafka","kafka", &sConfig,client)
	if err != nil {
		return err
	}
	return nil
}
