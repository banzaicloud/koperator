// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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

package k8sutil

import (
	"context"

	"emperror.dev/errors"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/api/v1alpha1"
)

func AddKafkaTopicIndexers(ctx context.Context, cache cache.Cache) error {
	nameIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.KafkaTopic).Spec.Name}
	}
	err := cache.IndexField(ctx, &v1alpha1.KafkaTopic{}, "spec.name", nameIndexFunc)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not setup indexer for field", "field", "spec.name")
	}

	clusterNameIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.KafkaTopic).Spec.ClusterRef.Name}
	}
	err = cache.IndexField(ctx, &v1alpha1.KafkaTopic{}, "spec.clusterRef.name", clusterNameIndexFunc)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not setup indexer for field", "field", "spec.clusterRef.name")
	}

	clusterNamespaceIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.KafkaTopic).Spec.ClusterRef.Namespace}
	}
	err = cache.IndexField(ctx, &v1alpha1.KafkaTopic{}, "spec.clusterRef.namespace", clusterNamespaceIndexFunc)
	if err != nil {
		return errors.WrapIfWithDetails(err, "could not setup indexer for field", "field", "spec.clusterRef.namespace")
	}
	return nil
}
