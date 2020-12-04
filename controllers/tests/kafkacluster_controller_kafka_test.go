// Copyright © 2020 Banzai Cloud
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

package tests

import (
	"context"
	"fmt"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"
)

func expectKafka(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	expectKafkaAllBrokerService(kafkaCluster, namespace)
	expectKafkaPDB(kafkaCluster, namespace)
	expectKafkaPVC(kafkaCluster, namespace)

	// per broker:
	// TODO expect configmaps
	// TODO expect service
	// TODO expect pod

	// TODO test reconcile PKI?

	// TODO test reconcileKafkaPodDelete
}

func expectKafkaAllBrokerService(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	service := &corev1.Service{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-all-broker", kafkaCluster.Name),
		}, service)
	}).Should(Succeed())

	Expect(service.Labels).To(HaveKeyWithValue("app", "kafka"))
	Expect(service.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))

	Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
	Expect(service.Spec.SessionAffinity).To(Equal(corev1.ServiceAffinityNone))
	Expect(service.Spec.Selector).To(HaveKeyWithValue("app", "kafka"))
	Expect(service.Spec.Selector).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
	Expect(service.Spec.Ports).To(ConsistOf(
		corev1.ServicePort{
			Name: "tcp-internal",
			Protocol: "TCP",
			Port: 29092,
			TargetPort: intstr.FromInt(29092),
		},
		corev1.ServicePort{
			Name: "tcp-controller",
			Protocol: "TCP",
			Port: 29093,
			TargetPort: intstr.FromInt(29093),
		},
		corev1.ServicePort{
			Name: "tcp-test",
			Protocol: "TCP",
			Port: 9733,
			TargetPort: intstr.FromInt(9733),
		}))
}

func expectKafkaPDB(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	// get current CR
	err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: kafkaCluster.Name, Namespace: namespace}, kafkaCluster)

	// set PDB and reset status
	kafkaCluster.Spec.DisruptionBudget = v1beta1.DisruptionBudget{
		Create: true,
		Budget: "20%",
	}
	kafkaCluster.Status = v1beta1.KafkaClusterStatus{}

	// update CR
	err = k8sClient.Update(context.TODO(), kafkaCluster)
	Expect(err).NotTo(HaveOccurred())

	// wait until reconcile finishes
	waitClusterRunningState(kafkaCluster, namespace)

	// get created PDB
	pdb := policyv1beta1.PodDisruptionBudget{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-pdb", kafkaCluster.Name),
		}, &pdb)
	}).Should(Succeed())

	// make assertions
	Expect(pdb.Labels).To(HaveKeyWithValue("app", "kafka"))
	Expect(pdb.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
	Expect(pdb.Spec.MinAvailable).To(Equal(util.IntstrPointer(1)))
	Expect(pdb.Spec.Selector).NotTo(BeNil())
	Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", "kafka"))
	Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
}

func expectKafkaPVC(kafkaCluster *v1beta1.KafkaCluster, namespace string) {
	// get PVCs
	pvcs := corev1.PersistentVolumeClaimList{}
	Eventually(func() error {
		return k8sClient.List(context.Background(), &pvcs,
			client.ListOption(client.InNamespace(kafkaCluster.Namespace)),
			client.ListOption(client.MatchingLabels(map[string]string{"app": "kafka", "kafka_cr": kafkaCluster.Name})))
	}).Should(Succeed())

	Expect(pvcs.Items).To(HaveLen(1))
	pvc := pvcs.Items[0]
	Expect(pvc.GenerateName).To(Equal(fmt.Sprintf("%s-0-storage-0-", kafkaCluster.Name)))
	Expect(pvc.Labels).To(HaveKeyWithValue("app", "kafka"))
	Expect(pvc.Labels).To(HaveKeyWithValue("brokerId", "0"))
	Expect(pvc.Labels).To(HaveKeyWithValue("kafka_cr", kafkaCluster.Name))
	Expect(pvc.Annotations).To(HaveKeyWithValue("mountPath", "/kafka-logs"))
	Expect(pvc.Spec.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))
	Expect(pvc.Spec.Resources).To(Equal(corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"storage": resource.MustParse("10Gi"),
		},
	}))
}
