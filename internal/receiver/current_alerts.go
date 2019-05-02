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
	"sync"

	"github.com/prometheus/common/model"
)

// CurrentAlerts singleton interface
type CurrentAlerts interface {
	AddAlert(alertState) alertState
	AlertGC(alertState) error
	DeleteAlert(model.Fingerprint) error
	ListAlerts() map[model.Fingerprint]model.AlertStatus
}

type alertState struct {
	FingerPrint model.Fingerprint
	Status      model.AlertStatus
}

type currentAlerts struct {
	lock   sync.Mutex
	values map[model.Fingerprint]model.AlertStatus
}

var currAlert *currentAlerts

var once sync.Once

// GetCurrentAlerts get current stored alerts
func GetCurrentAlerts() CurrentAlerts {
	once.Do(func() {
		currAlert = &currentAlerts{}
		if currAlert.values == nil {
			currAlert.values = make(map[model.Fingerprint]model.AlertStatus)
		}
	})

	return currAlert
}

func (a *currentAlerts) AddAlert(alert alertState) alertState {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.values[alert.FingerPrint] = alert.Status

	return alert
}

func (a *currentAlerts) ListAlerts() map[model.Fingerprint]model.AlertStatus {
	return a.values
}

func (a *currentAlerts) DeleteAlert(alert model.Fingerprint) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	delete(a.values, alert)
	return nil
}

func (a *currentAlerts) AlertGC(alert alertState) error {
	if alert.Status == "resolved" {
		err := a.DeleteAlert(alert.FingerPrint)
		if err != nil {
			return err
		}
	}
	return nil
}
