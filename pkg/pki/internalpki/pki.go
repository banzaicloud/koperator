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

package internalpki

import (
	"context"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FinalizePKI returns nil since owner references handle secret cleanup
func (i *internalPKI) FinalizePKI(ctx context.Context, logger logr.Logger) error { return nil }

func (i *internalPKI) ReconcilePKI(ctx context.Context, logger logr.Logger, scheme *runtime.Scheme, externalHostnames []string) error {
	if i.cluster.Spec.ListenersConfig.SSLSecrets.Create {
		if err := ensureManagedPKI(ctx, i.client, i.cluster, scheme, externalHostnames); err != nil {
			return err
		}
	}
	return nil
}

func ensureManagedPKI(ctx context.Context, client client.Client, cluster *v1beta1.KafkaCluster, scheme *runtime.Scheme, externalHostnames []string) error {
	// ensure ca secret
	if _, _, err := getCA(ctx, client, cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err = createNewCA(ctx, client, cluster, scheme); err != nil {
			return err
		}
	}

	// ensure broker/controller secrets
	if err := ensureBrokerAndControllerUsers(ctx, client, cluster, externalHostnames); err != nil {
		return err
	}

	return nil
}

func ensureBrokerAndControllerUsers(ctx context.Context, client client.Client, cluster *v1beta1.KafkaCluster, externalHostnames []string) error {
	users := []*v1alpha1.KafkaUser{
		// Broker "user"
		pkicommon.BrokerUserForCluster(cluster, externalHostnames),
		// Operator user
		pkicommon.ControllerUserForCluster(cluster),
	}
	for _, user := range users {
		u := &v1alpha1.KafkaUser{}
		o := types.NamespacedName{Name: user.Name, Namespace: user.Namespace}
		if err := client.Get(ctx, o, u); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			if err := client.Create(ctx, user); err != nil {
				return err
			}
		}
	}
	return nil
}
