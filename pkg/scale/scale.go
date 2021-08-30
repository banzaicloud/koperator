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

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	banzaicloudv1beta1 "github.com/banzaicloud/koperator/api/v1beta1"
	bcutil "github.com/banzaicloud/koperator/pkg/util"
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
var newCruiseControlScaler = createNewDefaultCruiseControlScaler

var log = logf.Log.WithName("cruise-control-methods")

type CruiseControlScaler interface {
	GetLiveKafkaBrokersFromCruiseControl(brokerIDs []string) ([]string, error)
	GetBrokerIDWithLeastPartition() (string, error)
	UpScaleCluster(brokerIDs []string) (string, string, error)
	DownsizeCluster(brokerIDs []string) (string, string, error)
	RebalanceDisks(brokerIDsWithMountPath map[string][]string) (string, string, error)
	RebalanceCluster() (string, error)
	RunPreferedLeaderElectionInCluster() (string, error)
	KillCCTask() error
	GetCCTaskState(uTaskID string) (banzaicloudv1beta1.CruiseControlUserTaskState, error)
}

type cruiseControlScaler struct {
	CruiseControlScaler
	namespace               string
	kubernetesClusterDomain string
	endpoint                string
	clusterName             string
}

func NewCruiseControlScaler(namespace, kubernetesClusterDomain, endpoint, clusterName string) CruiseControlScaler {
	return newCruiseControlScaler(namespace, kubernetesClusterDomain, endpoint, clusterName)
}

func MockNewCruiseControlScaler() {
	newCruiseControlScaler = createMockCruiseControlScaler
}

func createNewDefaultCruiseControlScaler(namespace, kubernetesClusterDomain, endpoint, clusterName string) CruiseControlScaler {
	return &cruiseControlScaler{
		namespace:               namespace,
		kubernetesClusterDomain: kubernetesClusterDomain,
		endpoint:                endpoint,
		clusterName:             clusterName,
	}
}

func createMockCruiseControlScaler(namespace, kubernetesClusterDomain, endpoint, clusterName string) CruiseControlScaler {
	return &mockCruiseControlScaler{}
}

func (cc *cruiseControlScaler) generateUrlForCC(action string, options map[string]string) string {
	optionURL := ""
	for option, value := range options {
		optionURL = optionURL + option + "=" + value + "&"
	}
	if cc.endpoint != "" {
		return "http://" + cc.endpoint + "/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
	}
	return "http://" + fmt.Sprintf(serviceNameTemplate, cc.clusterName) + "." + cc.namespace + ".svc." + cc.kubernetesClusterDomain + ":8090/" + basePath + "/" + action + "?" + strings.TrimSuffix(optionURL, "&")
}

