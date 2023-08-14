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

package mocks

import (
	"context"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/scale"
)

func NewMockScaleFactory(mock scale.CruiseControlScaler) func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
	return func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
		return mock, nil
	}
}
