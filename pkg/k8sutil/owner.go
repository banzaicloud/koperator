/*
Copyright 2019 Banzai Cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sutil

import (
	"github.com/goph/emperror"
	metaapi "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type OwnerReferenceMatcher struct {
	owner              runtime.Object
	ownerMeta          metav1.Object
	ownerTypeGroupKind schema.GroupKind
	isController       bool
	scheme             *runtime.Scheme
}

// NewOwnerReferenceMatcher initializes a new owner reference matcher
func NewOwnerReferenceMatcher(owner runtime.Object, ctrl bool, scheme *runtime.Scheme) *OwnerReferenceMatcher {
	m := &OwnerReferenceMatcher{
		owner:        owner,
		isController: ctrl,
		scheme:       scheme,
	}

	meta, _ := metaapi.Accessor(owner)
	m.ownerMeta = meta

	m.setOwnerTypeGroupKind()

	return m
}

// Match matches if an object is owned by the initialised owner
func (e *OwnerReferenceMatcher) Match(object runtime.Object) (bool, metav1.Object, error) {
	o, err := metaapi.Accessor(object)
	if err != nil {
		return false, o, emperror.WrapWith(err, "could not access object meta", "kind", object.GetObjectKind())
	}

	for _, owner := range e.getOwnersReferences(o) {
		groupVersion, err := schema.ParseGroupVersion(owner.APIVersion)
		if err != nil {
			return false, o, emperror.WrapWith(err, "could not parse api version", "apiVersion", owner.APIVersion)
		}

		if owner.UID == e.ownerMeta.GetUID() && owner.Kind == e.ownerTypeGroupKind.Kind && groupVersion.Group == e.ownerTypeGroupKind.Group {
			return true, o, nil
		}
	}

	return false, o, nil
}

func (e *OwnerReferenceMatcher) getOwnersReferences(object metav1.Object) []metav1.OwnerReference {
	if object == nil {
		return nil
	}
	if !e.isController {
		return object.GetOwnerReferences()
	}
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		return []metav1.OwnerReference{*ownerRef}
	}
	return nil
}

func (e *OwnerReferenceMatcher) setOwnerTypeGroupKind() error {
	kinds, _, err := e.scheme.ObjectKinds(e.owner)
	if err != nil || len(kinds) < 1 {
		return emperror.WrapWith(err, "could not get object kinds", "owner", e.owner)
	}

	e.ownerTypeGroupKind = schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}
	return nil
}
