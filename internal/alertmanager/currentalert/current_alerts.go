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

package currentalert

import (
	"errors"
	"sync"

	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CurrentAlerts interface
type CurrentAlerts interface {
	AddAlert(AlertState) *currentAlertStruct
	AlertGC(AlertState) error
	DeleteAlert(model.Fingerprint) error
	ListAlerts() map[model.Fingerprint]*currentAlertStruct
	HandleAlert(model.Fingerprint, client.Client, int) (*currentAlertStruct, error)
	GetRollingUpgradeAlertCount() int
}

// AlertState current alert state
type AlertState struct {
	FingerPrint model.Fingerprint
	Status      model.AlertStatus
	Labels      model.LabelSet
	Annotations model.LabelSet
}

type currentAlerts struct {
	lock   sync.Mutex
	alerts map[model.Fingerprint]*currentAlertStruct
}

type currentAlertStruct struct {
	Status      model.AlertStatus
	Labels      model.LabelSet
	Annotations model.LabelSet
	Processed   bool
}

var currAlert *currentAlerts

var once sync.Once

// GetCurrentAlerts get current stored alerts
func GetCurrentAlerts() CurrentAlerts {
	once.Do(func() {
		currAlert = &currentAlerts{}
		if currAlert.alerts == nil {
			currAlert.alerts = make(map[model.Fingerprint]*currentAlertStruct)
		}
	})

	return currAlert
}

func (a *currentAlerts) AddAlert(alert AlertState) *currentAlertStruct {
	a.lock.Lock()
	defer a.lock.Unlock()
	if _, ok := a.alerts[alert.FingerPrint]; !ok {
		a.alerts[alert.FingerPrint] = &currentAlertStruct{
			Status:      alert.Status,
			Labels:      alert.Labels,
			Annotations: alert.Annotations,
		}
	}
	return a.alerts[alert.FingerPrint]
}

func (a *currentAlerts) ListAlerts() map[model.Fingerprint]*currentAlertStruct {
	return a.alerts
}

func (a *currentAlerts) DeleteAlert(alertFp model.Fingerprint) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	delete(a.alerts, alertFp)
	return nil
}

func (a *currentAlerts) AlertGC(alert AlertState) error {
	if alert.Status == "resolved" {
		err := a.DeleteAlert(alert.FingerPrint)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *currentAlerts) HandleAlert(alertFp model.Fingerprint, client client.Client, rollingUpgradeAlertCount int) (*currentAlertStruct, error) {
	a.lock.Lock()
	defer a.lock.Unlock()
	if _, ok := a.alerts[alertFp]; !ok {
		return &currentAlertStruct{}, errors.New("alert doesn't exist")
	}
	if a.alerts[alertFp].Processed != true {
		err := examineAlert(a.alerts[alertFp], client, rollingUpgradeAlertCount)
		if err != nil {
			return nil, err
		}
		a.alerts[alertFp].Processed = true
	}
	return a.alerts[alertFp], nil
}

func (a *currentAlerts) GetRollingUpgradeAlertCount() int {
	alertCount := 0
	if len(a.alerts) > 0 {
		for _, alert := range a.alerts {
			for key := range alert.Labels {
				if key == "rollingupgrade" {
					alertCount++
				}
			}
		}
	}
	return alertCount
}
