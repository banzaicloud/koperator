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

/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkoconfig "github.com/onsi/ginkgo/config"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	ginkoconfig.DefaultReporterConfig.SlowSpecThreshold = 60

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "base", "crds")},
	}

	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		err := os.Setenv("KUBEBUILDER_ASSETS", filepath.Join("..", "bin", "kubebuilder", "bin"))
		Expect(err).ToNot(HaveOccurred())
	}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	scheme := runtime.NewScheme()

	err = k8sscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	/*err = cmv1alpha2.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = cmv1alpha3.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = apiextensionsv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())*/
	err = banzaicloudv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = banzaicloudv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		Port:               8443,
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(mgr).ToNot(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: mgr.GetScheme()})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	kafkaClusterReconciler := KafkaClusterReconciler{
		Client:       mgr.GetClient(),
		DirectClient: mgr.GetAPIReader(),
		Log:          ctrl.Log.WithName("controllers").WithName("KafkaCluster"),
		Scheme:       mgr.GetScheme(),
	}

	err = SetupKafkaClusterWithManager(mgr, kafkaClusterReconciler.Log).Complete(&kafkaClusterReconciler)
	Expect(err).NotTo(HaveOccurred())

	err = SetupKafkaTopicWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// TODO parameterize this
	certManagerEnabled := true
	err = SetupKafkaUserWithManager(mgr, certManagerEnabled)
	Expect(err).NotTo(HaveOccurred())

	kafkaClusterCCReconciler := &CruiseControlTaskReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controller").WithName("CruiseControlTask"),
	}

	err = SetupCruiseControlWithManager(mgr).Complete(kafkaClusterCCReconciler)
	Expect(err).NotTo(HaveOccurred())

	// TODO enable webhook
	// webhookCertDir := ""
	// webhook.SetupServerHandlers(mgr, webhookCertDir)

	// +kubebuilder:scaffold:builder

	//go func() {
		ctrl.Log.Info("starting manager")
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
		//defer GinkgoRecover()
	//}()

	crd := &apiextensionsv1beta1.CustomResourceDefinition{}

	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: "kafkaclusters.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("KafkaCluster"))

	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: "kafkatopics.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("KafkaTopic"))

	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: "kafkausers.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("KafkaUser"))

	close(done)
}, 600)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
