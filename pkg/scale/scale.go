// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/banzaicloud/go-cruise-control/pkg/api"
	"github.com/banzaicloud/go-cruise-control/pkg/client"
	"github.com/banzaicloud/go-cruise-control/pkg/types"
	"github.com/go-logr/logr"

	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/api/v1beta1"
)

const (
	// Constants for the Cruise Control operations parameters
	// Check for more details: https://github.com/linkedin/cruise-control/wiki/REST-APIs
	ParamBrokerID           = "brokerid"
	ParamExcludeDemoted     = "exclude_recently_demoted_brokers"
	ParamExcludeRemoved     = "exclude_recently_removed_brokers"
	ParamDestbrokerIDs      = "destination_broker_ids"
	ParamRebalanceDisk      = "rebalance_disk"
	ParamBrokerIDAndLogDirs = "brokerid_and_logdirs"
	// Cruise Control API returns NullPointerException when a broker storage capacity calculations are missing
	// from the Cruise Control configurations
	nullPointerExceptionErrString = "NullPointerException"
	// This error happens when the Cruise Control has not got enough information from the metrics yet
	notEnoughValidWindowsExceptionErrString = "NotEnoughValidWindowsException"
)

var (
	newCruiseControlScaler   = createNewDefaultCruiseControlScaler
	addBrokerSupportedParams = map[string]struct{}{
		ParamBrokerID:       {},
		ParamExcludeDemoted: {},
		ParamExcludeRemoved: {},
	}
	removeBrokerSupportedParams = map[string]struct{}{
		ParamBrokerID:       {},
		ParamExcludeDemoted: {},
		ParamExcludeRemoved: {},
	}
	rebalanceSupportedParams = map[string]struct{}{
		ParamDestbrokerIDs:  {},
		ParamRebalanceDisk:  {},
		ParamExcludeDemoted: {},
		ParamExcludeRemoved: {},
	}
	removeDisksSupportedParams = map[string]struct{}{
		ParamBrokerIDAndLogDirs: {},
	}
)

func ScaleFactoryFn() func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (CruiseControlScaler, error) {
	return func(ctx context.Context, kafkaCluster *v1beta1.KafkaCluster) (CruiseControlScaler, error) {
		return NewCruiseControlScaler(ctx, CruiseControlURLFromKafkaCluster(kafkaCluster))
	}
}

func NewCruiseControlScaler(ctx context.Context, serverURL string) (CruiseControlScaler, error) {
	return newCruiseControlScaler(ctx, serverURL)
}

func createNewDefaultCruiseControlScaler(ctx context.Context, serverURL string) (CruiseControlScaler, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("Scaler")

	cfg := &client.Config{
		ServerURL: serverURL,
		UserAgent: "koperator",
	}

	cruisecontrol, err := client.NewClient(cfg)
	if err != nil {
		log.Error(err, "creating Cruise Control client failed")
		return nil, err
	}
	return &cruiseControlScaler{
		log:    log,
		client: cruisecontrol,
	}, nil
}

type cruiseControlScaler struct {
	CruiseControlScaler

	log    logr.Logger
	client *client.Client
}

// Status returns a StatusTaskResult describing the internal state of Cruise Control.
func (cc *cruiseControlScaler) Status(ctx context.Context) (StatusTaskResult, error) {
	req := api.StateRequestWithDefaults()
	req.Verbose = true
	resp, err := cc.client.State(ctx, req)
	if err != nil {
		return StatusTaskResult{}, err
	}

	// if the execution takes too much time, then Cruise Control converts the request into async
	// and returns the taskID
	if resp.Result == nil {
		return StatusTaskResult{
			TaskResult: &Result{
				TaskID:             resp.TaskID,
				StartedAt:          resp.Date,
				ResponseStatusCode: resp.StatusCode,
				RequestURL:         resp.RequestURL,
				State:              v1beta1.CruiseControlTaskActive,
			},
		}, nil
	}

	status := convert(resp.Result)

	return StatusTaskResult{
		TaskResult: &Result{
			TaskID:             resp.TaskID,
			StartedAt:          resp.Date,
			ResponseStatusCode: resp.StatusCode,
			RequestURL:         resp.RequestURL,
			State:              v1beta1.CruiseControlTaskActive,
		},
		Status: &status,
	}, nil
}

