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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

// ObjectMeta returns a metav1.ObjectMeta object with labels, ownerReference and name
func ObjectMeta(name string, labels map[string]string, cluster *v1beta1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: cluster.Namespace,
		Labels:    ObjectMetaLabels(cluster, labels),
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

// ObjectMetaWithKafkaUserOwnerAndWithoutLabels returns a metav1.ObjectMeta object with ownerReference and name
func ObjectMetaWithKafkaUserOwnerAndWithoutLabels(name string, user *v1alpha1.KafkaUser) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: user.GetNamespace(),
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         user.APIVersion,
				Kind:               user.Kind,
				Name:               user.Name,
				UID:                user.UID,
				Controller:         util.BoolPointer(true),
				BlockOwnerDeletion: util.BoolPointer(true),
			},
		},
	}
}

// ObjectMetaWithoutOwnerRef returns a metav1.ObjectMeta object with labels, and name
func ObjectMetaWithoutOwnerRef(name string, labels map[string]string, cluster *v1beta1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: cluster.Namespace,
		Labels:    ObjectMetaLabels(cluster, labels),
	}
}

// ObjectMetaWithGeneratedName returns a metav1.ObjectMeta object with labels, ownerReference and generatedname
func ObjectMetaWithGeneratedName(namePrefix string, labels map[string]string, cluster *v1beta1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		GenerateName: namePrefix,
		Namespace:    cluster.Namespace,
		Labels:       ObjectMetaLabels(cluster, labels),
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

func ObjectMetaLabels(cluster *v1beta1.KafkaCluster, l map[string]string) map[string]string {
	if cluster.Spec.PropagateLabels {
		return util.MergeLabels(cluster.Labels, l)
	}
	return l
}

// ObjectMetaWithAnnotations returns a metav1.ObjectMeta object with labels, ownerReference, name and annotations
func ObjectMetaWithAnnotations(name string, labels map[string]string, annotations map[string]string, cluster *v1beta1.KafkaCluster) metav1.ObjectMeta {
	o := ObjectMeta(name, labels, cluster)
	o.Annotations = annotations
	return o
}

// ObjectMetaWithGeneratedNameAndAnnotations returns a metav1.ObjectMeta object with labels, ownerReference, generatedName and annotations
func ObjectMetaWithGeneratedNameAndAnnotations(namePrefix string, labels map[string]string, annotations map[string]string, cluster *v1beta1.KafkaCluster) metav1.ObjectMeta {
	o := ObjectMetaWithGeneratedName(namePrefix, labels, cluster)
	o.Annotations = annotations
	return o
}

// ObjectMetaClusterScope returns a metav1.ObjectMeta object with labels, ownerReference, name and annotations
func ObjectMetaClusterScope(name string, labels map[string]string, cluster *v1beta1.KafkaCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:   name,
		Labels: ObjectMetaLabels(cluster, labels),
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
