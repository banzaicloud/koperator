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
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	banzaicloudv1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	basePath                 = "kafkacruisecontrol"
	removeBrokerAction       = "remove_broker"
	cruiseControlStateAction = "state"
	addBrokerAction          = "add_broker"
	getTaskListAction        = "user_tasks"
	kafkaClusterStateAction  = "kafka_cluster_state"
	clusterLoad              = "load"
	rebalanceAction          = "rebalance"
	killProposalAction       = "stop_proposal_execution"
	serviceNameTemplate      = "%s-cruisecontrol-svc"
	brokerAlive              = "ALIVE"
)

var errCruiseControlNotReady = errors.New("cruise-control is not ready")
var errCruiseControlNotReturned200 = errors.New("non 200 response from cruise-control")

var log = logf.Log.WithName("cruise-control-methods")

func generateUrlForCC(action, namespace string, options map[string]string, ccEndpoint, clusterName string) string {
	optionURL := ""
	for option, value := range options {
		optionURL = optionURL + option + "=" + value + "&"
	}
	if ccEndpoint != "" {
		return "http://" + ccEndpoint + "/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
	}
	return "http://" + fmt.Sprintf(serviceNameTemplate, clusterName) + "." + namespace + ".svc.cluster.local:8090/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
	//TODO only for testing
	//return "http://localhost:8090/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
}

func postCruiseControl(action, namespace string, options map[string]string, ccEndpoint, clusterName string) (*http.Response, error) {

	requestURl := generateUrlForCC(action, namespace, options, ccEndpoint, clusterName)
	rsp, err := http.Post(requestURl, "text/plain", nil)
	if err != nil {
		log.Error(err, "error during talking to cruise-control")
		return nil, err
	}
	if rsp.StatusCode != 200 && rsp.StatusCode != 202 {
		log.Error(errors.New("Non 200 response from cruise-control: "+rsp.Status), "error during talking to cruise-control")
		return nil, errCruiseControlNotReturned200
	}

	return rsp, nil
}

func getCruiseControl(action, namespace string, options map[string]string, ccEndpoint, clusterName string) (*http.Response, error) {

	requestURl := generateUrlForCC(action, namespace, options, ccEndpoint, clusterName)
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

func GetCruiseControlStatus(namespace, ccEndpoint, clusterName string) error {

	options := map[string]string{
		"substates": "ANALYZER",
		"json":      "true",
	}

	rsp, err := getCruiseControl(cruiseControlStateAction, namespace, options, ccEndpoint, clusterName)
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

	var response struct {
		AnalyzerState struct {
			IsProposalReady bool
		}
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}
	if !response.AnalyzerState.IsProposalReady {
		log.Info("could not handle graceful operation because cruise-control is not ready")
		return errCruiseControlNotReady
	}

	return nil
}

func isKafkaBrokerReady(brokerId, namespace, ccEndpoint, clusterName string) (bool, error) {

	running := false

	options := map[string]string{
		"json": "true",
	}

	rsp, err := getCruiseControl(clusterLoad, namespace, options, ccEndpoint, clusterName)
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

	var response struct {
		Brokers []struct {
			Broker      float64
			BrokerState string
		}
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return running, err
	}

	bIdToFloat, _ := strconv.ParseFloat(brokerId, 32)

	for _, broker := range response.Brokers {
		if broker.Broker == bIdToFloat &&
			broker.BrokerState == brokerAlive {
			log.Info("broker is available in cruise-control")
			running = true
			break
		}
	}
	return running, nil
}

// GetBrokerIDWithLeastPartition returns
func GetBrokerIDWithLeastPartition(namespace, ccEndpoint, clusterName string) (string, error) {

	brokerWithLeastPartition := ""

	err := GetCruiseControlStatus(namespace, ccEndpoint, clusterName)
	if err != nil {
		return brokerWithLeastPartition, err
	}

	options := map[string]string{
		"json": "true",
	}

	rsp, err := getCruiseControl(kafkaClusterStateAction, namespace, options, ccEndpoint, clusterName)
	if err != nil {
		log.Error(err, "can't work with cruise-control because it is not ready")
		return brokerWithLeastPartition, err
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return brokerWithLeastPartition, err
	}

	err = rsp.Body.Close()
	if err != nil {
		return brokerWithLeastPartition, err
	}

	var response struct {
		KafkaBrokerState struct {
			ReplicaCountByBrokerId map[string]float64
		}
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return brokerWithLeastPartition, err
	}

	replicaCountByBroker := response.KafkaBrokerState.ReplicaCountByBrokerId
	replicaCount := float64(99999)

	for brokerID, replica := range replicaCountByBroker {
		if replicaCount > replica {
			replicaCount = replica
			brokerWithLeastPartition = brokerID
		}
	}
	return brokerWithLeastPartition, nil

}

// UpScaleCluster upscales Kafka cluster
func UpScaleCluster(brokerId, namespace, ccEndpoint, clusterName string) (string, string, error) {

	err := GetCruiseControlStatus(namespace, ccEndpoint, clusterName)
	if err != nil {
		return "", "", err
	}

	ready, err := isKafkaBrokerReady(brokerId, namespace, ccEndpoint, clusterName)
	if err != nil {
		return "", "", err
	}
	if !ready {
		return "", "", errors.New("broker is not ready yet")
	}

	options := map[string]string{
		"json":     "true",
		"dryrun":   "false",
		"brokerid": brokerId,
	}

	uResp, err := postCruiseControl(addBrokerAction, namespace, options, ccEndpoint, clusterName)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "can't upscale cluster gracefully since post to cruise-control failed")
		return "", "", err
	}
	if err == errCruiseControlNotReturned200 {
		log.Info("trying to communicate with cc")
		return "", "", err
	}

	log.Info("Initiated upscale in cruise control")

	uTaskId := uResp.Header.Get("User-Task-Id")
	startTimeStamp := uResp.Header.Get("Date")

	return uTaskId, startTimeStamp, nil
}

