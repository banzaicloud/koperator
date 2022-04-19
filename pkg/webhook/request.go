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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/pkg/util"

	"github.com/banzaicloud/koperator/api/v1alpha1"
)

var (
	kafkaTopic = reflect.TypeOf(v1alpha1.KafkaTopic{}).Name()
)

func (s *webhookServer) validate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request

	l := log.WithValues("kind", req.Kind, "namespace", req.Namespace, "name", req.Name, "uid", req.UID,
		"operation", req.Operation, "user info", req.UserInfo)
	l.Info("AdmissionReview")

	switch req.Kind.Kind {
	case kafkaTopic:
		var topic v1alpha1.KafkaTopic
		if err := json.Unmarshal(req.Object.Raw, &topic); err != nil {
			l.Error(err, "Could not unmarshal raw object")
			return notAllowed(err.Error(), metav1.StatusReasonBadRequest)
		}
		if ok := util.ObjectManagedByClusterRegistry(topic.GetObjectMeta()); ok {
			l.Info("Skip validation as the resource is managed by Cluster Registry")
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
		return s.validateKafkaTopic(&topic)

	default:
		return notAllowed(fmt.Sprintf("Unexpected resource kind: %s", req.Kind.Kind), metav1.StatusReasonBadRequest)
	}
}

func (s *webhookServer) serve(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "Failed to read request body")
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		log.Error(errors.New("empty body"), "empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err = fmt.Errorf("Content-Type=%s, expect application/json", contentType)
		log.Error(err, "invalid content type")
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := s.deserializer.Decode(body, nil, &ar); err != nil {
		log.Error(err, "Can't decode body")
		admissionResponse = notAllowed(err.Error(), metav1.StatusReasonBadRequest)
	} else {
		admissionResponse = s.validate(&ar)
	}

	admissionReview := admissionv1.AdmissionReview{
		// APIVersion and Kind must be set for admission/v1, or the request would fail
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionv1.SchemeGroupVersion.String(),
			Kind:       "AdmissionReview",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		log.Error(err, "Can't encode response")
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		log.Error(err, "Can't write response")
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func notAllowed(msg string, reason metav1.StatusReason) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: msg,
			Reason:  reason,
		},
	}
}
