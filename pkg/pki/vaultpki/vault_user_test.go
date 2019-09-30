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

package vaultpki

import (
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func newMockUser() *v1alpha1.KafkaUser {
	user := &v1alpha1.KafkaUser{}
	user.Name = "test-user"
	user.Namespace = "test-namespace"
	user.UID = types.UID("test-uid")
	user.Spec = v1alpha1.KafkaUserSpec{SecretName: "secret/test-secret", IncludeJKS: true}
	return user
}
