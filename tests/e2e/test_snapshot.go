// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package e2e

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type clusterSnapshot struct {
	resources []metav1.PartialObjectMetadata
}

func (s *clusterSnapshot) Resources() []metav1.PartialObjectMetadata {
	return s.resources
}

// ResourcesAsComparisonType returns a slice of a helper type that makes comparisons easier
func (s *clusterSnapshot) ResourcesAsComparisonType() []localComparisonPartialObjectMetadataType {
	var localList []localComparisonPartialObjectMetadataType
	for _, r := range s.resources {
		localList = append(localList, localComparisonPartialObjectMetadataType{
			GVK:       r.GroupVersionKind(),
			Namespace: r.GetNamespace(),
			Name:      r.GetName(),
		})
	}
	return localList
}

// localComparisonPartialObjectMetadataType holds a version of the minimal information required
// to compare k8s.io/apimachinery/pkg/apis/meta/v1.PartialObjectMetadata instances
type localComparisonPartialObjectMetadataType struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

// snapshotCluster takes a clusterSnapshot of a K8s cluster and
// stores it into the snapshotCluster instance referenced as input
func snapshotCluster(snapshottedInfo *clusterSnapshot) bool { //nolint:unparam // Note: respecting Ginkgo testing interface by returning bool.
	return When("Get cluster resources state", Ordered, func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		BeforeAll(func() {
			By("Acquiring K8s config and context")
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		var clusterResourceNames []string
		var namespacedResourceNames []string

		When("Get api-resources names", func() {
			It("Get cluster-scoped api-resources names", func() {
				clusterResourceNames, err = listK8sResourceKinds(kubectlOptions, "", "--namespaced=false")
				Expect(err).NotTo(HaveOccurred())
				Expect(clusterResourceNames).NotTo(BeNil())
				clusterResourceNames = pruneUnnecessaryClusterResourceNames(clusterResourceNames)
			})
			It("Get namespaced api-resources names", func() {
				namespacedResourceNames, err = listK8sResourceKinds(kubectlOptions, "", "--namespaced=true")
				Expect(err).NotTo(HaveOccurred())
				Expect(namespacedResourceNames).NotTo(BeNil())
				namespacedResourceNames = pruneUnnecessaryNamespacedResourceNames(namespacedResourceNames)
			})
		})

		var resources []metav1.PartialObjectMetadata

		var namespacesForNamespacedResources = []string{"default"}

		When("Snapshotting objects", func() {
			It("Recording cluster-scoped resource objects", func() {
				By(fmt.Sprintf("Getting cluster-scoped resources %v as json", clusterResourceNames))
				output, err := getK8sResources(kubectlOptions, clusterResourceNames, "", "", "--output=json")
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Unmarshalling cluster-scoped resources %v from json", clusterResourceNames))
				var resourceList metav1.PartialObjectMetadataList
				err = json.Unmarshal([]byte(strings.Join(output, "\n")), &resourceList)
				Expect(err).NotTo(HaveOccurred())

				resources = append(resources, resourceList.Items...)
			})
			It("Recording namespaced resource objects", func() {
				initialNS := kubectlOptions.Namespace
				for _, ns := range namespacesForNamespacedResources {
					kubectlOptions.Namespace = ns

					By(fmt.Sprintf("Getting namespaced resources %v as json for namespace %s", namespacedResourceNames, ns))
					output, err := getK8sResources(kubectlOptions, namespacedResourceNames, "", "", "--output=json")
					Expect(err).NotTo(HaveOccurred())

					By(fmt.Sprintf("Unmarshalling namespaced resources %v from json for namespace %s", namespacedResourceNames, ns))
					var resourceList metav1.PartialObjectMetadataList
					err = json.Unmarshal([]byte(strings.Join(output, "\n")), &resourceList)
					Expect(err).NotTo(HaveOccurred())

					resources = append(resources, resourceList.Items...)
				}
				kubectlOptions.Namespace = initialNS
			})
		})

		AfterAll(func() {
			By("Storing recorded objects into the input snapshot object")
			snapshottedInfo.resources = resources
		})
	})
}

// snapshotClusterAndCompare takes a current snapshot of the K8s cluster and
// compares it against a snapshot provided as input
func snapshotClusterAndCompare(snapshottedInitialInfo *clusterSnapshot) bool {
	return When("Verifying cluster resources state", Ordered, func() {
		var snapshottedCurrentInfo = &clusterSnapshot{}
		snapshotCluster(snapshottedCurrentInfo)

		It("Checking resources list", func() {
			// Temporarily increase maximum output length (default 4000) to fit more objects in the printed diff.
			// Only doing this here because other assertions typically don't run against objects with this many elements.
			initialMaxLength := format.MaxLength
			defer func() { format.MaxLength = initialMaxLength }()
			format.MaxLength = 9000

			Expect(snapshottedCurrentInfo.ResourcesAsComparisonType()).To(ConsistOf(snapshottedInitialInfo.ResourcesAsComparisonType()))
		})
	})
}

func pruneUnnecessaryClusterResourceNames(resourceNameList []string) []string {
	var updatedList []string
	for _, name := range resourceNameList {
		// Avoid failing because the number of K8s workers changed during the test. (e.g. PKE)
		if name == "nodes" {
			continue
		}
		// When the number of nodes changes we also get CSRs for signers kubernetes.io/kubelet-serving and kubernetes.io/kube-apiserver-client-kubelet
		// TODO: in time, we want to be able to compare CSRs, too, or be able to ignore particular CSR list differences.
		if name == "certificatesigningrequests.certificates.k8s.io" {
			continue
		}
		// Ignore CSI elements from storage.k8s.io
		// Additionally, these resources don't mesh well with computing differences for clusters with a variable number of workers.
		if name == "csidrivers.storage.k8s.io" || name == "csinodes.storage.k8s.io" || name == "csistoragecapacities.storage.k8s.io" {
			continue
		}
		// We never need to snapshot Cilium-related resources (namespaced or not).
		if strings.HasPrefix(name, "cilium") {
			continue
		}
		updatedList = append(updatedList, name)
	}
	return updatedList
}

func pruneUnnecessaryNamespacedResourceNames(resourceNameList []string) []string {
	var updatedList []string
	for _, name := range resourceNameList {
		// The list of K8s Events is rarely unchanged over time. It is not fit for comparison.
		// Additionally, at the very least, KafkaCluster installs create PVs which generate events by themselves.
		if name == "events" || name == "events.events.k8s.io" {
			continue
		}
		// We never need to snapshot Cilium-related resources (namespaced or not).
		if strings.HasPrefix(name, "cilium") {
			continue
		}
		updatedList = append(updatedList, name)
	}
	return updatedList
}