// StatusTask returns the latest state of the Status Cruise Control task.
func (cc *cruiseControlScaler) StatusTask(ctx context.Context, taskID string) (StatusTaskResult, error) {
	req := &api.UserTasksRequest{
		UserTaskIDs:         []string{taskID},
		FetchCompletedTasks: true,
	}

	resp, err := cc.client.UserTasks(ctx, req)
	if err != nil {
		return StatusTaskResult{}, err
	}

	//CC no longer has the info about the task, mark it as completed with error
	if len(resp.Result.UserTasks) == 0 {
		return StatusTaskResult{
			TaskResult: &Result{
				TaskID: taskID,
				State:  v1beta1.CruiseControlTaskCompletedWithError,
			},
		}, nil
	}

	if len(resp.Result.UserTasks) != 1 {
		return StatusTaskResult{}, fmt.Errorf("could not get the Cruise Control state, expected the response for 1 task (%s), but got %d responses", taskID, len(resp.Result.UserTasks))
	}

	taskInfo := resp.Result.UserTasks[0]
	status := taskInfo.Status
	if status == types.UserTaskStatusCompleted {
		result := &types.StateResult{}
		if err = json.Unmarshal([]byte(taskInfo.OriginalResponse), result); err != nil {
			return StatusTaskResult{}, err
		}

		status := convert(result)

		return StatusTaskResult{
			TaskResult: &Result{
				TaskID:    taskInfo.UserTaskID,
				StartedAt: taskInfo.StartMs.UTC().String(),
				State:     v1beta1.CruiseControlUserTaskState(taskInfo.Status.String()),
			},
			Status: &status,
		}, nil
	}

	return StatusTaskResult{
		TaskResult: &Result{
			TaskID:    taskInfo.UserTaskID,
			StartedAt: taskInfo.StartMs.UTC().String(),
			State:     v1beta1.CruiseControlUserTaskState(taskInfo.Status.String()),
		},
	}, nil
}

func convert(result *types.StateResult) CruiseControlStatus {
	goalsReady := true
	if len(result.AnalyzerState.GoalReadiness) > 0 {
		for _, goal := range result.AnalyzerState.GoalReadiness {
			if goal.Status != types.GoalReadinessStatusReady {
				goalsReady = false
				break
			}
		}
	}

	status := CruiseControlStatus{
		MonitorReady:       result.MonitorState.State == types.MonitorStateRunning,
		ExecutorReady:      result.ExecutorState.State == types.ExecutorStateTypeNoTaskInProgress,
		AnalyzerReady:      result.AnalyzerState.IsProposalReady && goalsReady,
		ProposalReady:      result.AnalyzerState.IsProposalReady,
		GoalsReady:         goalsReady,
		MonitoredWindows:   result.MonitorState.NumMonitoredWindows,
		MonitoringCoverage: result.MonitorState.MonitoringCoveragePercentage,
	}
	return status
}

// IsReady returns true if the Analyzer and Monitor components of Cruise Control are in ready state.
func (cc *cruiseControlScaler) IsReady(ctx context.Context) bool {
	status, err := cc.Status(ctx)
	if err != nil || status.Status == nil {
		cc.log.Error(err, "could not get Cruise Control status")
		return false
	}
	cc.log.Info("cruise control readiness",
		"analyzer", status.Status.AnalyzerReady,
		"monitor", status.Status.MonitorReady,
		"executor", status.Status.ExecutorReady,
		"goals ready", status.Status.GoalsReady,
		"monitored windows", status.Status.MonitoredWindows,
		"monitoring coverage percentage", status.Status.MonitoringCoverage)
	return status.Status.IsReady()
}

// IsUp returns true if Cruise Control is online.
func (cc *cruiseControlScaler) IsUp(ctx context.Context) bool {
	_, err := cc.client.State(ctx, api.StateRequestWithDefaults())
	return err == nil
}

// UserTasks returns list of Result describing User Tasks from Cruise Control for the provided task IDs.
func (cc *cruiseControlScaler) UserTasks(ctx context.Context, taskIDs ...string) ([]*Result, error) {
	req := &api.UserTasksRequest{
		UserTaskIDs: taskIDs,
	}

	resp, err := cc.client.UserTasks(ctx, req)
	if err != nil {
		return nil, err
	}

	results := make([]*Result, len(resp.Result.UserTasks))
	for idx, taskInfo := range resp.Result.UserTasks {
		results[idx] = &Result{
			TaskID:    taskInfo.UserTaskID,
			StartedAt: taskInfo.StartMs.UTC().String(),
			State:     v1beta1.CruiseControlUserTaskState(taskInfo.Status.String()),
		}
	}

	return results, nil
}

