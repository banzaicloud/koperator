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

package kafkaclient

import (
	"crypto/tls"
	"testing"
)

func TestNew(t *testing.T) {
	opts := newMockOpts()
	if client := New(opts); client == nil {
		t.Error("Expected new client, got nil")
	}
}

func TestOpen(t *testing.T) {
	client := newMockClient()
	if err := client.Open(); err != nil {
		t.Error("Expected to open mock client, got error:", err)
	}
	client.newClusterAdmin = newMockClusterAdminError
	if err := client.Open(); err == nil {
		t.Error("Expected error opening bad client, got nil")
	}
	client.newClusterAdmin = newMockClusterAdminFailOps
	if err := client.Open(); err == nil {
		t.Error("Expected error describing cluster, got nil")
	}
}

func TestClose(t *testing.T) {
	client := newMockClient()
	if err := client.Open(); err != nil {
		t.Error("Expected to open mock client, got error:", err)
	}
	if err := client.Close(); err != nil {
		t.Error("Expected no error, got:", err)
	}
}

func TestNumBrokers(t *testing.T) {
	client := newMockClient()
	if client.NumBrokers() != 0 {
		t.Error("Expected client with no brokers, got:", client.NumBrokers())
	}
}

func TestGetSaramaConfig(t *testing.T) {
	client := newMockClient()
	client.opts.UseSSL = true
	client.opts.TLSConfig = &tls.Config{}
	conf := client.getSaramaConfig()
	if conf.Net.TLS.Enable != true {
		t.Error("Expected sarama config with TLS enabled, got false")
	}
}
