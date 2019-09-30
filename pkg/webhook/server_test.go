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

package webhook

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newMockServer() *webhookServer {
	certv1.AddToScheme(scheme.Scheme)
	v1alpha1.AddToScheme(scheme.Scheme)
	v1beta1.AddToScheme(scheme.Scheme)
	return &webhookServer{
		client:              fake.NewFakeClientWithScheme(scheme.Scheme),
		scheme:              scheme.Scheme,
		deserializer:        serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer(),
		newKafkaFromCluster: kafkaclient.NewMockFromCluster,
	}
}

func TestNewServer(t *testing.T) {
	server := newWebHookServer(fake.NewFakeClient(), scheme.Scheme)
	if reflect.ValueOf(server.newKafkaFromCluster).Pointer() != reflect.ValueOf(kafkaclient.NewFromCluster).Pointer() {
		t.Error("Expected newKafkaFromCluster ptr -> kafkaclient.NewFromCluster")
	}
}

func TestNewServerMux(t *testing.T) {
	mux := newWebhookServerMux(fake.NewFakeClient(), scheme.Scheme)
	var buf bytes.Buffer
	req, _ := http.NewRequest("POST", "/validate", &buf)
	if _, pattern := mux.Handler(req); pattern == "" {
		t.Error("Expected mux to handle /validate, got 404")
	}
}
