// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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

package clusterregistry

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

const defaultBrokerConfigGroup = "default"

func createMinimalKafkaClusterCR(name, namespace string) *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		Spec: v1beta1.KafkaClusterSpec{
			ListenersConfig: v1beta1.ListenersConfig{
				ExternalListeners: []v1beta1.ExternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Name:          "test",
							ContainerPort: 9094,
							Type:          "plaintext",
						},
						ExternalStartingPort: 19090,
						IngressServiceSettings: v1beta1.IngressServiceSettings{
							HostnameOverride: "test-host",
						},
						AccessMethod: corev1.ServiceTypeLoadBalancer,
					},
				},
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:                            "plaintext",
							Name:                            "internal",
							ContainerPort:                   29092,
							UsedForInnerBrokerCommunication: true,
						},
					},
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:                            "plaintext",
							Name:                            "controller",
							ContainerPort:                   29093,
							UsedForInnerBrokerCommunication: false,
						},
						UsedForControllerCommunication: true,
					},
				},
			},
			BrokerConfigGroups: map[string]v1beta1.BrokerConfig{
				defaultBrokerConfigGroup: {
					StorageConfigs: []v1beta1.StorageConfig{
						{
							MountPath: "/kafka-logs",
							PvcSpec: &corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceStorage: resource.MustParse("10Gi"),
									},
								},
							},
						},
					},
				},
			},
			Brokers: []v1beta1.Broker{
				{
					Id:                0,
					BrokerConfigGroup: defaultBrokerConfigGroup,
				},
				{
					Id:                1,
					BrokerConfigGroup: defaultBrokerConfigGroup,
				},
				{
					Id:                2,
					BrokerConfigGroup: defaultBrokerConfigGroup,
				},
			},
			ClusterImage: "ghcr.io/banzaicloud/kafka:2.13-3.1.0",
			ZKAddresses:  []string{},
			MonitoringConfig: v1beta1.MonitoringConfig{
				CCJMXExporterConfig: "custom_property: custom_value",
			},
			ReadOnlyConfig: "cruise.control.metrics.topic.auto.create=true",
		},
	}
}

type ReconcileResult struct {
	result reconcile.Result
	err    error
}

type TestReconciler struct {
	requests []reconcile.Request
	result   ReconcileResult
}

func (r *TestReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logr.FromContextOrDiscard(ctx)
	if r.requests == nil {
		r.requests = make([]reconcile.Request, 0)
	}
	log.Info("received event")
	r.requests = append(r.requests, request)

	return r.result.result, r.result.err
}

func (r *TestReconciler) WithResult(result reconcile.Result, err error) {
	r.result = ReconcileResult{
		result: result,
		err:    err,
	}
}

func (r *TestReconciler) Reset() {
	r.requests = make([]reconcile.Request, 0)
}

func (r TestReconciler) Requests() []reconcile.Request {
	return r.requests
}

func (r TestReconciler) NumOfRequests() int {
	return len(r.requests)
}

func NewTestReconciler() *TestReconciler {
	return &TestReconciler{
		requests: make([]reconcile.Request, 0),
		result:   ReconcileResult{result: reconcile.Result{}},
	}
}
