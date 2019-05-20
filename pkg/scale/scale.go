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

package scale

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/banzaicloud/kafka-operator/pkg/util/backoff"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	basePath                 = "kafkacruisecontrol"
	removeBrokerAction       = "remove_broker"
	cruiseControlStateAction = "state"
	addBrokerAction          = "add_broker"
	getTaskListAction        = "user_tasks"
	kafkaClusterStateAction  = "kafka_cluster_state"
	rebalanceAction          = "rebalance"
	serviceName              = "cruisecontrol-svc"
)

var errCruiseControlNotReady = errors.New("cruise-control is not ready")
var errCruiseControlNotReturned200 = errors.New("non 200 response from cruise-control")

var log = logf.Log.WithName("cruise-control-methods")

func generateUrlForCC(action, namespace string, options map[string]string) string {
	optionURL := ""
	for option, value := range options {
		optionURL = optionURL + option + "=" + value + "&"
	}

	return "http://" + serviceName + "." + namespace + ".svc.cluster.local:8090/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
	//TODO only for testing
	//return "http://localhost:8090/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
}

func postCruiseControl(action, namespace string, options map[string]string) (*http.Response, error) {

	requestURl := generateUrlForCC(action, namespace, options)
	rsp, err := http.Post(requestURl, "text/plain", nil)
	if err != nil {
		log.Error(err, "error during talking to cruise-control")
		return nil, err
	}
	if rsp.StatusCode != 200 {
		log.Error(errors.New("Non 200 response from cruise-control: "+rsp.Status), "error during talking to cruise-control")
		return nil, errCruiseControlNotReturned200
	}

	return rsp, nil
}

func getCruiseControl(action, namespace string, options map[string]string) (*http.Response, error) {

	requestURl := generateUrlForCC(action, namespace, options)
	rsp, err := http.Get(requestURl)
	if err != nil {
		log.Error(err, "error during talking to cruise-control")
		return nil, err
	}
	if rsp.StatusCode != 200 {
		log.Error(errors.New("Non 200 response from cruise-control: "+rsp.Status), "error during talking to cruise-control")
		return nil, errors.New("Non 200 response from cruise-control: " + rsp.Status)
	}

	return rsp, nil
}

func getCruiseControlStatus(namespace string) error {

	options := map[string]string{
		"substates": "ANALYZER",
		"json":      "true",
	}

	rsp, err := getCruiseControl(cruiseControlStateAction, namespace, options)
	if err != nil {
		log.Error(err, "can't work with cruise-control because it is not ready")
		return err
	}
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	err = rsp.Body.Close()
	if err != nil {
		return err
	}

	var response map[string]interface{}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}
	if !response["AnalyzerState"].(map[string]interface{})["isProposalReady"].(bool) {
		log.Info("could not handle graceful operation because cruise-control is not ready")
		return errCruiseControlNotReady
	}

	return nil
}

func isKafkaBrokerReady(brokerId, namespace string) (bool, error) {

	running := false

	options := map[string]string{
		"json": "true",
	}

	rsp, err := getCruiseControl(kafkaClusterStateAction, namespace, options)
	if err != nil {
		log.Error(err, "can't work with cruise-control because it is not ready")
		return running, err
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return running, err
	}

	err = rsp.Body.Close()
	if err != nil {
		return running, err
	}

	var response map[string]interface{}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return running, err
	}
	bId, _ := strconv.Atoi(brokerId)

	if len(response["KafkaBrokerState"].(map[string]interface{})["OnlineLogDirsByBrokerId"].(map[string]interface{})) == bId+1 {
		log.Info("waiting for broker to became available in cruise-control")
		running = true
	}
	return running, nil
}

