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

package kafka

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/ghodss/yaml"

	"emperror.dev/errors"
	corev1 "k8s.io/api/core/v1"

	apiutil "github.com/banzaicloud/koperator/api/util"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/resources/templates"

	"github.com/Masterminds/sprig/v3"
)

func (r *Reconciler) pvc(brokerId int32, storageIndex int, storage v1beta1.StorageConfig) (*corev1.PersistentVolumeClaim, error) {
	errCtx := []interface{}{v1beta1.BrokerIdLabelKey, brokerId, "mountPath", storage.MountPath}

	pvcSpecYaml, err := yaml.Marshal(storage.PvcSpec)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "couldn't unmarshal Pvc spec", errCtx...)
	}

	tpl, err := template.New("PvcSpec").Funcs(sprig.TxtFuncMap()).Parse(string(pvcSpecYaml))
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "couldn't parse Pvc spec", errCtx...)
	}
	var buf bytes.Buffer
	tplValues := struct {
		BrokerId  int32
		MountPath string
	}{
		BrokerId:  brokerId,
		MountPath: storage.MountPath,
	}
	err = tpl.Execute(&buf, tplValues)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "couldn't apply data to Pvc spec template from storage config", errCtx...)
	}

	var pvcSpec corev1.PersistentVolumeClaimSpec
	err = yaml.Unmarshal(buf.Bytes(), &pvcSpec)
	if err != nil {
		return nil, errors.WrapIfWithDetails(err, "couldn't unmarshal Pvc spec", errCtx...)
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: templates.ObjectMetaWithNameAndAnnotations(
			fmt.Sprintf(brokerStorageTemplate, r.KafkaCluster.Name, brokerId, storageIndex),
			apiutil.MergeLabels(
				apiutil.LabelsForKafka(r.KafkaCluster.Name),
				map[string]string{v1beta1.BrokerIdLabelKey: fmt.Sprintf("%d", brokerId)},
			),
			map[string]string{"mountPath": storage.MountPath}, r.KafkaCluster),
		Spec: pvcSpec,
	}, nil
}
