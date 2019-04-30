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

package receiver

import (
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"
)

// APIEndPoint for token handling
const APIEndPoint = "/"

// HTTPController collects the greeting use cases and exposes them as HTTP handlers.
type HTTPController struct {
	Logger logr.Logger
}

// NewHTTPHandler returns a new HTTP handler for the greeter.
func NewHTTPHandler(log logr.Logger) http.Handler {
	mux := http.NewServeMux()
	controller := NewHTTPController(log)
	mux.HandleFunc(APIEndPoint, controller.reciveAlert)
	return mux
}

// NewHTTPController returns a new HTTPController instance.
func NewHTTPController(log logr.Logger) *HTTPController {
	return &HTTPController{
		Logger: log,
	}
}

func (a *HTTPController) reciveAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(body)
		a.Logger.Info("INFO", "http_body", body)

	default:
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
}
