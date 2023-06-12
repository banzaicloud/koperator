// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package webhooks

import (
	"context"
	"testing"
	"time"

	"emperror.dev/errors"
	clusterregv1alpha1 "github.com/cisco-open/cluster-registry-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util"
)

func newMockClusterBeingDeleted() *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KafkaCluster",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster-being-deleted",
			Namespace:         "test-namespace",
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
		Spec: v1beta1.KafkaClusterSpec{},
	}
}

func newMockClusterWithNoSSLSecrets() *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KafkaCluster",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-no-sslsecrets",
			Namespace: "test-namespace",
		},
		Spec: v1beta1.KafkaClusterSpec{
			ListenersConfig: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{
						CommonListenerSpec: v1beta1.CommonListenerSpec{
							Type:          "plaintext",
							Name:          "test-listener",
							ContainerPort: 9091,
						},
					},
				},
			},
		},
	}
}

func newMockRemoteCluster() *v1beta1.KafkaCluster {
	return &v1beta1.KafkaCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KafkaCluster",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster-remote",
			Namespace:   "test-namespace",
			Annotations: map[string]string{clusterregv1alpha1.OwnershipAnnotation: "test-val"},
		},
		Spec: v1beta1.KafkaClusterSpec{},
	}
}

func newMockClient() runtimeClient.Client {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func TestValidateKafkaUser(t *testing.T) {
	testCases := []struct {
		testName       string
		kafkaUser      *v1alpha1.KafkaUser
		kafkaCluster   *v1beta1.KafkaCluster
		expectedErrors error
	}{
		{
			testName: "referenced KafkaCluster CR is not found - name misconfigured",
			kafkaUser: &v1alpha1.KafkaUser{
				TypeMeta:   metav1.TypeMeta{Kind: "KafkaUser", APIVersion: "kafka.banzaicloud.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "test-namespace"},
				Spec: v1alpha1.KafkaUserSpec{
					ClusterRef: v1alpha1.ClusterReference{
						Name:      "invalid-name",
						Namespace: "test-namespace",
					},
				},
			},
			kafkaCluster: newMockCluster(),
			expectedErrors: apierrors.NewInvalid(schema.GroupKind{Group: "kafka.banzaicloud.io", Kind: "KafkaUser"},
				"test-user", append(field.ErrorList{}, field.Invalid(
					field.NewPath("spec").Child("clusterRef"),
					v1alpha1.ClusterReference{Name: "invalid-name", Namespace: "test-namespace"},
					"KafkaCluster CR 'invalid-name' in the namespace 'test-namespace' does not exist"))),
		},
		{
			testName: "referenced KafkaCluster CR is not found - namespace misconfigured",
			kafkaUser: &v1alpha1.KafkaUser{
				TypeMeta:   metav1.TypeMeta{Kind: "KafkaUser", APIVersion: "kafka.banzaicloud.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "test-namespace"},
				Spec: v1alpha1.KafkaUserSpec{
					ClusterRef: v1alpha1.ClusterReference{
						Name:      "test-cluster",
						Namespace: "invalid-namespace",
					},
				},
			},
			kafkaCluster: newMockCluster(),
			expectedErrors: apierrors.NewInvalid(schema.GroupKind{Group: "kafka.banzaicloud.io", Kind: "KafkaUser"},
				"test-user", append(field.ErrorList{}, field.Invalid(
					field.NewPath("spec").Child("clusterRef"), v1alpha1.ClusterReference{Name: "test-cluster", Namespace: "invalid-namespace"},
					"KafkaCluster CR 'test-cluster' in the namespace 'invalid-namespace' does not exist"))),
		},
		{
			testName: "referenced KafkaCluster CR is being deleted",
			kafkaUser: &v1alpha1.KafkaUser{
				TypeMeta:   metav1.TypeMeta{Kind: "KafkaUser", APIVersion: "kafka.banzaicloud.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "test-namespace"},
				Spec: v1alpha1.KafkaUserSpec{
					ClusterRef: v1alpha1.ClusterReference{
						Name:      "test-cluster-being-deleted",
						Namespace: "test-namespace",
					},
				},
			},
			kafkaCluster:   newMockClusterBeingDeleted(),
			expectedErrors: apierrors.NewInternalError(errors.WithMessage(errors.New("referenced KafkaCluster CR is being deleted"), errorDuringValidationMsg)),
		},
		{
			testName: "referenced KafkaCluster CR is a remote CR",
			kafkaUser: &v1alpha1.KafkaUser{
				TypeMeta:   metav1.TypeMeta{Kind: "KafkaUser", APIVersion: "kafka.banzaicloud.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "test-namespace"},
				Spec: v1alpha1.KafkaUserSpec{
					ClusterRef: v1alpha1.ClusterReference{
						Name:      "test-cluster-remote",
						Namespace: "test-namespace",
					},
					CreateCert: util.BoolPointer(false),
				},
			},
			kafkaCluster: newMockRemoteCluster(),
			expectedErrors: apierrors.NewInvalid(schema.GroupKind{Group: "kafka.banzaicloud.io", Kind: "KafkaUser"},
				"test-user", append(field.ErrorList{}, field.Invalid(
					field.NewPath("spec").Child("clusterRef"), v1alpha1.ClusterReference{Name: "test-cluster-remote", Namespace: "test-namespace"},
					"KafkaCluster CR 'test-cluster-remote' in the namespace 'test-namespace' is a remote resource"))),
		},
		{
			testName: "certificate needs to be created for the KafkaUser but the corresponding KafkaCluster's" +
				" listenerConfig doesn't have SSL secrets and there is no PKIBackendSpec specified under the KafkaUser",
			kafkaUser: &v1alpha1.KafkaUser{
				TypeMeta:   metav1.TypeMeta{Kind: "KafkaUser", APIVersion: "kafka.banzaicloud.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "test-namespace"},
				Spec: v1alpha1.KafkaUserSpec{
					ClusterRef: v1alpha1.ClusterReference{
						Name:      "test-cluster-no-sslsecrets",
						Namespace: "test-namespace",
					},
					CreateCert: util.BoolPointer(true),
				},
			},
			kafkaCluster: newMockClusterWithNoSSLSecrets(),
			expectedErrors: apierrors.NewInvalid(schema.GroupKind{Group: "kafka.banzaicloud.io", Kind: "KafkaUser"},
				"test-user", append(field.ErrorList{}, field.Invalid(
					field.NewPath("spec").Child("pkiBackendSpec"), nil,
					"KafkaCluster CR 'test-cluster-no-sslsecrets' in namespace 'test-namespace' is in plaintext"+
						" mode, therefore 'spec.pkiBackendSpec' must be provided to create certificate"))),
		},
	}

	ctx := context.Background()

	for _, testCase := range testCases {
		test := testCase
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()
			userValidator := KafkaUserValidator{
				Client: newMockClient(),
				Log:    logr.Discard(),
			}

			err := userValidator.Client.Create(ctx, test.kafkaCluster)
			require.Nil(t, err)

			actualErrs := userValidator.ValidateCreate(context.Background(), test.kafkaUser)
			require.Equal(t, test.expectedErrors, actualErrs)
		})
	}
}
