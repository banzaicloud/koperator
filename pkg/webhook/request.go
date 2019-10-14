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

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kafkaTopic = reflect.TypeOf(v1alpha1.KafkaTopic{}).Name()
)

func (s *webhookServer) validate(ar *admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
	req := ar.Request

	log.Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, req.UID, req.Operation, req.UserInfo))

	switch req.Kind.Kind {

	case kafkaTopic:
		var topic v1alpha1.KafkaTopic
		if err := json.Unmarshal(req.Object.Raw, &topic); err != nil {
			log.Error(err, "Could not unmarshal raw object")
			return notAllowed(err.Error(), metav1.StatusReasonBadRequest)
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

	var admissionResponse *admissionv1beta1.AdmissionResponse
	ar := admissionv1beta1.AdmissionReview{}
	if _, _, err := s.deserializer.Decode(body, nil, &ar); err != nil {
		log.Error(err, "Can't decode body")
		admissionResponse = notAllowed(err.Error(), metav1.StatusReasonBadRequest)
	} else {
		admissionResponse = s.validate(&ar)
	}

	admissionReview := admissionv1beta1.AdmissionReview{}
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

func notAllowed(msg string, reason metav1.StatusReason) *admissionv1beta1.AdmissionResponse {
	return &admissionv1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: msg,
			Reason:  reason,
		},
	}
}
