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

package templates

import (
	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectMeta returns a metav1.ObjectMeta object with labels, ownerReference and name
func ObjectMeta(name string, labels map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: cluster.Namespace,
		Labels:    labels,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         util.BoolPointer(true),
				BlockOwnerDeletion: util.BoolPointer(true),
			},
		},
	}
}

// ObjectMetaWithGeneratedName returns a metav1.ObjectMeta object with labels, ownerReference and generatedname
func ObjectMetaWithGeneratedName(namePrefix string, labels map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		GenerateName: namePrefix,
		Namespace:    cluster.Namespace,
		Labels:       labels,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         util.BoolPointer(true),
				BlockOwnerDeletion: util.BoolPointer(true),
			},
		},
	}
}

// ObjectMetaWithAnnotations returns a metav1.ObjectMeta object with labels, ownerReference, name and annotations
func ObjectMetaWithAnnotations(name string, labels map[string]string, annotations map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	o := ObjectMeta(name, labels, cluster)
	o.Annotations = annotations
	return o
}

// ObjectMetaWithGeneratedNameAndAnnotations returns a metav1.ObjectMeta object with labels, ownerReference, generatedname and annotations
func ObjectMetaWithGeneratedNameAndAnnotations(namePrefix string, labels map[string]string, annotations map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	o := ObjectMetaWithGeneratedName(namePrefix, labels, cluster)
	o.Annotations = annotations
	return o
}

// ObjectMetaClusterScope returns a metav1.ObjectMeta object with labels, ownerReference, name and annotations
func ObjectMetaClusterScope(name string, labels map[string]string, cluster *banzaicloudv1alpha1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:   name,
		Labels: labels,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         util.BoolPointer(true),
				BlockOwnerDeletion: util.BoolPointer(true),
			},
		},
	}
}
