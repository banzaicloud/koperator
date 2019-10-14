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
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newRawTopic() []byte {
	topic := &v1alpha1.KafkaTopic{}
	topic.ObjectMeta = metav1.ObjectMeta{
		Name:      "test-topic",
		Namespace: "test-namespace",
	}
	out, _ := json.Marshal(topic)
	return out
}

func newAdmissionReview() *admissionv1beta1.AdmissionReview {
	return &admissionv1beta1.AdmissionReview{
		Request: &admissionv1beta1.AdmissionRequest{
			Kind: metav1.GroupVersionKind{
				Kind: "non-topic-kind",
			},
			Namespace: "test-namespace",
			Name:      "test-resource",
			UID:       "test-uid",
			Operation: "CREATE",
			UserInfo:  authv1.UserInfo{},
		},
	}
}

func newRequest(data []byte) *http.Request {
	buf := new(bytes.Buffer)
	buf.Write(data)
	req, _ := http.NewRequest("POST", "/validate", buf)
	req.Header.Add("Content-Type", "application/json")
	return req
}

func TestValidate(t *testing.T) {
	server := newMockServer()

	req := newAdmissionReview()

	if res := server.validate(req); res.Allowed {
		t.Error("Expected denied request for unknown resource type, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonBadRequest {
		t.Error("Expected bad request, got:", res.Result.Reason)
	} else if !strings.Contains(res.Result.Message, "Unexpected resource kind") {
		t.Error("Expected unexpected resource kind message, got:", res.Result.Message)
	}

	req.Request.Kind.Kind = kafkaTopic

	if res := server.validate(req); res.Allowed {
		t.Error("Expected denied request for non-unmarshalable object, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonBadRequest {
		t.Error("Expected bad request, got:", res.Result.Reason)
	}

	req.Request.Object.Raw = newRawTopic()

	if res := server.validate(req); res.Allowed {
		t.Error("Expected not allowed, got allowed")
	} else if res.Result.Reason != metav1.StatusReasonNotFound {
		t.Error("Expected not found for no cluster, got:", res.Result.Reason)
	}
}

func TestServe(t *testing.T) {
	server := newMockServer()

	// Test bad body
	reader, writer := io.Pipe()
	// close both immediately to cause io error
	writer.Close()
	reader.Close()
	req, _ := http.NewRequest("POST", "/validate", reader)
	recorder := httptest.NewRecorder()
	server.serve(recorder, req)
	res := recorder.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Error("Expected unable to read request body, got:", res.Status)
	} else {
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		if !strings.Contains(string(body), "could not read request body") {
			t.Error("Expected unable to read request body, got:", string(body))
		}
	}

	// Test empty body
	reader, writer = io.Pipe()
	req, _ = http.NewRequest("POST", "/validate", reader)
	recorder = httptest.NewRecorder()

	// Close writer but leave reader open
	writer.Close()
	server.serve(recorder, req)
	reader.Close()

	res = recorder.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Error("Expected unable to read request body, got:", res.Status)
	} else {
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		if !strings.Contains(string(body), "empty body") {
			t.Error("Expected empty body, got:", string(body))
		}
	}

	// Test bad content-type
	buf := new(bytes.Buffer)
	buf.Write([]byte("some data"))
	req, _ = http.NewRequest("POST", "/validate", buf)
	recorder = httptest.NewRecorder()
	server.serve(recorder, req)
	res = recorder.Result()
	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Error("Expected unsupported media type, got:", res.StatusCode)
	}

	// test non-deserializeable review
	req = newRequest([]byte("some data"))
	recorder = httptest.NewRecorder()
	server.serve(recorder, req)
	res = recorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("Expected 200 with admission error, got:", res.StatusCode)
	} else {
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Error("Expected admission review response, got error")
		}
		admissionReview := admissionv1beta1.AdmissionReview{}
		json.Unmarshal(body, &admissionReview)
		if admissionReview.Response.Result.Reason != metav1.StatusReasonBadRequest {
			t.Error("Expected metav1 bad request, got:", admissionReview.Response.Result.Reason)
		}
	}

	// Test admission review
	review := newAdmissionReview()
	review.Request.Kind.Kind = kafkaTopic
	review.Request.Object.Raw = newRawTopic()
	out, _ := json.Marshal(review)
	req = newRequest(out)
	recorder = httptest.NewRecorder()
	server.serve(recorder, req)
	res = recorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("Expected successful admission review, got:", res.StatusCode)
	} else {
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Error("Expected admission review response, got error")
		}
		admissionReview := admissionv1beta1.AdmissionReview{}
		json.Unmarshal(body, &admissionReview)
		if admissionReview.Response.Result.Reason != metav1.StatusReasonNotFound {
			t.Error("Expected not found for no cluster, got:", admissionReview.Response.Result.Reason)
		}
	}
}
