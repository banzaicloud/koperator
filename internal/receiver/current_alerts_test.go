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
	"testing"

	"github.com/prometheus/common/model"
)

func TestGetCurrentAlerts(t *testing.T) {
	alerts1 := GetCurrentAlerts()
	if alerts1 == nil {
		t.Error("expected pointer to Singleton after calling GetCurrentAlerts(), not nil")
	}

	singleAlerts := alerts1

	testAlert1 := alertState{
		FingerPrint: model.Fingerprint(1111),
		Status:      model.AlertStatus("fireing"),
		Labels: model.LabelSet{
			"alertname": "PodAllert",
			"test":      "test",
		},
	}

	testAlert2 := alertState{
		FingerPrint: model.Fingerprint(1111),
		Status:      model.AlertStatus("resolved"),
		Labels: model.LabelSet{
			"alertname": "PodAllert",
			"test":      "test",
		},
	}

	a1 := alerts1.AddAlert(testAlert1)
	if a1.FingerPrint != 1111 {
		t.Error("AdAlert failed a1")
	}

	list1 := alerts1.ListAlerts()
	if list1 == nil || list1[model.Fingerprint(1111)].Status != "fireing" || list1[model.Fingerprint(1111)].Labels["alertname"] != "PodAllert" {
		t.Error("Listing alerts failed a1")
	}

	alerts2 := GetCurrentAlerts()
	if alerts2 != singleAlerts {
		t.Error("Expected same instance in alerts2 but it got a different instance")
	}

	a2 := alerts2.AddAlert(testAlert2)
	if a2.FingerPrint != 1111 {
		t.Error("AdAlert failed a2")
	}

	list2 := alerts2.ListAlerts()
	if list2 == nil || list2[model.Fingerprint(1111)].Status != "resolved" || list2[model.Fingerprint(1111)].Labels["alertname"] != "PodAllert" {
		t.Error("Listing alerts failed a2")
	}

	alerts3 := GetCurrentAlerts()
	if alerts3.AlertGC(testAlert2) != nil {
		t.Error("Unable to delete alert a2")
	}

	list3 := alerts3.ListAlerts()
	if list3 == nil || list3[model.Fingerprint(1111)].Status != "" {
		t.Error("1111 alert wasn't deleted")
	}
}
