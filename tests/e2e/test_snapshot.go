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
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func snapshotCluster(snapshottedInfo *clusterSnapshot) bool {
	return When("Get cluster resources state", Ordered, func() {
		var kubectlOptions k8s.KubectlOptions
		var err error

		BeforeAll(func() {
			By("Acquiring K8s config and context")
			kubectlOptions, err = kubectlOptionsForCurrentContext()
			Expect(err).NotTo(HaveOccurred())
		})

		/*
			var namespaces []string

			When("Get namespaces", func() {
				It("List namespaces", func() {
					// output, err := k8s.RunKubectlAndGetOutputE(
					// 	GinkgoT(),
					// 	&kubectlOptions,
					// 	[]string{"get", "namespaces", kubectlArgGoTemplateName}...,
					// )
					// Expect(err).NotTo(HaveOccurred())
					// namespaces = outputToSliceOfStringWithoutWarnings(output, "\n")

					namespaces, err = getK8sResources(kubectlOptions, []string{"namespaces"}, "", "", kubectlArgGoTemplateName)
					Expect(err).NotTo(HaveOccurred())
				})
			})


			var crds []string

			When("Get CRDs", func() {
				It("List CRDs", func() {
					// output, err := k8s.RunKubectlAndGetOutputE(
					// 	GinkgoT(),
					// 	&kubectlOptions,
					// 	[]string{"get", "crds", kubectlArgGoTemplateName}...,
					// )
					// Expect(err).NotTo(HaveOccurred())
					// crds = removeUnnecessaryResourceNames(outputToSliceOfStringWithoutWarnings(output, "\n"))

					crds, err = listK8sCRDs(kubectlOptions)
					Expect(err).NotTo(HaveOccurred())
					crds = removeUnnecessaryResourceNames(crds)
				})
			})
		*/

		var clusterResourceNames []string
		var namespacedResourceNames []string

		When("Get api-resources", func() {
			It("Run kubectl api-resources --namespaced=false", func() {
				// output, err := k8s.RunKubectlAndGetOutputE(
				// 	GinkgoT(),
				// 	&kubectlOptions,
				// 	[]string{"api-resources", "--verbs=list", "--namespaced=false", "--output=name"}...,
				// )
				// Expect(err).NotTo(HaveOccurred())

				// clusterResourceNames = removeUnnecessaryResourceNames(outputToSliceOfStringWithoutWarnings(output, "\n"))

				clusterResourceNames, err = listK8sResourceKinds(kubectlOptions, "", "--namespaced=false")
				Expect(err).NotTo(HaveOccurred())
				clusterResourceNames = pruneUnnecessaryClusterResourceNames(clusterResourceNames)
			})
			It("Run kubectl api-resources --namespaced=true", func() {
				// output, err := k8s.RunKubectlAndGetOutputE(
				// 	GinkgoT(),
				// 	&kubectlOptions,
				// 	[]string{"api-resources", "--verbs=list", "--namespaced=true", "--output=name"}...,
				// )
				// Expect(err).NotTo(HaveOccurred())

				// namespacedResourceNames = removeUnnecessaryResourceNames(outputToSliceOfStringWithoutWarnings(output, "\n"))

				namespacedResourceNames, err = listK8sResourceKinds(kubectlOptions, "", "--namespaced=true")
				Expect(err).NotTo(HaveOccurred())
				namespacedResourceNames = pruneUnnecessaryNamespacedResourceNames(namespacedResourceNames)
			})
		})

		var resources []metav1.PartialObjectMetadata

		//var namespacesForNamespacedResources = []string{"default", "kafka"}
		var namespacesForNamespacedResources = []string{"default"}

		When("Snapshotting objects", func() {
			It("Recording cluster-scoped resource objects", func() {
				// for _, r := range clusterResourceNames {
				// 	//for _, r := range []string{"mutatingwebhookconfigurations.admissionregistration.k8s.io", "validatingwebhookconfigurations.admissionregistration.k8s.io", "customresourcedefinitions.apiextensions.k8s.io", "clusterissuers.cert-manager.io"} {
				// 	By(fmt.Sprintf("Getting cluster-scoped resource %s from json", r))
				// 	output, err := k8s.RunKubectlAndGetOutputE(
				// 		GinkgoT(),
				// 		&kubectlOptions,
				// 		[]string{"get", r, "--ignore-not-found", "--output=json"}...,
				// 	)
				// 	Expect(err).NotTo(HaveOccurred())

				// 	By(fmt.Sprintf("Unmarshalling cluster resource %s list from json", r))
				// 	// Get rid of "Warning:" lines present at the beginning of some outputs
				// 	outputWithoutWarnings := strings.Join(outputToSliceOfStringWithoutWarnings(output, "\n"), "\n")
				// 	if outputWithoutWarnings != "" {
				// 		var resourceList metav1.PartialObjectMetadataList
				// 		err = json.Unmarshal([]byte(outputWithoutWarnings), &resourceList)
				// 		Expect(err).NotTo(HaveOccurred())

				// 		resources = append(resources, resourceList.Items...)
				// 	}
				// }

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
					// for _, r := range namespacedResourceNames {
					// 	By(fmt.Sprintf("Getting namespaced resource %s for namespace %s", r, ns))
					// 	output, err := k8s.RunKubectlAndGetOutputE(
					// 		GinkgoT(),
					// 		&kubectlOptions,
					// 		[]string{"get", r, "--namespace", ns, "--ignore-not-found", "--output=json"}...,
					// 	)
					// 	Expect(err).NotTo(HaveOccurred())

					// 	By(fmt.Sprintf("Unmarshaling namespaced resource %s list from json within namespace %s", r, ns))
					// 	outputWithoutWarnings := strings.Join(outputToSliceOfStringWithoutWarnings(output, "\n"), "\n")
					// 	if outputWithoutWarnings != "" {
					// 		var resourceList metav1.PartialObjectMetadataList
					// 		err = json.Unmarshal([]byte(outputWithoutWarnings), &resourceList)
					// 		Expect(err).NotTo(HaveOccurred())

					// 		resources = append(resources, resourceList.Items...)
					// 	}
					// }
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

		/*
			It("Debug info on accumulated variables", func() {
				// fmt.Println("\n###\nnamespaces =", namespaces, "with len", len(namespaces), "\n---")
				// fmt.Println("\n###\ncrds =", crds, "with len", len(crds), "\n---")
				fmt.Println("\n###\nnamespacedResourceNames =", namespacedResourceNames, "with len", len(namespacedResourceNames), "\n---")
				fmt.Println("\n###\nclusterResourceNames =", clusterResourceNames, "with len", len(clusterResourceNames), "\n---")

				//for _, obj := range resources {
				//	fmt.Printf("TypeMeta=%v ; Name=%s ; Namespace=%q\n", obj.GetObjectKind(), obj.GetName(), obj.GetNamespace())
				//}
			})
		*/

		AfterAll(func() {
			//snapshottedInfo.namespaces = namespaces
			//snapshottedInfo.crds = crds
			snapshottedInfo.resources = resources
		})

	})
}

// func outputToSliceOfStringWithoutWarnings(output string, separator string) []string {
// 	var lines []string
// 	for _, line := range strings.Split(strings.TrimSpace(strings.Trim(output, "'")), separator) {
// 		if line == "" || strings.HasPrefix(line, "Warning:") {
// 			continue
// 		}
// 		lines = append(lines, line)
// 	}
// 	return lines
// }

/*
###
namespacedResourceNames = [ciliumendpoints.cilium.io ciliumnetworkpolicies.cilium.io configmaps controllerrevisions.apps cronjobs.batch csistoragecapacities.storage.k8s.io daemonsets.apps deployments.apps endpoints endpointslices.discovery.k8s.io events events.events.k8s.io horizontalpodautoscalers.autoscaling ingresses.networking.k8s.io ingresses.extensions jobs.batch leases.coordination.k8s.io limitranges networkpolicies.networking.k8s.io nodepoollabelsets.labels.banzaicloud.io persistentvolumeclaims poddisruptionbudgets.policy pods podtemplates replicasets.apps replicationcontrollers resourcequotas rolebindings.rbac.authorization.k8s.io roles.rbac.authorization.k8s.io secrets serviceaccounts services statefulsets.apps] with len 33
---

###
clusterResourceNames = [apiservices.apiregistration.k8s.io certificatesigningrequests.certificates.k8s.io ciliumclusterwidenetworkpolicies.cilium.io ciliumexternalworkloads.cilium.io ciliumidentities.cilium.io ciliumnodes.cilium.io clusterrolebindings.rbac.authorization.k8s.io clusterroles.rbac.authorization.k8s.io componentstatuses csidrivers.storage.k8s.io csinodes.storage.k8s.io customresourcedefinitions.apiextensions.k8s.io flowschemas.flowcontrol.apiserver.k8s.io ingressclasses.networking.k8s.io mutatingwebhookconfigurations.admissionregistration.k8s.io namespaces nodes persistentvolumes podsecuritypolicies.policy priorityclasses.scheduling.k8s.io prioritylevelconfigurations.flowcontrol.apiserver.k8s.io runtimeclasses.node.k8s.io storageclasses.storage.k8s.io validatingwebhookconfigurations.admissionregistration.k8s.io volumeattachments.storage.k8s.io] with len 25
---

###
namespacedResourceNames = [configmaps controllerrevisions.apps cronjobs.batch csistoragecapacities.storage.k8s.io daemonsets.apps deployments.apps endpoints endpointslices.discovery.k8s.io events.events.k8s.io horizontalpodautoscalers.autoscaling ingresses.networking.k8s.io ingresses.extensions jobs.batch leases.coordination.k8s.io limitranges networkpolicies.networking.k8s.io nodepoollabelsets.labels.banzaicloud.io persistentvolumeclaims poddisruptionbudgets.policy pods podtemplates replicasets.apps replicationcontrollers resourcequotas rolebindings.rbac.authorization.k8s.io roles.rbac.authorization.k8s.io secrets serviceaccounts services statefulsets.apps] with len 30
---

###
clusterResourceNames = [apiservices.apiregistration.k8s.io certificatesigningrequests.certificates.k8s.io clusterrolebindings.rbac.authorization.k8s.io clusterroles.rbac.authorization.k8s.io componentstatuses csidrivers.storage.k8s.io csinodes.storage.k8s.io customresourcedefinitions.apiextensions.k8s.io flowschemas.flowcontrol.apiserver.k8s.io ingressclasses.networking.k8s.io mutatingwebhookconfigurations.admissionregistration.k8s.io namespaces persistentvolumes podsecuritypolicies.policy priorityclasses.scheduling.k8s.io prioritylevelconfigurations.flowcontrol.apiserver.k8s.io runtimeclasses.node.k8s.io storageclasses.storage.k8s.io validatingwebhookconfigurations.admissionregistration.k8s.io volumeattachments.storage.k8s.io] with len 20
*/

func pruneUnnecessaryClusterResourceNames(resourceNameList []string) []string {
	var updatedList []string
	for _, name := range resourceNameList {
		// On PKE clusters we could have nodes automatically added as load increases, for instace, because we install koperator components.
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

func printSnapshot(snapshottedInfo *clusterSnapshot) bool {
	return When("Debug info on accumulated variables", func() {
		It("Run some print statements", func() {
			//fmt.Println("###\nnamespaces =", snapshottedInfo.Namespaces(), "with len", len(snapshottedInfo.Namespaces()), "\n---")
			//fmt.Println("###\ncrds =", snapshottedInfo.CRDs(), "with len", len(snapshottedInfo.CRDs()), "\n---")
			for _, obj := range snapshottedInfo.Resources() {
				fmt.Printf("TypeMeta=%v ; Name=%s ; Namespace=%q\n", obj.GetObjectKind(), obj.GetName(), obj.GetNamespace())
			}
		})
	})
}

func snapshotClusterAndCompare(snapshottedInitialInfo *clusterSnapshot) bool {
	return When("Verifying cluster resources state", Ordered, func() {
		// Temporary When node
		When("Temporary: Create a difference in resources with an SA", func() {
			var kubectlOptions k8s.KubectlOptions
			var err error

			It("Acquiring K8s config and context", func() {
				kubectlOptions, err = kubectlOptionsForCurrentContext()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a test resource difference", func() {
				time.Sleep(1 * time.Second)
				var opc string = "create"
				var opd string = "delete"
				k8s.RunKubectlE(GinkgoT(), &kubectlOptions, []string{opc, "serviceaccount", "my-service-account"}...)
				k8s.RunKubectlE(GinkgoT(), &kubectlOptions, []string{opd, "namespace", "my-namespace"}...)
				time.Sleep(1 * time.Second)
			})
		})

		/*
			// This allows us to run the function without failure if an initial snapshot has not been collected beforehand.
			// In such a case, it does not make sense to run the logic of collecting and comparing snapshots.
			// ?????????????
			var inputSnapshotExists bool
			BeforeAll(func() {
				if snapshottedInitialInfo != nil && !snapshottedInitialInfo.IsEmpty() {
					inputSnapshotExists = true
				}
				//Expect(snapshottedInitialInfo.IsEmpty()).To(BeFalse())
			})
		*/

		var snapshottedCurrentInfo = &clusterSnapshot{}
		snapshotCluster(snapshottedCurrentInfo)
		//It("Print new cluster state info", func() {})
		//printSnapshot(snapshottedCurrentInfo)

		/*
			It("Checking namespaces list", func() {
				if !inputSnapshotExists { Skip("Input snapshot was not present") }
				Expect(snapshottedCurrentInfo.Namespaces()).To(ConsistOf(snapshottedInitialInfo.Namespaces()))
			})

			It("Checking CRDs list", func() {
				if !inputSnapshotExists { Skip("Input snapshot was not present") }
				Expect(snapshottedCurrentInfo.CRDs()).To(ConsistOf(snapshottedInitialInfo.CRDs()))
			})
		*/

		It("Checking resources list", func() {
			//if !inputSnapshotExists { Skip("Input snapshot was not present") }

			// Temporarily increase maximum output length (default 4000) to fit more objects in the printed diff.
			// Only doing this here because other assertions typically don't run against objects with this many elements.
			initialMaxLength := format.MaxLength
			defer func() { format.MaxLength = initialMaxLength }()
			format.MaxLength = 19000

			// Example: generated matchers directly for PartialObjectMetadata objects
			//Expect(snapshottedCurrentInfo.Resources()).To(ConsistOf(generateListOfMatchersForPartialObjectMetadataSlice(snapshottedInitialInfo.Resources())))
			// Example: default comparison directly on structs parsed into localComparison type
			Expect(snapshottedCurrentInfo.ResourcesAsComparisonType()).To(ConsistOf(snapshottedInitialInfo.ResourcesAsComparisonType()))
			// Example: custom comparison by implementing the GomegaMatcher interface and the GomegaStringer interface
			//Expect(snapshottedCurrentInfo.ResourcesAsComparisonType()).To(ConsistOf(snapshottedInitialInfo.ResourcesAsComparisonType()))

		})

		/*
			It("Check on format.MaxLength", func() {
				// check that default is 4000
				fmt.Println("format.MaxLength =", format.MaxLength)
			})
		*/
	})
}

/*
func beEqualPartialObjectMetadata(expected metav1.PartialObjectMetadata) gomegatypes.GomegaMatcher { // Find a format option for Matchers
	return And(
		WithTransform(func(e metav1.PartialObjectMetadata) schema.GroupVersionKind {
			return e.GroupVersionKind()
		}, Equal(expected.GroupVersionKind())),
		WithTransform(func(e metav1.PartialObjectMetadata) string {
			return e.GetNamespace()
		}, Equal(expected.GetNamespace())),
		WithTransform(func(e metav1.PartialObjectMetadata) string {
			return e.GetName()
		}, Equal(expected.GetName())),
	)
}

func generateListOfMatchersForPartialObjectMetadataSlice(expectedList []metav1.PartialObjectMetadata) []gomegatypes.GomegaMatcher {
	var matchers []gomegatypes.GomegaMatcher
	for _, object := range expectedList {
		matchers = append(matchers, beEqualPartialObjectMetadata(object))
	}
	return matchers
}
*/

type clusterSnapshot struct {
	//namespaces []string
	//crds       []string
	resources []metav1.PartialObjectMetadata
}

/*
	func (s *clusterSnapshot) Namespaces() []string {
		return s.namespaces
	}

	func (s *clusterSnapshot) CRDs() []string {
		return s.crds
	}
*/
func (s *clusterSnapshot) Resources() []metav1.PartialObjectMetadata {
	return s.resources
}

/*
func (s *clusterSnapshot) IsEmpty() bool {
	//return len(s.namespaces) == 0 && len(s.crds) == 0 && len(s.resources) == 0
	return len(s.resources) == 0
}
*/

// Alternative to matchers
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

type localComparisonPartialObjectMetadataType struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

/*
func (locCompObj localComparisonPartialObjectMetadataType) Match(actual interface{}) (success bool, err error) {
	// TODO: assert for correct type on input
	// maybe with a type switch I can do a more detailed error message
	actualCompObj, ok := actual.(localComparisonPartialObjectMetadataType)
	if !ok {
		return false, fmt.Errorf("error type asserting to local comparison object type")
	}

	// Do match work
	if reflect.DeepEqual(actualCompObj.GVK, locCompObj.GVK) &&
		actualCompObj.Namespace == locCompObj.Namespace &&
		actualCompObj.Name == locCompObj.Name {
		return true, nil
	}

	return false, nil
}

func (locCompObj localComparisonPartialObjectMetadataType) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%s\nto be equal to \n\t%s", format.Object(actual, 2), format.Object(locCompObj, 2))
}
func (locCompObj localComparisonPartialObjectMetadataType) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to be equal to \n\t%#v", format.Object(actual, 2), format.Object(locCompObj, 2))
}

func (locCompObj localComparisonPartialObjectMetadataType) GomegaString() string {
	return format.Object(locCompObj, 2)
}
*/

/*
// Trying a Gomega custom formatter
func MyCustomFormatter(value interface{}) (string, bool) {
	switch v := value.(type) {
	case metav1.PartialObjectMetadata:
		return fmt.Sprintf("GVK=%q;Name=%q,Namespace=%q\n", v.GroupVersionKind(), v.GetName(), v.GetNamespace()), true
	case []metav1.PartialObjectMetadata:
		var a []string
		for _, obj := range v {
			a = append(a, fmt.Sprintf("GVK=%q;Name=%q,Namespace=%q\n", obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace()))
		}
		return strings.Join(a, "\n"), true
	case gomegatypes.GomegaMatcher:
		return "Some GomegaMatcher", true
	}
	return "", false
}
*/

/*
var _ = When("Testing e2e test altogether", Ordered, func() {
	var snapshottedInfo = &clusterSnapshot{}
	testMihai_1(snapshottedInfo)
	//printSnapshot(snapshottedInfo)
	testInstall()
	//testInstallZookeeperCluster()
	//testInstallKafkaCluster("../../config/samples/simplekafkacluster.yaml")
	//testUninstallKafkaCluster()
	//testInstallKafkaCluster("../../config/samples/simplekafkacluster_ssl.yaml")
	//testUninstallKafkaCluster()
	//testUninstallZookeeperCluster()
	testUninstall()
	testMihai_2(snapshottedInfo)
})
*/

/*
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=ciliumclusterwidenetworkpolicies.cilium.io ; Namespace="" ; ResourceVersion=607
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=ciliumendpoints.cilium.io ; Namespace="" ; ResourceVersion=600
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=ciliumexternalworkloads.cilium.io ; Namespace="" ; ResourceVersion=593
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=ciliumidentities.cilium.io ; Namespace="" ; ResourceVersion=596
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=ciliumnetworkpolicies.cilium.io ; Namespace="" ; ResourceVersion=605
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=ciliumnodes.cilium.io ; Namespace="" ; ResourceVersion=602
TypeMeta=&TypeMeta{Kind:CustomResourceDefinition,APIVersion:apiextensions.k8s.io/v1,} ; Name=nodepoollabelsets.labels.banzaicloud.io ; Namespace="" ; ResourceVersion=1101

TypeMeta=&TypeMeta{Kind:Namespace,APIVersion:v1,} ; Name=default ; Namespace="" ; ResourceVersion=206
TypeMeta=&TypeMeta{Kind:Namespace,APIVersion:v1,} ; Name=kube-node-lease ; Namespace="" ; ResourceVersion=58
TypeMeta=&TypeMeta{Kind:Namespace,APIVersion:v1,} ; Name=kube-public ; Namespace="" ; ResourceVersion=40
TypeMeta=&TypeMeta{Kind:Namespace,APIVersion:v1,} ; Name=kube-system ; Namespace="" ; ResourceVersion=1088
TypeMeta=&TypeMeta{Kind:Namespace,APIVersion:v1,} ; Name=pipeline-system ; Namespace="" ; ResourceVersion=1082
*/

/*
  the extra elements were
      <[]e2e.localComparisonPartialObjectMetadataType | len:2, cap:2>: [
          {
              GVK: {Group: "", Version: "v1", Kind: "Secret"},
              Namespace: "default",
              Name: "my-service-account-token-x7csv",
          },
          {
              GVK: {
                  Group: "",
                  Version: "v1",
                  Kind: "ServiceAccount",
              },
              Namespace: "default",
              Name: "my-service-account",
          },
      ]
*/

/*
  the missing elements were
      <[]*matchers.AndMatcher | len:2, cap:2>: [
          {
              Matchers: [
                  <*matchers.WithTransformMatcher | 0x140005c8c80>{
                      Transform: <func(v1.PartialObjectMetadata) schema.GroupVersionKind>0x10138d9e0,
                      Matcher: <*matchers.EqualMatcher | 0x140005bee90>{
                          Expected: <schema.GroupVersionKind>{Group: "", Version: "v1", Kind: "Secret"},
                      },
                      transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                      transformedValue: <schema.GroupVersionKind>{Group: "", Version: "v1", Kind: "Service"},
                  },
                  <*matchers.WithTransformMatcher | 0x140005c8cc0>{
                      Transform: <func(v1.PartialObjectMetadata) string>0x10138da80,
                      Matcher: <*matchers.EqualMatcher | 0x140005beeb0>{
                          Expected: <string>"default",
                      },
                      transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                      transformedValue: <string>"default",
                  },
                  <*matchers.WithTransformMatcher | 0x140005c8d00>{
                      Transform: <func(v1.PartialObjectMetadata) string>0x10138da90,
                      Matcher: <*matchers.EqualMatcher | 0x140005beed0>{
                          Expected: <string>"my-service-account-token-x7csv",
                      },
                      transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                      transformedValue: <string>"default-token-7gq4d",
                  },
              ],
              firstFailedMatcher: <*matchers.WithTransformMatcher | 0x140005c8c80>{
                  Transform: <func(v1.PartialObjectMetadata) schema.GroupVersionKind>0x10138d9e0,
                  Matcher: <*matchers.EqualMatcher | 0x140005bee90>{
                      Expected: <schema.GroupVersionKind>{Group: "", Version: "v1", Kind: "Secret"},
                  },
                  transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                  transformedValue: <schema.GroupVersionKind>{Group: "", Version: "v1", Kind: "Service"},
              },
          },
          {
              Matchers: [
                  <*matchers.WithTransformMatcher | 0x140005c8e00>{
                      Transform: <func(v1.PartialObjectMetadata) schema.GroupVersionKind>0x10138d9e0,
                      Matcher: <*matchers.EqualMatcher | 0x140005bef30>{
                          Expected: <schema.GroupVersionKind>{
                              Group: "",
                              Version: "v1",
                              Kind: "ServiceAccount",
                          },
                      },
                      transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                      transformedValue: <schema.GroupVersionKind>{Group: "", Version: "v1", Kind: "Service"},
                  },
                  <*matchers.WithTransformMatcher | 0x140005c8e40>{
                      Transform: <func(v1.PartialObjectMetadata) string>0x10138da80,
                      Matcher: <*matchers.EqualMatcher | 0x140005bef50>{
                          Expected: <string>"default",
                      },
                      transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                      transformedValue: <string>"default",
                  },
                  <*matchers.WithTransformMatcher | 0x140005c8e80>{
                      Transform: <func(v1.PartialObjectMetadata) string>0x10138da90,
                      Matcher: <*matchers.EqualMatcher | 0x140005bef70>{
                          Expected: <string>"my-service-account",
                      },
                      transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                      transformedValue: <string>"default",
                  },
              ],
              firstFailedMatcher: <*matchers.WithTransformMatcher | 0x140005c8e00>{
                  Transform: <func(v1.PartialObjectMetadata) schema.GroupVersionKind>0x10138d9e0,
                  Matcher: <*matchers.EqualMatcher | 0x140005bef30>{
                      Expected: <schema.GroupVersionKind>{
                          Group: "",
                          Version: "v1",
                          Kind: "ServiceAccount",
                      },
                  },
                  transformArgType: <*reflect.rtype | 0x101a2ad00>{size: 0x108, ptrdata: 0xf8, hash: 2118212376, tflag: 7, align: 8, fieldAlign: 8, kind: 25, equal: nil, gcdata: 85, str: 289763, ptrToThis: 3960544},
                  transformedValue: <schema.GroupVersionKind>{Group: "", Version: "v1", Kind: "Service"},
              },
          },
      ]
*/

/*
// Example output with diff both ways:

  the missing elements were
      <[]e2e.localComparisonPartialObjectMetadataType | len:1, cap:1>: [
          {
              GVK: {Group: "", Version: "v1", Kind: "Namespace"},
              Namespace: "",
              Name: "my-namespace",
          },
      ]
  the extra elements were
      <[]e2e.localComparisonPartialObjectMetadataType | len:2, cap:2>: [
          {
              GVK: {Group: "", Version: "v1", Kind: "Secret"},
              Namespace: "default",
              Name: "my-service-account-token-8wpls",
          },
          {
              GVK: {
                  Group: "",
                  Version: "v1",
                  Kind: "ServiceAccount",
              },
              Namespace: "default",
              Name: "my-service-account",
          },
      ]

// Duration for 2 snapshots and comparison on an empty k8s cluster: 17 seconds.

// Duration for 2 snapshots and comparison plus testInstall() and testUninstall() starting from an empty k8s cluster: 300 seconds (290-330).

// Duration for 2 snapshots and comparison plus testInstall() + testInstallZookeeperCluster() + testInstallKafkaCluster(no-ssl) and testUninstallKafkaCluster() + testUninstallZookeeperCluster() + testUninstall() starting from an empty k8s cluster: 700 seconds.

*/

/*
|> % kubectl get persistentvolumes --watch
<initially nothing>
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS    CLAIM                               STORAGECLASS   REASON   AGE
pvc-6ea02177-b60f-426f-82e0-9c1e35e58bdf   20Gi       RWO            Delete           Pending   zookeeper/data-zookeeper-server-0   gp2                     0s
pvc-6ea02177-b60f-426f-82e0-9c1e35e58bdf   20Gi       RWO            Delete           Bound     zookeeper/data-zookeeper-server-0   gp2                     0s
pvc-e211b21b-f181-4934-95b6-4172377afed9   10Gi       RWO            Delete           Pending   kafka/kafka-0-storage-0-8f4k2       gp2                     0s
pvc-e211b21b-f181-4934-95b6-4172377afed9   10Gi       RWO            Delete           Bound     kafka/kafka-0-storage-0-8f4k2       gp2                     0s
pvc-9af9f0a5-c222-4e90-808f-f202ceb7f798   10Gi       RWO            Delete           Pending   kafka/kafka-1-storage-0-xttr9       gp2                     0s
pvc-9af9f0a5-c222-4e90-808f-f202ceb7f798   10Gi       RWO            Delete           Bound     kafka/kafka-1-storage-0-xttr9       gp2                     0s
pvc-c3a3b9c4-3f9b-4ef7-a8b8-e70659663cec   10Gi       RWO            Delete           Pending   kafka/kafka-2-storage-0-nq5pb       gp2                     0s
pvc-c3a3b9c4-3f9b-4ef7-a8b8-e70659663cec   10Gi       RWO            Delete           Bound     kafka/kafka-2-storage-0-nq5pb       gp2                     0s
pvc-c3a3b9c4-3f9b-4ef7-a8b8-e70659663cec   10Gi       RWO            Delete           Released   kafka/kafka-2-storage-0-nq5pb       gp2                     3m23s
pvc-9af9f0a5-c222-4e90-808f-f202ceb7f798   10Gi       RWO            Delete           Released   kafka/kafka-1-storage-0-xttr9       gp2                     4m
pvc-e211b21b-f181-4934-95b6-4172377afed9   10Gi       RWO            Delete           Released   kafka/kafka-0-storage-0-8f4k2       gp2                     4m4s
pvc-c3a3b9c4-3f9b-4ef7-a8b8-e70659663cec   10Gi       RWO            Delete           Terminating   kafka/kafka-2-storage-0-nq5pb       gp2                     3m42s
pvc-c3a3b9c4-3f9b-4ef7-a8b8-e70659663cec   10Gi       RWO            Delete           Terminating   kafka/kafka-2-storage-0-nq5pb       gp2                     3m43s
pvc-9af9f0a5-c222-4e90-808f-f202ceb7f798   10Gi       RWO            Delete           Terminating   kafka/kafka-1-storage-0-xttr9       gp2                     4m10s
pvc-9af9f0a5-c222-4e90-808f-f202ceb7f798   10Gi       RWO            Delete           Terminating   kafka/kafka-1-storage-0-xttr9       gp2                     4m10s
pvc-e211b21b-f181-4934-95b6-4172377afed9   10Gi       RWO            Delete           Terminating   kafka/kafka-0-storage-0-8f4k2       gp2                     4m11s
pvc-e211b21b-f181-4934-95b6-4172377afed9   10Gi       RWO            Delete           Terminating   kafka/kafka-0-storage-0-8f4k2       gp2                     4m11s
pvc-6ea02177-b60f-426f-82e0-9c1e35e58bdf   20Gi       RWO            Delete           Released      zookeeper/data-zookeeper-server-0   gp2                     5m24s
pvc-6ea02177-b60f-426f-82e0-9c1e35e58bdf   20Gi       RWO            Delete           Terminating   zookeeper/data-zookeeper-server-0   gp2                     5m25s
pvc-6ea02177-b60f-426f-82e0-9c1e35e58bdf   20Gi       RWO            Delete           Terminating   zookeeper/data-zookeeper-server-0   gp2                     5m25s

*/

/*
// CSRs for dynamically added nodes during e2e test runs

|> % kubectl get csr --watch

NAME        AGE   SIGNERNAME                                    REQUESTOR                 CONDITION
csr-94jnb   0s    kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:73cy2r   Pending
csr-94jnb   0s    kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:73cy2r   Approved
csr-94jnb   0s    kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:73cy2r   Approved,Issued
csr-xmz7v   0s    kubernetes.io/kubelet-serving                 system:node:ip-192-168-8-27.eu-central-1.compute.internal   Pending
csr-xmz7v   0s    kubernetes.io/kubelet-serving                 system:node:ip-192-168-8-27.eu-central-1.compute.internal   Approved
csr-xmz7v   0s    kubernetes.io/kubelet-serving                 system:node:ip-192-168-8-27.eu-central-1.compute.internal   Approved,Issued

*/