func (cc *cruiseControlScaler) postCruiseControl(action string, options map[string]string) (*http.Response, error) {
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

func (cc *cruiseControlScaler) getCruiseControl(action string, options map[string]string) (*http.Response, error) {
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

func (cc *cruiseControlScaler) isKafkaBrokerDiskReady(brokerIDsWithMountPath map[string][]string) (bool, error) {
	options := map[string]string{
		"json": "true",
	}

	rsp, err := cc.getCruiseControl(kafkaClusterStateAction, options)
	if err != nil {
		keyVals := []interface{}{
			"namespace", cc.namespace,
			"clusterName", cc.clusterName,
			//"brokerId", brokerIDs,
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
			OnlineLogDirsByBrokerID map[string][]string
		}
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, err
	}

	for brokerID, volumeMounts := range brokerIDsWithMountPath {
		if ccOnlineLogDirs, ok := response.KafkaBrokerState.OnlineLogDirsByBrokerID[brokerID]; ok {
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
func (cc *cruiseControlScaler) GetLiveKafkaBrokersFromCruiseControl(brokerIDs []string) ([]string, error) {
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

	aliveBrokers := make([]string, 0, len(brokerIDs))

	for _, broker := range response.Brokers {
		bIDStr := fmt.Sprintf("%g", broker.Broker)
		if broker.BrokerState == brokerAlive && bcutil.StringSliceContains(brokerIDs, bIDStr) {
			aliveBrokers = append(aliveBrokers, bIDStr)
			log.Info("broker is available in cruise-control", "brokerID", bIDStr)
		}
	}
	return aliveBrokers, nil
}

// GetBrokerIDWithLeastPartition returns
func (cc *cruiseControlScaler) GetBrokerIDWithLeastPartition() (string, error) {
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
			ReplicaCountByBrokerID map[string]float64
		}
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return brokerWithLeastPartition, err
	}

	replicaCountByBroker := response.KafkaBrokerState.ReplicaCountByBrokerID
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
func (cc *cruiseControlScaler) UpScaleCluster(brokerIDs []string) (string, string, error) {
	liveBrokers, err := cc.GetLiveKafkaBrokersFromCruiseControl(brokerIDs)
	if err != nil {
		return "", "", err
	}
	ready := bcutil.AreStringSlicesIdentical(liveBrokers, brokerIDs)

	if !ready {
		return "", "", errors.New("broker(s) not yet ready in cruise-control")
	}

	options := map[string]string{
		"json":     "true",
		"dryrun":   "false",
		"brokerid": strings.Join(brokerIDs, ","),
	}

	uResp, err := cc.postCruiseControl(addBrokerAction, options)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "can't upscale cluster gracefully since post to cruise-control failed")
		return "", "", err
	}
	defer uResp.Body.Close()

	log.Info("Initiated upscale in cruise control")

	uTaskID := uResp.Header.Get("User-Task-Id")
	startTimeStamp := uResp.Header.Get("Date")

	return uTaskID, startTimeStamp, nil
}

// DownsizeCluster downscales Kafka cluster
func (cc *cruiseControlScaler) DownsizeCluster(brokerIDs []string) (string, string, error) {
	options := map[string]string{
		"brokerid": strings.Join(brokerIDs, ","),
		"dryrun":   "false",
		"json":     "true",
	}

	var dResp *http.Response

	dResp, err := cc.postCruiseControl(removeBrokerAction, options)
	if err != nil && err != errCruiseControlNotReturned200 {
		log.Error(err, "downsize cluster gracefully failed since CC returned non 200")
		return "", "", err
	}

	defer dResp.Body.Close()

	log.Info("Initiated downsize in cruise control")
	uTaskID := dResp.Header.Get("User-Task-Id")
	startTimeStamp := dResp.Header.Get("Date")

	return uTaskID, startTimeStamp, nil
}

// RebalanceDisks rebalances Kafka broker replicas between disks using CC
func (cc *cruiseControlScaler) RebalanceDisks(brokerIDsWithMountPath map[string][]string) (string, string, error) {
	ready, err := cc.isKafkaBrokerDiskReady(brokerIDsWithMountPath)
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

	rResp.Body.Close()

	log.Info("Initiated disk rebalance in cruise control")
	uTaskID := rResp.Header.Get("User-Task-Id")
	startTimeStamp := rResp.Header.Get("Date")

	return uTaskID, startTimeStamp, nil
}

// RebalanceCluster rebalances Kafka cluster using CC
func (cc *cruiseControlScaler) RebalanceCluster() (string, error) {
	options := map[string]string{
		"dryrun": "false",
		"json":   "true",
	}

	dResp, err := cc.postCruiseControl(rebalanceAction, options)
	if err != nil {
		log.Error(err, "can't rebalance cluster gracefully since post to cruise-control failed")
		return "", err
	}
	defer dResp.Body.Close()
	log.Info("Initiated rebalance in cruise control")

	uTaskID := dResp.Header.Get("User-Task-Id")

	return uTaskID, nil
}

// RunPreferedLeaderElectionInCluster runs leader election in  Kafka cluster using CC
func (cc *cruiseControlScaler) RunPreferedLeaderElectionInCluster() (string, error) {
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
	defer dResp.Body.Close()

	log.Info("Initiated rebalance in cruise control")

	uTaskID := dResp.Header.Get("User-Task-Id")

	return uTaskID, nil
}

// KillCCTask kills the specified CC task
func (cc *cruiseControlScaler) KillCCTask() error {
	options := map[string]string{
		"json": "true",
	}

	resp, err := cc.postCruiseControl(killProposalAction, options)
	if err != nil {
		log.Error(err, "can't kill running tasks since post to cruise-control failed")
		return err
	}
	defer resp.Body.Close()
	log.Info("Task killed")

	return nil
}

// GetCCTaskState checks whether the given CC Task ID finished or not
func (cc *cruiseControlScaler) GetCCTaskState(uTaskID string) (banzaicloudv1beta1.CruiseControlUserTaskState, error) {
	gResp, err := cc.getCruiseControl(getTaskListAction, map[string]string{
		"json":          "true",
		"user_task_ids": uTaskID,
	})
	if err != nil {
		log.Error(err, "can't get task list from cruise-control")
		return "", err
	}

	var taskLists struct {
		UserTasks []struct {
			Status     string
			UserTaskID string
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
		if task.UserTaskID == uTaskID {
			log.Info("Cruise control task state", "state", task.Status, "taskID", uTaskID)
			return banzaicloudv1beta1.CruiseControlUserTaskState(task.Status), nil
		}
	}
	log.Info("Cruise control task not found", "taskID", uTaskID)
	return banzaicloudv1beta1.CruiseControlTaskNotFound, nil
}
