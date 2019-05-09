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
