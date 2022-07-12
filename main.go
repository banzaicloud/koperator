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

package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/cache"

	istioclientv1beta1 "github.com/banzaicloud/istio-client-go/pkg/networking/v1beta1"

	banzaiistiov1alpha1 "github.com/banzaicloud/istio-operator/api/v2/v1alpha1"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"

	banzaicloudv1alpha1 "github.com/banzaicloud/koperator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/controllers"
	"github.com/banzaicloud/koperator/pkg/k8sutil"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
	"github.com/banzaicloud/koperator/pkg/util"
	"github.com/banzaicloud/koperator/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = banzaicloudv1alpha1.AddToScheme(scheme)

	_ = banzaicloudv1beta1.AddToScheme(scheme)

	_ = banzaiistiov1alpha1.AddToScheme(scheme)

	_ = istioclientv1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		podNamespace                      string
		podServiceAccount                 string
		namespaces                        string
		metricsAddr                       string
		enableLeaderElection              bool
		webhookCertDir                    string
		webhookDisabled                   bool
		webhookServerPort                 int
		developmentLogging                bool
		verboseLogging                    bool
		certSigningDisabled               bool
		certManagerEnabled                bool
		maxKafkaTopicConcurrentReconciles int
	)

	flag.StringVar(&namespaces, "namespaces", "", "Comma separated list of namespaces where operator listens for resources")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&webhookDisabled, "disable-webhooks", false, "Disable webhooks used to validate custom resources")
	flag.StringVar(&webhookCertDir, "tls-cert-dir", "/etc/webhook/certs", "The directory with a tls.key and tls.crt for serving HTTPS requests")
	flag.IntVar(&webhookServerPort, "webhook-server-port", 443, "The port that the webhook server serves at")
	flag.BoolVar(&developmentLogging, "development", false, "Enable development logging")
	flag.BoolVar(&verboseLogging, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&certManagerEnabled, "cert-manager-enabled", false, "Enable cert-manager integration")
	flag.BoolVar(&certSigningDisabled, "disable-cert-signing-support", false, "Disable native certificate signing integration")
	flag.IntVar(&maxKafkaTopicConcurrentReconciles, "max-kafka-topic-concurrent-reconciles", 10, "Define max amount of concurrent KafkaTopic reconciles")
	flag.Parse()

	podNamespace = os.Getenv("POD_NAMESPACE")
	podServiceAccount = os.Getenv("SERVICE_ACCOUNT")

	ctrl.SetLogger(util.CreateLogger(verboseLogging, developmentLogging))

	// adding indexers to KafkaTopics so that the KafkaTopic admission webhooks could work
	ctx := context.Background()
	var managerWatchCacheBuilder cache.NewCacheFunc

	// When operator is started to watch resources in a specific set of namespaces, we use the MultiNamespacedCacheBuilder cache.
	// In this scenario, it is also suggested to restrict the provided authorization to this namespace by replacing the default
	// ClusterRole and ClusterRoleBinding to Role and RoleBinding respectively
	// For further information see the kubernetes documentation about
	// Using [RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).
	var namespaceList []string
	if namespaces != "" {
		namespaceList = strings.Split(namespaces, ",")
		for i := range namespaceList {
			namespaceList[i] = strings.TrimSpace(namespaceList[i])
		}
		managerWatchCacheBuilder = cache.MultiNamespacedCacheBuilder(namespaceList)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "controller-leader-election-helper",
		NewCache:           managerWatchCacheBuilder,
		Port:               webhookServerPort,
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := certv1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err = controllers.SetAlertManagerWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AlertManagerForKafka")
		os.Exit(1)
	}

	kafkaClusterReconciler := &controllers.KafkaClusterReconciler{
		Client:              mgr.GetClient(),
		DirectClient:        mgr.GetAPIReader(),
		Namespaces:          namespaceList,
		KafkaClientProvider: kafkaclient.NewDefaultProvider(),
	}

	if err = controllers.SetupKafkaClusterWithManager(mgr).Complete(kafkaClusterReconciler); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaCluster")
		os.Exit(1)
	}

	kafkaTopicReconciler := &controllers.KafkaTopicReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = controllers.SetupKafkaTopicWithManager(mgr, maxKafkaTopicConcurrentReconciles).Complete(kafkaTopicReconciler); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaTopic")
		os.Exit(1)
	}

	// Create a new  kafka user reconciler
	kafkaUserReconciler := &controllers.KafkaUserReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = controllers.SetupKafkaUserWithManager(mgr, !certSigningDisabled, certManagerEnabled).Complete(kafkaUserReconciler); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaUser")
		os.Exit(1)
	}

	kafkaClusterCCReconciler := &controllers.CruiseControlTaskReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = controllers.SetupCruiseControlWithManager(mgr).Complete(kafkaClusterCCReconciler); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CruiseControl")
		os.Exit(1)
	}

	if !webhookDisabled {
		webhook.SetupServerHandlers(mgr, webhookCertDir, podNamespace, podServiceAccount)
	}

	// +kubebuilder:scaffold:builder

	if err := k8sutil.AddKafkaTopicIndexers(ctx, mgr.GetCache()); err != nil {
		setupLog.Error(err, "unable to add indexers to manager's cache")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
