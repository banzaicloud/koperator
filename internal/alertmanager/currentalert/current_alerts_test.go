// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

package currentalert

import (
	"context"
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var c client.Client
var cfg *rest.Config

const (
	firingAlertStatus   = "firing"
	resolvedAlertStatus = "resolved"
)

func TestMain(m *testing.M) {
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "base", "crds")},
	}
	if err := v1beta1.AddToScheme(scheme.Scheme); err != nil {
		stdlog.Fatal(err)
	}

	var err error
	if cfg, err = t.Start(); err != nil {
		stdlog.Fatal(err)
	}

	code := m.Run()
	if err := t.Stop(); err != nil {
		stdlog.Fatal(err)
	}
	os.Exit(code)
}

// StartTestManager adds recFn
func StartTestManager(ctx context.Context, mgr manager.Manager, g *gomega.GomegaWithT) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		g.Expect(mgr.Start(ctx)).NotTo(gomega.HaveOccurred())
		wg.Done()
	}()
	return wg
}

// nolint:unparam
func ensureCreated(t *testing.T, object runtime.Object, mgr manager.Manager) func() {
	err := mgr.GetClient().Create(context.TODO(), object.(client.Object))
	if err != nil {
		t.Fatalf("%+v", err)
	}
	return func() {
		if err := mgr.GetClient().Delete(context.TODO(), object.(client.Object)); err != nil {
			t.Error("Expected no error, got:", err)
		}
	}
}

func updateStatus(t *testing.T, object *v1beta1.KafkaCluster, mgr manager.Manager) {
	object.Status = v1beta1.KafkaClusterStatus{
		State: v1beta1.KafkaClusterRunning,
		RollingUpgrade: v1beta1.RollingUpgradeStatus{
			LastSuccess: "00000-00000",
			ErrorCount:  0,
		},
	}
	err := mgr.GetClient().Status().Update(context.TODO(), object)
	if err != nil {
		t.Fatalf("%+v", err)
	}
}

func TestGetCurrentAlerts(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	gomega.RegisterFailHandler(ginkgo.Fail)
	defer ginkgo.GinkgoRecover()

	log := logf.Log.WithName("alertmanager")

	mgr, err := manager.New(cfg, manager.Options{Scheme: scheme.Scheme})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()
	ctx, cancelFunc := context.WithCancel(context.Background())

	mgrStopped := StartTestManager(ctx, mgr, g)

	defer func() {
		cancelFunc()
		mgrStopped.Wait()
	}()

	kafkaNamespace := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: "kafka",
		},
	}

	kafkaCluster := &v1beta1.KafkaCluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      "kafka",
			Namespace: "kafka",
		},
		Spec: v1beta1.KafkaClusterSpec{
			HeadlessServiceEnabled: true,
			ListenersConfig: v1beta1.ListenersConfig{
				InternalListeners: []v1beta1.InternalListenerConfig{
					{CommonListenerSpec: v1beta1.CommonListenerSpec{
						Type:                            "plaintext",
						Name:                            "plaintext",
						ContainerPort:                   29092,
						UsedForInnerBrokerCommunication: true},
					},
				},
			},
			ZKAddresses: []string{},
			Brokers: []v1beta1.Broker{
				{
					Id: 1,
				},
			},
			OneBrokerPerNode: true,
			CruiseControlConfig: v1beta1.CruiseControlConfig{
				CruiseControlEndpoint: "kafka",
			},
		},
	}
	// Create Namespace first
	ensureCreated(t, kafkaNamespace, mgr)

	ensureCreated(t, kafkaCluster, mgr)

	// We have to update the status of the KafkaCluster CR separately since create drops changes to status field
	updateStatus(t, kafkaCluster, mgr)

	alerts1 := GetCurrentAlerts()
	if alerts1 == nil {
		t.Error("expected pointer to Singleton after calling GetCurrentAlerts(), not nil")
	}
	alerts1.IgnoreCCStatusCheck(true)

	singleAlerts := alerts1

	testAlert1 := AlertState{
		FingerPrint: model.Fingerprint(1111),
		Status:      model.AlertStatus(firingAlertStatus),
		Labels: model.LabelSet{
			"alertname":             "PodAlert",
			"test":                  "test",
			v1beta1.KafkaCRLabelKey: "kafka",
			"namespace":             "kafka",
		},
		Annotations: map[model.LabelName]model.LabelValue{
			"command": "testing",
		},
	}

	testAlert2 := AlertState{
		FingerPrint: model.Fingerprint(2222),
		Status:      model.AlertStatus(resolvedAlertStatus),
		Labels: model.LabelSet{
			"alertname": "PodAlert",
			"test":      "test",
		},
		Annotations: map[model.LabelName]model.LabelValue{
			"command": "testing",
		},
	}

	a1 := alerts1.AddAlert(testAlert1)
	if a1.Status != firingAlertStatus {
		t.Error("AdAlert failed a1")
	}

	list1 := alerts1.ListAlerts()
	if list1 == nil || list1[testAlert1.FingerPrint].Status != firingAlertStatus || list1[testAlert1.FingerPrint].Labels["alertname"] != "PodAlert" {
		t.Error("Listing alerts failed a1")
	}

	testRollingUpgradeErrorCount := 5
	currAlert, err := alerts1.HandleAlert(ctx, testAlert1.FingerPrint, c, testRollingUpgradeErrorCount, log)
	if err != nil {
		t.Error("Hanlde alert failed a1 with error", err)
	}
	t.Log(currAlert)

	if list1 == nil || list1[testAlert1.FingerPrint].Status != firingAlertStatus || list1[testAlert1.FingerPrint].Processed != true {
		t.Error("Process alert failed a1")
	}

	// verify rolling upgrade alert count in KafkaCluster status
	g.Eventually(func() (int, error) {
		var actualKafkaCluster v1beta1.KafkaCluster
		if err := c.Get(ctx, client.ObjectKeyFromObject(kafkaCluster), &actualKafkaCluster); err != nil {
			return -1, err
		}
		return actualKafkaCluster.Status.RollingUpgrade.ErrorCount, nil
	}, 10*time.Second, 1*time.Second).Should(
		gomega.Equal(testRollingUpgradeErrorCount),
		"rolling upgrade error count should match expected error count")

	alerts2 := GetCurrentAlerts()
	if alerts2 != singleAlerts {
		t.Error("Expected same instance in alerts2 but it got a different instance")
	}

	a2 := alerts2.AddAlert(testAlert2)
	if a2.Status != resolvedAlertStatus {
		t.Error("AdAlert failed a2")
	}

	list2 := alerts2.ListAlerts()
	if list2 == nil || list2[testAlert2.FingerPrint].Status != resolvedAlertStatus || list2[testAlert2.FingerPrint].Labels["alertname"] != "PodAlert" {
		t.Error("Listing alerts failed a2")
	}

	alerts3 := GetCurrentAlerts()
	if alerts3.AlertGC(testAlert2) != nil {
		t.Error("Unable to delete alert a2")
	}

	list3 := alerts3.ListAlerts()
	if list3 == nil || list3[testAlert2.FingerPrint] != nil {
		t.Error("2222 alert wasn't deleted")
	}

	_, err = alerts3.HandleAlert(ctx, model.Fingerprint(2222), c, 0, log)
	expected := "alert doesn't exist"
	if err == nil || err.Error() != expected {
		t.Error("alert with 2222 isn't the expected", err)
	}
}
