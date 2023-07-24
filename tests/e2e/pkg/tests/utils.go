package tests

import (
	"errors"
	"fmt"
	"regexp"
	"sort"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
)

type k8sProvider string

const (
	k8sProviderGKE     k8sProvider = "GKE"
	k8sProviderPKE     k8sProvider = "PKE"
	k8sProviderAKS     k8sProvider = "AKS"
	k8sProviderKind    k8sProvider = "Kind"
	k8sProviderEKS     k8sProvider = "EKS"
	k8sProviderUnknown k8sProvider = "Unknown"

	k8sVersionRegexp = "^v[0-9].[0-9]+.[0-9]+"
)

var k8sProviderRegexp = map[k8sProvider]string{
	k8sProviderGKE:  "^gke-",
	k8sProviderAKS:  "^aks-",
	k8sProviderPKE:  ".compute.internal", //TODO (marbarta): this is not so good (is not unique PKE)
	k8sProviderEKS:  ".[0-9]-eks-",
	k8sProviderKind: "^kind",
}

func versionIdentifier(version string) (string, error) {
	version = regexp.MustCompile(k8sVersionRegexp).FindString(version)
	if version == "" {
		return "", fmt.Errorf("K8s cluster version could not be recognized: '%s'", version)
	}
	return version, nil
}

func providerIdentifier(node corev1.Node) (k8sProvider, error) {
	checkString := node.Name + node.Status.NodeInfo.KubeletVersion
	var foundProvider k8sProvider
	for provider, regexpProvider := range k8sProviderRegexp {
		if regexp.MustCompile(regexpProvider).MatchString(checkString) {
			if foundProvider != "" {
				return "", fmt.Errorf("K8s cluster provider name matched for multiple patterns: '%s' and '%s'", foundProvider, provider)
			}
			foundProvider = provider
		}
	}
	if foundProvider == "" {
		return "", errors.New("provider could not been identified")
	}
	return foundProvider, nil
}

func sortedKeys[K constraints.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(maps.Keys(m)))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
