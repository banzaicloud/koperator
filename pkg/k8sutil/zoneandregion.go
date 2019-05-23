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

package k8sutil

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	zoneLabel   = "failure-domain.beta.kubernetes.io/zone"
	regionLabel = "failure-domain.beta.kubernetes.io/region"
)

// NodeZoneAndRegion holds information about Zone and Region
type NodeZoneAndRegion struct {
	Zone   string
	Region string
}

func determineNodeZoneAndRegion(nodeName string, client runtimeClient.Client) (*NodeZoneAndRegion, error) {

	node := &corev1.Node{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: nodeName, Namespace: ""}, node)
	if err != nil {
		return nil, err
	}
	return &NodeZoneAndRegion{
		Zone:   node.Labels[zoneLabel],
		Region: node.Labels[regionLabel],
	}, nil
}
