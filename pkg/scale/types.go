// Copyright © 2022 Cisco Systems, Inc. and/or its affiliates
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

	"github.com/banzaicloud/go-cruise-control/pkg/api"
	"github.com/banzaicloud/go-cruise-control/pkg/types"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

type CruiseControlScaler interface {
	IsReady(ctx context.Context) bool
	Status(ctx context.Context) (CruiseControlStatus, error)
	UserTasks(ctx context.Context, taskIDs ...string) ([]*Result, error)
	IsUp(ctx context.Context) bool
	AddBrokers(ctx context.Context, brokerIDs ...string) (*Result, error)
	AddBrokersWithParams(ctx context.Context, params map[string]string) (*Result, error)
	RemoveBrokersWithParams(ctx context.Context, params map[string]string) (*Result, error)
	RebalanceWithParams(ctx context.Context, params map[string]string) (*Result, error)
	StopExecution(ctx context.Context) (*Result, error)
	RemoveBrokers(ctx context.Context, brokerIDs ...string) (*Result, error)
	RemoveDisksWithParams(ctx context.Context, params map[string]string) (*Result, error)
	RebalanceDisks(ctx context.Context, brokerIDs ...string) (*Result, error)
	BrokersWithState(ctx context.Context, states ...KafkaBrokerState) ([]string, error)
	KafkaClusterState(ctx context.Context) (*types.KafkaClusterState, error)
	PartitionReplicasByBroker(ctx context.Context) (map[string]int32, error)
	BrokerWithLeastPartitionReplicas(ctx context.Context) (string, error)
	LogDirsByBroker(ctx context.Context) (map[string]map[LogDirState][]string, error)
	KafkaClusterLoad(ctx context.Context) (*api.KafkaClusterLoadResponse, error)
}

type Result struct {
	TaskID             string
	StartedAt          string
	ResponseStatusCode int
	RequestURL         string
	Result             *types.OptimizationResult
	State              v1beta1.CruiseControlUserTaskState
	Err                error
}

type LogDirState int8

const (
	LogDirStateOnline LogDirState = iota
	LogDirStateOffline
)

// CruiseControlStatus struct is used to describe internal state of Cruise Control.
type CruiseControlStatus struct {
	MonitorReady  bool
	ExecutorReady bool
	AnalyzerReady bool
	ProposalReady bool
	GoalsReady    bool

	MonitoredWindows   float32
	MonitoringCoverage float64
}

// IsReady returns true if the Analyzer and Monitor components of Cruise Control are in ready state.
func (s CruiseControlStatus) IsReady() bool {
	return s.AnalyzerReady && s.MonitorReady
}

// InExecution returns true if the Executor component of Cruise Control is performing an operation which means that new
// operations cannot be started until the current has finished or the forced to be terminated.
func (s CruiseControlStatus) InExecution() bool {
	return !s.ExecutorReady
}