// parseBrokerIDtoSlice parses brokerIDs to int slice
func parseBrokerIDtoSlice(brokerid string) ([]int32, error) {
	var brokerIDIntSlice []int32
	splitBrokerIDs := strings.Split(brokerid, ",")
	for _, brokerID := range splitBrokerIDs {
		brokerIDint, err := strconv.ParseInt(brokerID, 10, 32)
		if err != nil {
			return nil, err
		}
		brokerIDIntSlice = append(brokerIDIntSlice, int32(brokerIDint))
	}
	return brokerIDIntSlice, nil
}

// AddBrokersWithParams requests Cruise Control to add the list of provided brokers to the Kafka cluster
// by reassigning partition replicas to them. The broker list and operation properties can be added
// with the use of the params argument.
func (cc *cruiseControlScaler) AddBrokersWithParams(ctx context.Context, params map[string]string) (*Result, error) {
	addBrokerReq := &api.AddBrokerRequest{
		AllowCapacityEstimation: true,
		DataFrom:                types.ProposalDataSourceValidWindows,
		UseReadyDefaultGoals:    true,
	}
	for param, pvalue := range params {
		if _, ok := addBrokerSupportedParams[param]; ok {
			switch param {
			case ParamBrokerID:
				ret, err := parseBrokerIDtoSlice(pvalue)
				if err != nil {
					return nil, err
				}
				addBrokerReq.BrokerIDs = ret
			case ParamExcludeDemoted:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				addBrokerReq.ExcludeRecentlyDemotedBrokers = ret
			case ParamExcludeRemoved:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				addBrokerReq.ExcludeRecentlyRemovedBrokers = ret
			default:
				return nil, fmt.Errorf("unsupported %s parameter: %s, supported parameters: %s", v1alpha1.OperationAddBroker, param, addBrokerSupportedParams)
			}
		}
	}

	addBrokerResp, err := cc.client.AddBroker(ctx, addBrokerReq)
	if err != nil {
		return &Result{
			TaskID:             addBrokerResp.TaskID,
			StartedAt:          addBrokerResp.Date,
			ResponseStatusCode: addBrokerResp.StatusCode,
			RequestURL:         addBrokerResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             addBrokerResp.TaskID,
		StartedAt:          addBrokerResp.Date,
		ResponseStatusCode: addBrokerResp.StatusCode,
		RequestURL:         addBrokerResp.RequestURL,
		Result:             addBrokerResp.Result,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

// StopExecution requests Cruise Control to stop running operation gracefully
func (cc *cruiseControlScaler) StopExecution(ctx context.Context) (*Result, error) {
	stopReq := &api.StopProposalExecutionRequest{}
	stopResp, err := cc.client.StopProposalExecution(ctx, stopReq)
	if err != nil {
		return &Result{
			TaskID:             stopResp.TaskID,
			StartedAt:          stopResp.Date,
			ResponseStatusCode: stopResp.StatusCode,
			RequestURL:         stopResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:    stopResp.TaskID,
		StartedAt: stopResp.Date,
		State:     v1beta1.CruiseControlTaskActive,
	}, nil
}

func (cc *cruiseControlScaler) RemoveBrokersWithParams(ctx context.Context, params map[string]string) (*Result, error) {
	rmBrokerReq := &api.RemoveBrokerRequest{
		AllowCapacityEstimation: true,
		DataFrom:                types.ProposalDataSourceValidWindows,
		UseReadyDefaultGoals:    true,
	}
	for param, pvalue := range params {
		if _, ok := removeBrokerSupportedParams[param]; ok {
			switch param {
			case ParamBrokerID:
				ret, err := parseBrokerIDtoSlice(pvalue)
				if err != nil {
					return nil, err
				}
				rmBrokerReq.BrokerIDs = ret
			case ParamExcludeDemoted:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				rmBrokerReq.ExcludeRecentlyDemotedBrokers = ret
			case ParamExcludeRemoved:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				rmBrokerReq.ExcludeRecentlyRemovedBrokers = ret
			default:
				return nil, fmt.Errorf("unsupported %s parameter: %s, supported parameters: %s", v1alpha1.OperationRemoveBroker, param, removeBrokerSupportedParams)
			}
		}
	}

	rmBrokerResp, err := cc.client.RemoveBroker(ctx, rmBrokerReq)
	if err != nil {
		return &Result{
			TaskID:             rmBrokerResp.TaskID,
			StartedAt:          rmBrokerResp.Date,
			ResponseStatusCode: rmBrokerResp.StatusCode,
			RequestURL:         rmBrokerResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             rmBrokerResp.TaskID,
		StartedAt:          rmBrokerResp.Date,
		ResponseStatusCode: rmBrokerResp.StatusCode,
		RequestURL:         rmBrokerResp.RequestURL,
		Result:             rmBrokerResp.Result,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

// AddBrokers requests Cruise Control to add the list of provided brokers to the Kafka cluster
// by reassigning partition replicas to them.
// Request returns an error if not all brokers are available in Cruise Control.
func (cc *cruiseControlScaler) AddBrokers(ctx context.Context, brokerIDs ...string) (*Result, error) {
	if len(brokerIDs) == 0 {
		return nil, errors.New("no broker id(s) provided for add brokers request")
	}

	brokersToAdd, err := brokerIDsFromStringSlice(brokerIDs)
	if err != nil {
		cc.log.Error(err, "failed to cast broker IDs from string slice")
		return nil, err
	}

	states := []KafkaBrokerState{KafkaBrokerAlive, KafkaBrokerNew}
	availableBrokers, err := cc.BrokersWithState(ctx, states...)
	if err != nil {
		cc.log.Error(err, "failed to retrieve list of available brokers from Cruise Control")
		return nil, err
	}

	availableBrokersMap := stringSliceToMap(availableBrokers)
	unavailableBrokerIDs := make([]string, 0, len(brokerIDs))
	for _, id := range brokerIDs {
		if _, ok := availableBrokersMap[id]; !ok {
			unavailableBrokerIDs = append(unavailableBrokerIDs, id)
		}
	}

	if len(unavailableBrokerIDs) > 0 {
		cc.log.Error(err, "there are offline brokers to be added", "broker(s)", unavailableBrokerIDs)
		return nil, errors.New("not all brokers are available which are meant to be added to the Kafka cluster")
	}

	addBrokerReq := &api.AddBrokerRequest{
		AllowCapacityEstimation: true,
		BrokerIDs:               brokersToAdd,
		DataFrom:                types.ProposalDataSourceValidWindows,
		UseReadyDefaultGoals:    true,
	}
	addBrokerResp, err := cc.client.AddBroker(ctx, addBrokerReq)
	if err != nil {
		return &Result{
			TaskID:             addBrokerResp.TaskID,
			StartedAt:          addBrokerResp.Date,
			ResponseStatusCode: addBrokerResp.StatusCode,
			RequestURL:         addBrokerResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             addBrokerResp.TaskID,
		StartedAt:          addBrokerResp.Date,
		ResponseStatusCode: addBrokerResp.StatusCode,
		RequestURL:         addBrokerResp.RequestURL,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

// RemoveBrokers requests Cruise Control to move partition replicase off from the provided brokers.
// The broker list and operation properties can be added with the use of the params argument.
func (cc *cruiseControlScaler) RemoveBrokers(ctx context.Context, brokerIDs ...string) (*Result, error) {
	if len(brokerIDs) == 0 {
		return nil, errors.New("no broker id(s) provided for remove brokers request")
	}

	clusterStateReq := api.KafkaClusterStateRequestWithDefaults()
	clusterStateResp, err := cc.client.KafkaClusterState(ctx, clusterStateReq)
	if err != nil {
		return nil, err
	}

	brokersToRemove := make([]int32, 0, len(brokerIDs))
	brokerStates := clusterStateResp.Result.KafkaBrokerState
	for _, brokerID := range brokerIDs {
		replicas, ok := brokerStates.ReplicaCountByBrokerID[brokerID]
		if !ok || replicas <= 0 {
			cc.log.Info("removing broker is skipped as it is either not available or has 0 partition replicas",
				"broker_id", brokerID, "replicas", replicas)
			continue
		}
		var bID int
		bID, err = strconv.Atoi(brokerID)
		if err != nil {
			cc.log.Error(err, "failed to cast broker ID from string to integer", "broker_id", brokerID)
			return nil, err
		}
		brokersToRemove = append(brokersToRemove, int32(bID))
	}

	if len(brokersToRemove) == 0 {
		return &Result{
			State: v1beta1.CruiseControlTaskCompleted,
		}, nil
	}

	rmBrokerReq := &api.RemoveBrokerRequest{
		AllowCapacityEstimation: true,
		BrokerIDs:               brokersToRemove,
		DataFrom:                types.ProposalDataSourceValidWindows,
		UseReadyDefaultGoals:    true,
	}
	rmBrokerResp, err := cc.client.RemoveBroker(ctx, rmBrokerReq)

	if err != nil {
		return &Result{
			TaskID:             rmBrokerResp.TaskID,
			StartedAt:          rmBrokerResp.Date,
			ResponseStatusCode: rmBrokerResp.StatusCode,
			RequestURL:         rmBrokerResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             rmBrokerResp.TaskID,
		StartedAt:          rmBrokerResp.Date,
		ResponseStatusCode: rmBrokerResp.StatusCode,
		RequestURL:         rmBrokerResp.RequestURL,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

func (cc *cruiseControlScaler) RebalanceWithParams(ctx context.Context, params map[string]string) (*Result, error) {
	rebalanceReq := &api.RebalanceRequest{
		AllowCapacityEstimation: true,
		DataFrom:                types.ProposalDataSourceValidWindows,
		UseReadyDefaultGoals:    true,
	}

	for param, pvalue := range params {
		if _, ok := rebalanceSupportedParams[param]; ok {
			switch param {
			case ParamDestbrokerIDs:
				ret, err := parseBrokerIDtoSlice(pvalue)
				if err != nil {
					return nil, err
				}
				rebalanceReq.DestinationBrokerIDs = ret
			case ParamRebalanceDisk:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				rebalanceReq.RebalanceDisk = ret
			case ParamExcludeDemoted:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				rebalanceReq.ExcludeRecentlyDemotedBrokers = ret
			case ParamExcludeRemoved:
				ret, err := strconv.ParseBool(pvalue)
				if err != nil {
					return nil, err
				}
				rebalanceReq.ExcludeRecentlyRemovedBrokers = ret
			default:
				return nil, fmt.Errorf("unsupported %s parameter: %s, supported parameters: %s", v1alpha1.OperationRebalance, param, rebalanceSupportedParams)
			}
		}
	}

	rebalanceResp, err := cc.client.Rebalance(ctx, rebalanceReq)
	if err != nil {
		return &Result{
			TaskID:             rebalanceResp.TaskID,
			StartedAt:          rebalanceResp.Date,
			ResponseStatusCode: rebalanceResp.StatusCode,
			RequestURL:         rebalanceResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             rebalanceResp.TaskID,
		StartedAt:          rebalanceResp.Date,
		ResponseStatusCode: rebalanceResp.StatusCode,
		RequestURL:         rebalanceResp.RequestURL,
		Result:             rebalanceResp.Result,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

func (cc *cruiseControlScaler) RemoveDisksWithParams(ctx context.Context, params map[string]string) (*Result, error) {
	removeReq := &api.RemoveDisksRequest{}

	for param, pvalue := range params {
		if _, ok := removeDisksSupportedParams[param]; ok {
			switch param {
			case ParamBrokerIDAndLogDirs:
				ret, err := parseBrokerIDsAndLogDirsToMap(pvalue)
				if err != nil {
					return nil, err
				}
				removeReq.BrokerIDAndLogDirs = ret
			default:
				return nil, fmt.Errorf("unsupported %s parameter: %s, supported parameters: %s", v1alpha1.OperationRemoveDisks, param, removeDisksSupportedParams)
			}
		}
	}

	if len(removeReq.BrokerIDAndLogDirs) == 0 {
		return &Result{
			State: v1beta1.CruiseControlTaskCompleted,
		}, nil
	}

	removeResp, err := cc.client.RemoveDisks(ctx, removeReq)
	if err != nil {
		return &Result{
			TaskID:             removeResp.TaskID,
			StartedAt:          removeResp.Date,
			ResponseStatusCode: removeResp.StatusCode,
			RequestURL:         removeResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             removeResp.TaskID,
		StartedAt:          removeResp.Date,
		ResponseStatusCode: removeResp.StatusCode,
		RequestURL:         removeResp.RequestURL,
		Result:             removeResp.Result,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

func parseBrokerIDsAndLogDirsToMap(brokerIDsAndLogDirs string) (map[int32][]string, error) {
	// brokerIDsAndLogDirs format: brokerID1-logDir1,brokerID2-logDir2,brokerID1-logDir3
	brokerIDLogDirMap := make(map[int32][]string)

	if len(brokerIDsAndLogDirs) == 0 {
		return brokerIDLogDirMap, nil
	}

	pairs := strings.Split(brokerIDsAndLogDirs, ",")
	for _, pair := range pairs {
		components := strings.SplitN(pair, "-", 2)
		if len(components) != 2 {
			return nil, errors.New("invalid format for brokerIDsAndLogDirs")
		}

		brokerID, err := strconv.ParseInt(components[0], 10, 32)
		if err != nil {
			return nil, errors.New("invalid broker ID")
		}

		logDir := components[1]

		// Add logDir to the corresponding brokerID's list
		brokerIDLogDirMap[int32(brokerID)] = append(brokerIDLogDirMap[int32(brokerID)], logDir)
	}

	return brokerIDLogDirMap, nil
}

func (cc *cruiseControlScaler) KafkaClusterLoad(ctx context.Context) (*api.KafkaClusterLoadResponse, error) {
	clusterLoadResp, err := cc.client.KafkaClusterLoad(ctx, api.KafkaClusterLoadRequestWithDefaults())
	if err != nil {
		return nil, err
	}
	return clusterLoadResp, nil
}

// RebalanceDisks performs a disk rebalance via Cruise Control for the provided list of brokers.
func (cc *cruiseControlScaler) RebalanceDisks(ctx context.Context, brokerIDs ...string) (*Result, error) {
	clusterLoadResp, err := cc.client.KafkaClusterLoad(ctx, api.KafkaClusterLoadRequestWithDefaults())
	if err != nil {
		return nil, err
	}

	brokerIDsMap := stringSliceToMap(brokerIDs)

	brokersWithEmptyDisks := make([]int32, 0, len(brokerIDs))
	for _, brokerStat := range clusterLoadResp.Result.Brokers {
		if _, ok := brokerIDsMap[strconv.Itoa(int(brokerStat.Broker))]; !ok {
			continue
		}
		for _, diskState := range brokerStat.DiskState {
			if diskState.NumReplicas <= 0 && !diskState.DiskMB.Dead {
				brokersWithEmptyDisks = append(brokersWithEmptyDisks, brokerStat.Broker)
			}
		}
	}

	if len(brokersWithEmptyDisks) == 0 {
		return &Result{
			State: v1beta1.CruiseControlTaskCompleted,
		}, nil
	}

	rebalanceReq := &api.RebalanceRequest{
		AllowCapacityEstimation:       true,
		DestinationBrokerIDs:          brokersWithEmptyDisks,
		DataFrom:                      types.ProposalDataSourceValidWindows,
		UseReadyDefaultGoals:          true,
		ExcludeRecentlyRemovedBrokers: true,
	}
	rebalanceResp, err := cc.client.Rebalance(ctx, rebalanceReq)
	if err != nil {
		return &Result{
			TaskID:             rebalanceResp.TaskID,
			StartedAt:          rebalanceResp.Date,
			ResponseStatusCode: rebalanceResp.StatusCode,
			RequestURL:         rebalanceResp.RequestURL,
			State:              v1beta1.CruiseControlTaskCompletedWithError,
			Err:                err,
		}, err
	}

	return &Result{
		TaskID:             rebalanceResp.TaskID,
		StartedAt:          rebalanceResp.Date,
		ResponseStatusCode: rebalanceResp.StatusCode,
		RequestURL:         rebalanceResp.RequestURL,
		State:              v1beta1.CruiseControlTaskActive,
	}, nil
}

// BrokersWithState returns a list of IDs for Kafka brokers which are available in Cruise Control
// and have one of the expected states.
func (cc *cruiseControlScaler) BrokersWithState(ctx context.Context, states ...KafkaBrokerState) ([]string, error) {
	resp, err := cc.client.KafkaClusterLoad(ctx, api.KafkaClusterLoadRequestWithDefaults())
	if err != nil {
		switch {
		case strings.Contains(err.Error(), nullPointerExceptionErrString):
			err = errors.WrapIff(err, "could not get Kafka cluster load from Cruise Control because broker storage capacity calculation has not been finished yet")
		case strings.Contains(err.Error(), notEnoughValidWindowsExceptionErrString):
			err = errors.WrapIff(err, "could not get Kafka cluster load from Cruise Control because has not been collected enough data yet")
		default:
			err = errors.WrapIff(err, "getting Kafka cluster load from Cruise Control returned an error")
		}
		return nil, err
	}

	statesMap := kafkaBrokerStatesToMap(states...)
	brokersIDs := make([]string, 0, len(resp.Result.Brokers))
	for _, broker := range resp.Result.Brokers {
		if _, ok := statesMap[broker.BrokerState]; ok {
			brokerID := strconv.Itoa(int(broker.Broker))
			brokersIDs = append(brokersIDs, brokerID)
			cc.log.Info("Kafka broker is available in Cruise Control",
				"broker id", brokerID, "state", broker.BrokerState)
		}
	}
	return brokersIDs, nil
}

// KafkaClusterState returns the state of the Kafka cluster
func (cc *cruiseControlScaler) KafkaClusterState(ctx context.Context) (*types.KafkaClusterState, error) {
	clusterStateReq := api.KafkaClusterStateRequestWithDefaults()
	clusterStateResp, err := cc.client.KafkaClusterState(ctx, clusterStateReq)
	if err != nil {
		return nil, err
	}
	return clusterStateResp.Result, nil
}

// PartitionLeadersReplicasByBroker returns the number of partition replicas for every broker in the Kafka cluster.
func (cc *cruiseControlScaler) PartitionLeadersReplicasByBroker(ctx context.Context) (brokerIDReplicaCounts map[string]int32, brokerIDLeaderCounts map[string]int32, err error) {
	clusterStateReq := api.KafkaClusterStateRequestWithDefaults()
	clusterStateResp, err := cc.client.KafkaClusterState(ctx, clusterStateReq)
	if err != nil {
		return nil, nil, err
	}
	return clusterStateResp.Result.KafkaBrokerState.ReplicaCountByBrokerID, clusterStateResp.Result.KafkaBrokerState.LeaderCountByBrokerID, nil
}

// BrokerWithLeastPartitionReplicas returns the ID of the broker which host the least partition replicas.
func (cc *cruiseControlScaler) BrokerWithLeastPartitionReplicas(ctx context.Context) (string, error) {
	var brokerWithLeastPartitionReplicas string

	brokerPartitions, err := cc.PartitionReplicasByBroker(ctx)
	if err != nil {
		cc.log.Error(err, "could not retrieve partition map for brokers")
		return brokerWithLeastPartitionReplicas, err
	}

	replicaCount := int32(math.MaxInt32)
	for brokerID, replica := range brokerPartitions {
		if replicaCount > replica {
			replicaCount = replica
			brokerWithLeastPartitionReplicas = brokerID
		}
	}
	return brokerWithLeastPartitionReplicas, nil
}

// LogDirsByBroker returns the ID of the broker which host the least partition replicas.
func (cc *cruiseControlScaler) LogDirsByBroker(ctx context.Context) (map[string]map[LogDirState][]string, error) {
	resp, err := cc.client.KafkaClusterState(ctx, api.KafkaClusterStateRequestWithDefaults())
	if err != nil {
		cc.log.Error(err, "getting Kafka cluster state from Cruise Control returned an error")
		return nil, err
	}

	newLogDirsByBroker := func() map[LogDirState][]string {
		return map[LogDirState][]string{
			LogDirStateOnline:  {},
			LogDirStateOffline: {},
		}
	}

	logDirsByBrokers := make(map[string]map[LogDirState][]string)
	for broker, onlineLogDirs := range resp.Result.KafkaBrokerState.OnlineLogDirsByBrokerID {
		logDirsByBroker, ok := logDirsByBrokers[broker]
		if !ok || logDirsByBroker == nil {
			logDirsByBroker = newLogDirsByBroker()
		}
		logDirsByBroker[LogDirStateOnline] = onlineLogDirs
		logDirsByBrokers[broker] = logDirsByBroker
	}
	for broker, offlineLogDirs := range resp.Result.KafkaBrokerState.OfflineLogDirsByBrokerID {
		logDirsByBroker, ok := logDirsByBrokers[broker]
		if !ok || logDirsByBroker == nil {
			logDirsByBroker = newLogDirsByBroker()
		}
		logDirsByBroker[LogDirStateOffline] = offlineLogDirs
		logDirsByBrokers[broker] = logDirsByBroker
	}
	return logDirsByBrokers, nil
}
