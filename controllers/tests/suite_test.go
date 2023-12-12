// Copyright © 2020 Cisco Systems, Inc. and/or its affiliates
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

package tests

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	csrclient "k8s.io/client-go/kubernetes/typed/certificates/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	istioclientv1beta1 "github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"
	banzaiistiov1alpha1 "github.com/banzaicloud/istio-operator/api/v2/v1alpha1"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/controllers"
	"github.com/banzaicloud/koperator/pkg/jmxextractor"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/scale"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var csrClient *csrclient.CertificatesV1Client
var testEnv *envtest.Environment
var mockKafkaClients map[types.NamespacedName]kafkaclient.KafkaClient
var cruiseControlOperationReconciler controllers.CruiseControlOperationReconciler
var kafkaClusterCCReconciler controllers.CruiseControlTaskReconciler

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {

	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	timeout := 2 * time.Minute
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "base", "crds"),
			filepath.Join("..", "..", "config", "test", "crd", "cert-manager"),
			filepath.Join("..", "..", "config", "test", "crd", "istio"),
		},
		ControlPlaneStartTimeout: timeout,
		ControlPlaneStopTimeout:  timeout,
		AttachControlPlaneOutput: false,
	}

	var cfg *rest.Config
	var err error
	done := make(chan interface{})
	go func() {
		defer GinkgoRecover()
		cfg, err = testEnv.Start()
		close(done)
	}()
	Eventually(done).WithContext(ctx).WithTimeout(timeout).Should(BeClosed())
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()

	Expect(banzaiistiov1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(k8sscheme.AddToScheme(scheme)).To(Succeed())
	Expect(apiv1.AddToScheme(scheme)).To(Succeed())
	Expect(cmv1.AddToScheme(scheme)).To(Succeed())
	Expect(banzaicloudv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(banzaicloudv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(istioclientv1beta1.AddToScheme(scheme)).To(Succeed())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	csrClient, err = csrclient.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(csrClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: "0",
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 8443,
		}),
		LeaderElection: false,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(mgr).ToNot(BeNil())

	jmxextractor.NewMockJMXExtractor()

	mockKafkaClients = make(map[types.NamespacedName]kafkaclient.KafkaClient)

	// mock the creation of Kafka clients
	controllers.SetNewKafkaFromCluster(
		func(k8sclient client.Client, cluster *banzaicloudv1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error) {
			client, closeFunc := getMockedKafkaClientForCluster(cluster)
			return client, closeFunc, nil
		})

	kafkaClusterReconciler := controllers.KafkaClusterReconciler{
		Client:              mgr.GetClient(),
		DirectClient:        mgr.GetAPIReader(),
		KafkaClientProvider: kafkaclient.NewMockProvider(),
	}

	err = controllers.SetupKafkaClusterWithManager(mgr).Complete(&kafkaClusterReconciler)
	Expect(err).NotTo(HaveOccurred())

	kafkaTopicReconciler := &controllers.KafkaTopicReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = controllers.SetupKafkaTopicWithManager(mgr, 10).Complete(kafkaTopicReconciler)
	Expect(err).NotTo(HaveOccurred())

	// Create a new  kafka user reconciler
	kafkaUserReconciler := controllers.KafkaUserReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = controllers.SetupKafkaUserWithManager(mgr, true, true).Complete(&kafkaUserReconciler)
	Expect(err).NotTo(HaveOccurred())

	kafkaClusterCCReconciler = controllers.CruiseControlTaskReconciler{
		Client:       mgr.GetClient(),
		DirectClient: mgr.GetAPIReader(),
		Scheme:       mgr.GetScheme(),
		ScaleFactory: func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
			return nil, errors.New("there is no scale mock")
		},
	}

	err = controllers.SetupCruiseControlWithManager(mgr).Complete(&kafkaClusterCCReconciler)
	Expect(err).NotTo(HaveOccurred())

	cruiseControlOperationReconciler = controllers.CruiseControlOperationReconciler{
		Client:       mgr.GetClient(),
		DirectClient: mgr.GetAPIReader(),
		Scheme:       mgr.GetScheme(),
		ScaleFactory: func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (scale.CruiseControlScaler, error) {
			return nil, errors.New("there is no scale mock")
		},
	}

	err = controllers.SetupCruiseControlOperationWithManager(mgr).Complete(&cruiseControlOperationReconciler)
	Expect(err).NotTo(HaveOccurred())

	cruiseControlOperationTTLReconciler := controllers.CruiseControlOperationTTLReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = controllers.SetupCruiseControlOperationTTLWithManager(mgr).Complete(&cruiseControlOperationTTLReconciler)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:builder
	go func() {
		defer GinkgoRecover()
		ctrl.Log.Info("starting manager")
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: mgr.GetScheme()})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	crd := &apiv1.CustomResourceDefinition{}

	err = k8sClient.Get(ctx, types.NamespacedName{Name: "kafkaclusters.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("KafkaCluster"))

	err = k8sClient.Get(ctx, types.NamespacedName{Name: "cruisecontroloperations.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("CruiseControlOperation"))

	err = k8sClient.Get(ctx, types.NamespacedName{Name: "kafkatopics.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("KafkaTopic"))

	err = k8sClient.Get(ctx, types.NamespacedName{Name: "kafkausers.kafka.banzaicloud.io"}, crd)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd.Spec.Names.Kind).To(Equal("KafkaUser"))
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
