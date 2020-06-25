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
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	banzaicloudv1beta1 "github.com/banzaicloud/kafka-operator/api/v1beta1"
	bcutil "github.com/banzaicloud/kafka-operator/pkg/util"
)

const (
	basePath                = "kafkacruisecontrol"
	removeBrokerAction      = "remove_broker"
	addBrokerAction         = "add_broker"
	getTaskListAction       = "user_tasks"
	kafkaClusterStateAction = "kafka_cluster_state"
	clusterLoadAction       = "load"
	rebalanceAction         = "rebalance"
	killProposalAction      = "stop_proposal_execution"
	serviceNameTemplate     = "%s-cruisecontrol-svc"
	brokerAlive             = "ALIVE"
)

var errCruiseControlNotReturned200 = errors.New("non 200 response from cruise-control")

var log = logf.Log.WithName("cruise-control-methods")

type CruiseControlScaler struct {
	namespace               string
	kubernetesClusterDomain string
	endpoint                string
	clusterName             string
}

func NewCruiseControlScaler(namespace, kubernetesClusterDomain, endpoint, clusterName string) CruiseControlScaler {
	return CruiseControlScaler{
		namespace:               namespace,
		kubernetesClusterDomain: kubernetesClusterDomain,
		endpoint:                endpoint,
		clusterName:             clusterName,
	}
}

func (cc *CruiseControlScaler) generateUrlForCC(action string, options map[string]string) string {
	optionURL := ""
	for option, value := range options {
		optionURL = optionURL + option + "=" + value + "&"
	}
	if cc.endpoint != "" {
		return "http://" + cc.endpoint + "/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
	}
	return "http://" + fmt.Sprintf(serviceNameTemplate, cc.clusterName) + "." + cc.namespace + ".svc." + cc.kubernetesClusterDomain + ":8090/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
}

func (cc *CruiseControlScaler) postCruiseControl(action string, options map[string]string) (*http.Response, error) {

	requestURL := cc.generateUrlForCC(action, options)
	rsp, err := http.Post(requestURL, "text/plain", nil)
	if err != nil {
		log.Error(err, "error during talking to cruise-control")
		return nil, err
	}
	if rsp.StatusCode != 200 && rsp.StatusCode != 202 {
		ccErr, parseErr := parseCCErrorFromResp(rsp.Body)
		if parseErr != nil {
			return nil, parseErr
		}
		closeErr := rsp.Body.Close()
		if closeErr != nil {
			return nil, closeErr
		}
		log.Error(errCruiseControlNotReturned200, "non-200 response returned by cruise-control",
			"request", requestURL, "status", rsp.Status, "error message", ccErr)
		return rsp, errCruiseControlNotReturned200
	}

	return rsp, nil
}

func (cc *CruiseControlScaler) getCruiseControl(action string, options map[string]string) (*http.Response, error) {

	requestURL := cc.generateUrlForCC(action, options)
	rsp, err := http.Get(requestURL)
	if err != nil {
		log.Error(err, "error during talking to cruise-control")
		return nil, err
	}
	if rsp.StatusCode != 200 {
		ccErr, parseErr := parseCCErrorFromResp(rsp.Body)
		if parseErr != nil {
			return nil, parseErr
		}
		closeErr := rsp.Body.Close()
		if closeErr != nil {
			return nil, closeErr
		}
		log.Error(errCruiseControlNotReturned200, "non-200 response returned by cruise-control",
			"request", requestURL, "status", rsp.Status, "error message", ccErr)
		return rsp, errCruiseControlNotReturned200
	}

	return rsp, nil
}

func parseCCErrorFromResp(input io.Reader) (string, error) {
	var errorFromResponse struct {
		ErrorMessage string
	}

	body, err := ioutil.ReadAll(input)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(body, &errorFromResponse)
	if err != nil {
		return "", err
	}

	return errorFromResponse.ErrorMessage, err
}

func (cc *CruiseControlScaler) isKafkaBrokerDiskReady(brokerIdsWithMountPath map[string][]string) (bool, error) {
	options := map[string]string{
		"json": "true",
	}

	rsp, err := cc.getCruiseControl(kafkaClusterStateAction, options)
	if err != nil {
		keyVals := []interface{}{
			"namespace", cc.namespace,
			"clusterName", cc.clusterName,
			//"brokerId", brokerIds,
			//"path", mountPath,
		}
		log.Error(err, "can't check if broker disk is ready as Cruise Control not ready", keyVals...)
		return false, err
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return false, err
	}

	err = rsp.Body.Close()
	if err != nil {
		return false, err
	}

	var response struct {
		KafkaBrokerState struct {
			OnlineLogDirsByBrokerId map[string][]string
		}
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, err
	}

	for brokerId, volumeMounts := range brokerIdsWithMountPath {
		if ccOnlineLogDirs, ok := response.KafkaBrokerState.OnlineLogDirsByBrokerId[brokerId]; ok {
			for _, volumeMount := range volumeMounts {
				match := false
				for _, ccOnlineLogDir := range ccOnlineLogDirs {
					if strings.HasPrefix(strings.TrimSpace(ccOnlineLogDir), strings.TrimSpace(volumeMount)) {
						match = true
						break
					}
				}

				if !match {
					return false, nil
				}
			}

		} else {
			return false, nil
		}
	}

	return true, nil
}

