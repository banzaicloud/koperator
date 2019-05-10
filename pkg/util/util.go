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

package util

import (
	"strings"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func IntstrPointer(i int) *intstr.IntOrString {
	is := intstr.FromInt(i)
	return &is
}

func Int64Pointer(i int64) *int64 {
	return &i
}

func Int32Pointer(i int32) *int32 {
	return &i
}

func BoolPointer(b bool) *bool {
	return &b
}

func StringPointer(s string) *string {
	return &s
}

func MergeLabels(l map[string]string, l2 map[string]string) map[string]string {
	if l == nil {
		l = make(map[string]string)
	}
	for lKey, lValue := range l2 {
		l[lKey] = lValue
	}
	return l
}

func MonitoringAnnotations() map[string]string {
	return map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/probe":  "cruisecontrol",
		"prometheus.io/port":   "9020",
	}
}

func IsSSLEnabledForInternalCommunication(l []banzaicloudv1alpha1.InternalListenerConfig) (enabled bool) {

	for _, listener := range l {
		if strings.ToLower(listener.Type) == "ssl" {
			enabled = true
			break
		}
	}
	return enabled
}
