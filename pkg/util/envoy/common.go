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

package envoy

const (
	// EnvoyServiceName name for loadbalancer service
	EnvoyServiceName = "envoy-loadbalancer-%s-%s"
	// EnvoyServiceNameWithScope name for loadbalancer service
	EnvoyServiceNameWithScope = "envoy-loadbalancer-%s-%s-%s"
	// IngressControllerName name for envoy ingress service
	IngressControllerName = "envoy"
)