// Get brokers status from CC from a provided list of broker ids
func (cc *CruiseControlScaler) GetLiveKafkaBrokersFromCruiseControl(brokerIds []string) ([]string, error) {

	options := map[string]string{
		"json": "true",
	}

	rsp, err := cc.getCruiseControl(clusterLoadAction, options)
	if err != nil {
		log.Error(err, "can't work with cruise-control because it is not ready")
		return nil, err
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	err = rsp.Body.Close()
	if err != nil {
		return nil, err
	}

	var response struct {
		Brokers []struct {
			Broker      float64
			BrokerState string
		}
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	aliveBrokers := make([]string, 0, len(brokerIds))

	for _, broker := range response.Brokers {
		bIdStr := fmt.Sprintf("%g", broker.Broker)
		if broker.BrokerState == brokerAlive && bcutil.StringSliceContains(brokerIds, bIdStr) {
			aliveBrokers = append(aliveBrokers, bIdStr)
			log.Info("broker is available in cruise-control", "brokerId", bIdStr)
		}
	}
	return aliveBrokers, nil
}

// GetBrokerIDWithLeastPartition returns
func (cc *CruiseControlScaler) GetBrokerIDWithLeastPartition() (string, error) {

	brokerWithLeastPartition := ""

	options := map[string]string{
		"json": "true",
	}

	rsp, err := cc.getCruiseControl(kafkaClusterStateAction, options)
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
func (cc *CruiseControlScaler) UpScaleCluster(brokerIds []string) (string, string, error) {

	liveBrokers, err := cc.GetLiveKafkaBrokersFromCruiseControl(brokerIds)
	if err != nil {
		return "", "", err
	}
	ready := bcutil.AreStringSlicesIdentical(liveBrokers, brokerIds)

	if !ready {
		return "", "", errors.New("broker(s) not yet ready in cruise-control")
	}

	options := map[string]string{
		"json":     "true",
		"dryrun":   "false",
		"brokerid": strings.Join(brokerIds, ","),
	}

	uResp, err := cc.postCruiseControl(addBrokerAction, options)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "can't upscale cluster gracefully since post to cruise-control failed")
		return "", "", err
	}

	log.Info("Initiated upscale in cruise control")

	uTaskId := uResp.Header.Get("User-Task-Id")
	startTimeStamp := uResp.Header.Get("Date")

	return uTaskId, startTimeStamp, nil
}

// DownsizeCluster downscales Kafka cluster
func (cc *CruiseControlScaler) DownsizeCluster(brokerIds []string) (string, string, error) {

	options := map[string]string{
		"brokerid": strings.Join(brokerIds, ","),
		"dryrun":   "false",
		"json":     "true",
	}

	var dResp *http.Response

	dResp, err := cc.postCruiseControl(removeBrokerAction, options)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "downsize cluster gracefully failed since CC returned non 200")
		return "", "", err
	}

	log.Info("Initiated downsize in cruise control")
	uTaskId := dResp.Header.Get("User-Task-Id")
	startTimeStamp := dResp.Header.Get("Date")

	return uTaskId, startTimeStamp, nil
}

// RebalanceDisks rebalances Kafka broker replicas between disks using CC
func (cc *CruiseControlScaler) RebalanceDisks(brokerIdsWithMountPath map[string][]string) (string, string, error) {

	ready, err := cc.isKafkaBrokerDiskReady(brokerIdsWithMountPath)
	if err != nil {
		return "", "", err
	}
	if !ready {
		return "", "", errors.New("broker disk is not ready yet")
	}

	options := map[string]string{
		"dryrun":         "false",
		"json":           "true",
		"rebalance_disk": "true",
	}

	rResp, err := cc.postCruiseControl(rebalanceAction, options)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "can't rebalance brokers disk gracefully since post to cruise-control failed")
		return "", "", err
	}

	log.Info("Initiated disk rebalance in cruise control")
	uTaskId := rResp.Header.Get("User-Task-Id")
	startTimeStamp := rResp.Header.Get("Date")

	return uTaskId, startTimeStamp, nil
}

// RebalanceCluster rebalances Kafka cluster using CC
func (cc *CruiseControlScaler) RebalanceCluster() (string, error) {

	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
	}

	dResp, err := cc.postCruiseControl(rebalanceAction, options)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return "", err
	}
	log.Info("Initiated rebalance in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	return uTaskId, nil
}

// RunPreferedLeaderElectionInCluster runs leader election in  Kafka cluster using CC
func (cc *CruiseControlScaler) RunPreferedLeaderElectionInCluster() (string, error) {

	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
		"goals":  "PreferredLeaderElectionGoal",
	}

	dResp, err := cc.postCruiseControl(rebalanceAction, options)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return "", err
	}
	log.Info("Initiated rebalance in cruise control")

	uTaskId := dResp.Header.Get("User-Task-Id")

	return uTaskId, nil
}

// KillCCTask kills the specified CC task
func (cc *CruiseControlScaler) KillCCTask() error {
	options := map[string]string{
		"json": "true",
	}

	_, err := cc.postCruiseControl(killProposalAction, options)
	if err != nil {
		log.Error(err, "can't kill running tasks since post to cruise-control failed")
		return err
	}
	log.Info("Task killed")

	return nil
}

// GetCCTaskState checks whether the given CC Task ID finished or not
func (cc *CruiseControlScaler) GetCCTaskState(uTaskId string) (banzaicloudv1beta1.CruiseControlUserTaskState, error) {

	gResp, err := cc.getCruiseControl(getTaskListAction, map[string]string{
		"json":          "true",
		"user_task_ids": uTaskId,
	})
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
