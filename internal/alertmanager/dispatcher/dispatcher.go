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

package dispatcher

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/banzaicloud/koperator/internal/alertmanager/currentalert"
)

// Dispatcher calls actors based on alert annotations
func Dispatcher(ctx context.Context, promAlerts []model.Alert, log logr.Logger, client client.Client) {
	storedAlerts := currentalert.GetCurrentAlerts()
	for _, promAlert := range alertFilter(promAlerts) {
		store := currentalert.AlertState{
			FingerPrint: promAlert.Fingerprint(),
			Status:      promAlert.Status(),
			Labels:      promAlert.Labels,
			Annotations: promAlert.Annotations,
		}
		storedAlerts.AddAlert(store)
		err := storedAlerts.AlertGC(store)
		if err != nil {
			log.Error(err, "alerts garbage collection failed")
		}
	}
	rollingUpgradeAlertCount := storedAlerts.GetRollingUpgradeAlertCount()
	for key, value := range storedAlerts.ListAlerts() {
		log.Info("Stored Alert", "key", key, "status", value.Status, "labels", value.Labels, "annotations", value.Annotations, "processed", value.Processed)
		_, err := storedAlerts.HandleAlert(ctx, key, client, rollingUpgradeAlertCount, log)
		if err != nil {
			log.Error(err, "failed to handle alert", "fingerprint", key)
		}
	}
}

func alertFilter(promAlerts []model.Alert) []model.Alert {
	supportedCommandList := currentalert.GetCommandList()
	filteredAlerts := []model.Alert{}
	for _, alert := range promAlerts {
		if annotation, annotationOK := alert.Annotations["command"]; annotationOK {
			for _, command := range supportedCommandList {
				if string(annotation) == command {
					filteredAlerts = append(filteredAlerts, alert)
				}
			}
		}
	}
	return filteredAlerts
}
