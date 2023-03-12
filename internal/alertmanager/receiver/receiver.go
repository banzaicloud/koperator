// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

package receiver

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/internal/alertmanager/dispatcher"
)

func alertReciever(ctx context.Context, log logr.Logger, alert []byte, client client.Client) error {
	promAlerts := make([]model.Alert, 0)
	err := json.Unmarshal(alert, &promAlerts)
	if err != nil {
		return err
	}

	dispatcher.Dispatcher(ctx, promAlerts, log, client)
	return nil
}
