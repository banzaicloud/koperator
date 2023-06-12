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
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/util"
)

type KafkaUserValidator struct {
	Client client.Client
	Log    logr.Logger
}

// ValidateCreate validates if the user-provided KafkaUser specifications valid for creation
func (validator KafkaUserValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return validator.validate(ctx, obj)
}

// ValidateUpdate validates if the user-provided KafkaUser specifications valid for update
func (validator KafkaUserValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	return validator.validate(ctx, newObj)
}

// ValidateDeleet validates if the user-selctd KafkaUser specifications valid for deletion
func (validator KafkaUserValidator) ValidateDelete(_ context.Context, _ runtime.Object) error {
	// nothing to validate on deletion
	return nil
}

func (validator KafkaUserValidator) validate(ctx context.Context, obj runtime.Object) error {
	var (
		kafkaUser = obj.(*banzaicloudv1alpha1.KafkaUser)
		log       = validator.Log.WithValues("name", kafkaUser.GetName(), "namespace", kafkaUser.GetNamespace())
	)

	fieldErrs, err := validator.validateKafkaUser(ctx, kafkaUser)
	if err != nil {
		log.Error(err, errorDuringValidationMsg)
		return apierrors.NewInternalError(errors.WithMessage(err, errorDuringValidationMsg))
	}

	if len(fieldErrs) == 0 {
		return nil
	}

	log.Info("rejected", "invalid field(s)", fieldErrs.ToAggregate().Error())

	return apierrors.NewInvalid(kafkaUser.GetObjectKind().GroupVersionKind().GroupKind(), kafkaUser.Name, fieldErrs)
}

func (validator KafkaUserValidator) validateKafkaUser(ctx context.Context, kafkaUser *banzaicloudv1alpha1.KafkaUser) (field.ErrorList, error) {
	var (
		err     error
		allErrs field.ErrorList
		logMsg  string

		kafkaCluster     *v1beta1.KafkaCluster
		clusterNamespace = util.GetClusterRefNamespace(kafkaUser.GetNamespace(), kafkaUser.Spec.ClusterRef)
		clusterName      = kafkaUser.Spec.ClusterRef.Name
	)

	if kafkaCluster, err = k8sutil.LookupKafkaCluster(ctx, validator.Client, clusterName, clusterNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			logMsg = fmt.Sprintf("KafkaCluster CR '%s' in the namespace '%s' does not exist", clusterName, clusterNamespace)
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterRef"), kafkaUser.Spec.ClusterRef, logMsg))
			return allErrs, nil
		}

		return nil, err
	}

	if k8sutil.IsMarkedForDeletion(kafkaCluster.ObjectMeta) {
		return nil, errors.New("referenced KafkaCluster CR is being deleted")
	}

	if util.ObjectManagedByClusterRegistry(kafkaCluster) {
		// referencing remote Kafka clusters is not allowed
		logMsg = fmt.Sprintf("KafkaCluster CR '%s' in the namespace '%s' is a remote resource", clusterName, clusterNamespace)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterRef"), kafkaUser.Spec.ClusterRef, logMsg))
	}

	if kafkaUser.Spec.GetIfCertShouldBeCreated() {
		// avoid panic if the user wants to create a kafka user but the cluster is in plaintext mode
		if kafkaCluster.Spec.ListenersConfig.SSLSecrets == nil && kafkaUser.Spec.PKIBackendSpec == nil {
			logMsg = fmt.Sprintf("KafkaCluster CR '%s' in namespace '%s' is in plaintext mode, "+
				"therefore 'spec.pkiBackendSpec' must be provided to create certificate", clusterName, clusterNamespace)
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("pkiBackendSpec"), kafkaUser.Spec.PKIBackendSpec, logMsg))
		}
	}

	return allErrs, nil
}