func UpScaleCluster(brokerId, namespace string) error {

	err := getCruiseControlStatus(namespace)
	if err != nil {
		return err
	}

	var backoffConfig = backoff.ConstantBackoffConfig{
		Delay:      10 * time.Second,
		MaxRetries: 5,
	}
	var backoffPolicy = backoff.NewConstantBackoffPolicy(&backoffConfig)

	err = backoff.Retry(func() error {
		ready, err := isKafkaBrokerReady(brokerId, namespace)
		if err != nil {
			return err
		}
		if !ready {
			return errors.New("broker is not ready yet")
		}
		return nil
	}, backoffPolicy)
	if err != nil {
		return err
	}

	options := map[string]string{
		"json":     "true",
		"dryrun":   "false",
		"brokerid": brokerId,
	}

	var uResp *http.Response

	err = backoff.Retry(func() error {
		uResp, err = postCruiseControl(addBrokerAction, namespace, options)
		if err != nil && err != errCruiseControlNotReturned200 {
			log.Error(err, "can't upscale cluster gracefully since post to cruise-control failed")
			return err
		}
		if err == errCruiseControlNotReturned200 {
			log.Info("trying to communicate with cc")
			return err
		}
		return nil
	}, backoffPolicy)
	if err != nil {
		return err
	}

	log.Info("Initiated upscale in cruise control")

	uTaskId := uResp.Header.Get("User-Task-Id")

	err = checkIfCCTaskFinished(uTaskId, namespace)
	if err != nil {
		return err
	}
	return nil
}

func DownsizeCluster(brokerId, namespace string) error {

	err := getCruiseControlStatus(namespace)
	if err != nil {
		return err
	}

	options := map[string]string{
		"brokerid": brokerId,
		"dryrun":   "false",
		"json":     "true",
	}

	var backoffConfig = backoff.ConstantBackoffConfig{
		Delay:      10 * time.Second,
		MaxRetries: 5,
	}
	var backoffPolicy = backoff.NewConstantBackoffPolicy(&backoffConfig)

	var dResp *http.Response

	err = backoff.Retry(func() error {

		dResp, err = postCruiseControl(removeBrokerAction, namespace, options)
		if err != nil && err != errCruiseControlNotReturned200 {
			log.Error(err, "can't downsize cluster gracefully since post to cruise-control failed")
			return err
		}
		if err == errCruiseControlNotReturned200 {
			log.Info("trying to communicate with cc")
			return err
		}
		return nil
	}, backoffPolicy)
	log.Info("Initiated downsize in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	err = checkIfCCTaskFinished(uTaskId, namespace)
	if err != nil {
		return err
	}

	return nil
}

func RebalanceCluster(namespace string) error {

	err := getCruiseControlStatus(namespace)
	if err != nil {
		return err
	}

	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
	}

	dResp, err := postCruiseControl(rebalanceAction, namespace, options)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return err
	}
	log.Info("Initiated rebalance in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	err = checkIfCCTaskFinished(uTaskId, namespace)
	if err != nil {
		return err
	}

	return nil
}

func RunPreferedLeaderElectionInCluster(namespace string) error {

	err := getCruiseControlStatus(namespace)
	if err != nil {
		return err
	}

	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
		"goals":  "PreferredLeaderElectionGoal",
	}

	dResp, err := postCruiseControl(rebalanceAction, namespace, options)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return err
	}
	log.Info("Initiated rebalance in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	err = checkIfCCTaskFinished(uTaskId, namespace)
	if err != nil {
		return err
	}

	return nil
}

func checkIfCCTaskFinished(uTaskId, namespace string) error {
	ccRunning := true

	for ccRunning {

		gResp, err := getCruiseControl(getTaskListAction, namespace, map[string]string{
			"json":          "true",
			"user_task_ids": uTaskId,
		})
		if err != nil {
			log.Error(err, "can't get task list from cruise-control")
			return err
		}

		var taskLists map[string]interface{}

		body, err := ioutil.ReadAll(gResp.Body)
		if err != nil {
			return err
		}

		err = gResp.Body.Close()
		if err != nil {
			return err
		}

		err = json.Unmarshal(body, &taskLists)
		if err != nil {
			return err
		}
		// TODO use struct instead of casting things
		for _, task := range taskLists["userTasks"].([]interface{}) {
			if task.(map[string]interface{})["Status"].(string) != "Completed" {
				log.Info("Cruise control task  still running", "taskID", uTaskId)
				time.Sleep(20 * time.Second)
			}
		}
		ccRunning = false
	}
	return nil
}