// DownsizeCluster downscales Kafka cluster
func DownsizeCluster(brokerId, namespace, ccEndpoint, clusterName string) (string, string, error) {

	err := GetCruiseControlStatus(namespace, ccEndpoint, clusterName)
	if err != nil {
		return "", "", err
	}

	options := map[string]string{
		"brokerid": brokerId,
		"dryrun":   "false",
		"json":     "true",
	}

	var dResp *http.Response

	dResp, err = postCruiseControl(removeBrokerAction, namespace, options, ccEndpoint, clusterName)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "downsize cluster gracefully failed since CC returned non 200")
		return "", "", err
	}
	if err == errCruiseControlNotReturned200 {
		log.Error(err, "could not communicate with cc")
		return "", "", err
	}

	log.Info("Initiated downsize in cruise control")
	uTaskId := dResp.Header.Get("User-Task-Id")
	startTimeStamp := dResp.Header.Get("Date")

	return uTaskId, startTimeStamp, nil
}

// RebalanceCluster rebalances Kafka cluster using CC
func RebalanceCluster(namespace, ccEndpoint, clusterName string) (string, error) {

	err := GetCruiseControlStatus(namespace, ccEndpoint, clusterName)
	if err != nil {
		return "", err
	}

	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
	}

	dResp, err := postCruiseControl(rebalanceAction, namespace, options, ccEndpoint, clusterName)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return "", err
	}
	log.Info("Initiated rebalance in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	return uTaskId, nil
}

// RunPreferedLeaderElectionInCluster runs leader election in  Kafka cluster using CC
func RunPreferedLeaderElectionInCluster(namespace, ccEndpoint, clusterName string) (string, error) {

	err := GetCruiseControlStatus(namespace, ccEndpoint, clusterName)
	if err != nil {
		return "", err
	}

	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
		"goals":  "PreferredLeaderElectionGoal",
	}

	dResp, err := postCruiseControl(rebalanceAction, namespace, options, ccEndpoint, clusterName)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return "", err
	}
	log.Info("Initiated rebalance in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	return uTaskId, nil
}

// KillCCTask kills the specified CC task
func KillCCTask(namespace, ccEndpoint, clusterName string) error {
	err := GetCruiseControlStatus(namespace, ccEndpoint, clusterName)
	if err != nil {
		return err
	}
	options := map[string]string{
		"json": "true",
	}

	_, err = postCruiseControl(killProposalAction, namespace, options, ccEndpoint, clusterName)
	if err != nil {
		log.Error(err, "can't kill running tasks since post to cruise-control failed")
		return err
	}
	log.Info("Task killed")

	return nil
}

// GetCCTaskState checks whether the given CC Task ID finished or not
func GetCCTaskState(uTaskId, namespace, ccEndpoint, clusterName string) (banzaicloudv1beta1.CruiseControlUserTaskState, error) {

	gResp, err := getCruiseControl(getTaskListAction, namespace, map[string]string{
		"json":          "true",
		"user_task_ids": uTaskId,
	}, ccEndpoint, clusterName)
	if err != nil {
		log.Error(err, "can't get task list from cruise-control")
		return "", err
	}

	var taskLists struct {
		UserTasks []struct {
			Status     string
			UserTaskId string
		}
	}

	body, err := ioutil.ReadAll(gResp.Body)
	if err != nil {
		return "", err
	}

	err = gResp.Body.Close()
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(body, &taskLists)
	if err != nil {
		return "", err
	}
	// No cc task found with this UID
	if len(taskLists.UserTasks) == 0 {
		return banzaicloudv1beta1.CruiseControlTaskNotFound, nil
	}

	for _, task := range taskLists.UserTasks {
		if task.UserTaskId == uTaskId {
			log.Info("Cruise control task state", "state", task.Status, "taskID", uTaskId)
			return banzaicloudv1beta1.CruiseControlUserTaskState(task.Status), nil
		}
	}
	log.Info("Cruise control task not found", "taskID", uTaskId)
	return banzaicloudv1beta1.CruiseControlTaskNotFound, nil
}
