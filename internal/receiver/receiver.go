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

package receiver

import (
	"encoding/json"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
)

func alertReciever(log logr.Logger, alert []byte) {
	promAlerts := make([]model.Alert, 0)
	_ = json.Unmarshal(alert, &promAlerts)
	log.Info("FROM", "promentheus", promAlerts)

	storedAlerts := GetCurrentAlerts()
	for _, promAlert := range promAlerts {
		store := alertState{
			FingerPrint: promAlert.Fingerprint(),
			Status:      promAlert.Status(),
			Labels:      promAlert.Labels,
		}
		storedAlerts.AddAlert(store)
		storedAlerts.AlertGC(store)
	}
	for key, value := range storedAlerts.ListAlerts() {
		log.Info("Stored Alert", "key", key, "value", value.Status, "labels", value.Labels)
	}
}
