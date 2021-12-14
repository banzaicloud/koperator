// Copyright © 2021 Banzai Cloud
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

package k8scsrpki

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	certsigningreqv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/istio-client-go/pkg/networking/v1alpha3"
	banzaiistiov1beta1 "github.com/banzaicloud/istio-operator/pkg/apis/istio/v1beta1"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/util/cert"
)

const (
	testNamespace = "test-namespace-csr"
	testDns       = "example.com"
)

func createKafkaUser() *v1alpha1.KafkaUser {
	return &v1alpha1.KafkaUser{
		TypeMeta: metav1.TypeMeta{
			Kind: "KafkaUser", // it is not populated by default and required for the tests
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: testNamespace,
		},
		Spec: v1alpha1.KafkaUserSpec{
			SecretName: "test-secret",
			PKIBackendSpec: &v1alpha1.PKIBackendSpec{
				PKIBackend: string(v1beta1.PKIBackendK8sCSR),
				SignerName: "foo.bar/foobar",
			},
			DNSNames: []string{testDns},
		},
	}
}

func setupSchemeForTests() (*runtime.Scheme, error) {
	sch := runtime.NewScheme()
	err := scheme.AddToScheme(sch)
	if err != nil {
		return nil, err
	}
	err = v1alpha1.AddToScheme(sch)
	if err != nil {
		return nil, err
	}
	err = v1beta1.AddToScheme(sch)
	if err != nil {
		return nil, err
	}
	err = banzaiistiov1beta1.AddToScheme(sch)
	if err != nil {
		return nil, err
	}
	err = v1alpha3.AddToScheme(sch)
	if err != nil {
		return nil, err
	}
	return sch, nil
}

func TestReconcileUserCertificate(t *testing.T) {
	sch, err := setupSchemeForTests()
	if err != nil {
		t.Fatal("failed to setup scheme", err)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(sch).Build()
	pkiManager := New(fakeClient, newMockCluster(), log.Log)
	ctx := context.Background()
	user := createKafkaUser()
	_, err = pkiManager.ReconcileUserCertificate(ctx, user, sch, "")
	if err != nil && err.Error() != fmt.Sprintf("%s: %s", notFoundApprovedCsrErrMsg, notApprovedErrMsg) {
		t.Fatal("failed to reconcile user certificate", err)
	}

	var requestList certsigningreqv1.CertificateSigningRequestList
	err = fakeClient.List(ctx, &requestList)
	if err != nil {
		t.Fatal("failed to get reconciled CertificateSigningRequest")
	}
	if len(requestList.Items) == 0 {
		t.Fatal("could not find CertificateSigningRequest in cluster")
	}
	if len(requestList.Items) > 1 {
		t.Fatal("multiple CertificateSigningRequests found in cluster")
	}
	csr := requestList.Items[0]
	req := csr.Spec.Request
	block, _ := pem.Decode(req)
	if block == nil {
		t.Fatal("found empty block")
	}
	if block.Type != cert.CertRequestType {
		t.Error("type of block mismatch, found:", block.Type)
	}
	certReq, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal("could not parse cert request", err)
	}
	if certReq.Subject.CommonName != user.GetName() {
		t.Error("wrong common name:", certReq.Subject.CommonName)
	}
	if len(certReq.DNSNames) != 1 && certReq.DNSNames[0] != testDns {
		t.Error("wrong DNS names:", certReq.DNSNames)
	}
}
