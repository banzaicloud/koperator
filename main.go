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
	"flag"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/cache"

	istioclientv1alpha3 "github.com/banzaicloud/istio-client-go/pkg/networking/v1alpha3"
	banzaiistiov1beta1 "github.com/banzaicloud/istio-operator/pkg/apis/istio/v1beta1"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
	banzaicloudv1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/controllers"
	"github.com/banzaicloud/kafka-operator/pkg/webhook"
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

	_ = banzaiistiov1beta1.AddToScheme(scheme)

	_ = istioclientv1alpha3.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var namespaces string
	var metricsAddr string
	var enableLeaderElection bool
	var webhookCertDir string
	var webhookDisabled bool
	var verboseLogging bool

	flag.StringVar(&namespaces, "namespaces", "", "Comma separated list of namespaces where operator listens for resources")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&webhookDisabled, "disable-webhooks", false, "Disable webhooks used to validate custom resources")
	flag.StringVar(&webhookCertDir, "tls-cert-dir", "/etc/webhook/certs", "The directory with a tls.key and tls.crt for serving HTTPS requests")
	flag.BoolVar(&verboseLogging, "verbose", false, "Enable verbose logging")
	flag.Parse()

	ctrl.SetLogger(zap.Logger(verboseLogging))

	//When operator is started to watch resources in a specific set of namespaces, we use the MultiNamespacedCacheBuilder cache.
	//In this scenario, it is also suggested to restrict the provided authorization to this namespace by replacing the default
	//ClusterRole and ClusterRoleBinding to Role and RoleBinding respectively
	//For further information see the kubernetes documentation about
	//Using [RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).
	managerWatchCache := (cache.NewCacheFunc)(nil)
	if namespaces != "" {
		ns := strings.Split(namespaces, ",")
		for i := range ns {
			ns[i] = strings.TrimSpace(ns[i])
		}
		managerWatchCache = cache.MultiNamespacedCacheBuilder(ns)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "controller-leader-election-helper",
		NewCache:           managerWatchCache,
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
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("KafkaCluster"),
	}

	if err = controllers.SetupKafkaClusterWithManager(mgr, kafkaClusterReconciler.Log).Complete(kafkaClusterReconciler); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaCluster")
		os.Exit(1)
	}

	if err = controllers.SetupKafkaTopicWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaTopic")
		os.Exit(1)
	}

	if err = controllers.SetupKafkaUserWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaUser")
		os.Exit(1)
	}

	kafkaClusterCCReconciler := &controllers.CruiseControlTaskReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controller").WithName("CruiseControlTask"),
	}

	if err = controllers.SetupCruiseControlWithManager(mgr).Complete(kafkaClusterCCReconciler); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CruiseControl")
		os.Exit(1)
	}

	if !webhookDisabled {
		webhook.SetupServerHandlers(mgr, webhookCertDir)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
