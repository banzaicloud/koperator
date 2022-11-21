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

package cruisecontrolmonitoring

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/banzaicloud/koperator/pkg/resources/templates"

	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) configMap() runtime.Object {
	configMap := &corev1.ConfigMap{
		ObjectMeta: templates.ObjectMeta(fmt.Sprintf(CruiseControlJmxTemplate, r.KafkaCluster.Name), labelsForJmx(r.KafkaCluster.Name), r.KafkaCluster),
		Data:       map[string]string{"config.yaml": r.KafkaCluster.Spec.MonitoringConfig.GetCCJMXExporterConfig()},
	}
	return configMap
}
