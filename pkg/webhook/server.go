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
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/banzaicloud/koperator/api/v1beta1"
	"github.com/banzaicloud/koperator/pkg/kafkaclient"
)

var (
	log = logf.Log.WithName("webhooks")
)

type webhookServer struct {
	client            client.Client
	scheme            *runtime.Scheme
	deserializer      runtime.Decoder
	podNamespace      string
	podServiceAccount string

	// For mocking - use kafkaclient.NewMockFromCluster
	newKafkaFromCluster func(client.Client, *v1beta1.KafkaCluster) (kafkaclient.KafkaClient, func(), error)
}

func newWebHookServer(client client.Client, scheme *runtime.Scheme, podNamespace, podServiceAccount string) *webhookServer {
	return &webhookServer{
		client:              client,
		scheme:              scheme,
		deserializer:        serializer.NewCodecFactory(scheme).UniversalDeserializer(),
		podNamespace:        podNamespace,
		podServiceAccount:   podServiceAccount,
		newKafkaFromCluster: kafkaclient.NewFromCluster,
	}
}

func newWebhookServerMux(client client.Client, scheme *runtime.Scheme, podNamespace, podServiceAccount string) *http.ServeMux {
	mux := http.NewServeMux()
	webhookServer := newWebHookServer(client, scheme, podNamespace, podServiceAccount)
	mux.HandleFunc("/validate", webhookServer.serve)
	return mux
}

// SetupServerHandlers sets up a webhook with the manager
func SetupServerHandlers(mgr ctrl.Manager, certDir, podNamespace, podServiceAccount string) {
	server := mgr.GetWebhookServer()
	server.CertDir = certDir
	mux := newWebhookServerMux(mgr.GetClient(), mgr.GetScheme(), podNamespace, podServiceAccount)
	server.Register("/validate", mux)
}
