// Copyright © 2021 Banzai Cloud
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

package k8scsrpki

import (
	"context"

	"github.com/banzaicloud/kafka-operator/api/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c *k8sCSR) ReconcilePKI(
	ctx context.Context, logger logr.Logger, scheme *runtime.Scheme,
	externalHostnames map[string]v1beta1.ListenerStatusList) error {
	logger.Info("k8sCSR PKI reconcile is skipped since it is not supported yet for server certs")
	return nil
}

func (c *k8sCSR) FinalizePKI(ctx context.Context, logger logr.Logger) error {
	return nil
}
