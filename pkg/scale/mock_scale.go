// Copyright © 2022 Banzai Cloud
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
	reflect "reflect"

	api "github.com/banzaicloud/go-cruise-control/pkg/api"
	types "github.com/banzaicloud/go-cruise-control/pkg/types"
	gomock "github.com/golang/mock/gomock"
)

// MockCruiseControlScaler is a mock of CruiseControlScaler interface.
type MockCruiseControlScaler struct {
	ctrl     *gomock.Controller
	recorder *MockCruiseControlScalerMockRecorder
}

// MockCruiseControlScalerMockRecorder is the mock recorder for MockCruiseControlScaler.
type MockCruiseControlScalerMockRecorder struct {
	mock *MockCruiseControlScaler
}

// NewMockCruiseControlScaler creates a new mock instance.
func NewMockCruiseControlScaler(ctrl *gomock.Controller) *MockCruiseControlScaler {
	mock := &MockCruiseControlScaler{ctrl: ctrl}
	mock.recorder = &MockCruiseControlScalerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCruiseControlScaler) EXPECT() *MockCruiseControlScalerMockRecorder {
	return m.recorder
}

// AddBrokers mocks base method.
func (m *MockCruiseControlScaler) AddBrokers(arg0 ...string) (*Result, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AddBrokers", varargs...)
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddBrokers indicates an expected call of AddBrokers.
func (mr *MockCruiseControlScalerMockRecorder) AddBrokers(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddBrokers", reflect.TypeOf((*MockCruiseControlScaler)(nil).AddBrokers), arg0...)
}

// AddBrokersWithParams mocks base method.
func (m *MockCruiseControlScaler) AddBrokersWithParams(arg0 map[string]string) (*Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddBrokersWithParams", arg0)
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddBrokersWithParams indicates an expected call of AddBrokersWithParams.
func (mr *MockCruiseControlScalerMockRecorder) AddBrokersWithParams(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddBrokersWithParams", reflect.TypeOf((*MockCruiseControlScaler)(nil).AddBrokersWithParams), arg0)
}

// BrokerWithLeastPartitionReplicas mocks base method.
func (m *MockCruiseControlScaler) BrokerWithLeastPartitionReplicas() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BrokerWithLeastPartitionReplicas")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BrokerWithLeastPartitionReplicas indicates an expected call of BrokerWithLeastPartitionReplicas.
func (mr *MockCruiseControlScalerMockRecorder) BrokerWithLeastPartitionReplicas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BrokerWithLeastPartitionReplicas", reflect.TypeOf((*MockCruiseControlScaler)(nil).BrokerWithLeastPartitionReplicas))
}

// BrokersWithState mocks base method.
func (m *MockCruiseControlScaler) BrokersWithState(arg0 ...types.BrokerState) ([]string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "BrokersWithState", varargs...)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BrokersWithState indicates an expected call of BrokersWithState.
func (mr *MockCruiseControlScalerMockRecorder) BrokersWithState(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BrokersWithState", reflect.TypeOf((*MockCruiseControlScaler)(nil).BrokersWithState), arg0...)
}

// GetKafkaClusterLoad mocks base method.
func (m *MockCruiseControlScaler) GetKafkaClusterLoad() (*api.KafkaClusterLoadResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetKafkaClusterLoad")
	ret0, _ := ret[0].(*api.KafkaClusterLoadResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetKafkaClusterLoad indicates an expected call of GetKafkaClusterLoad.
func (mr *MockCruiseControlScalerMockRecorder) GetKafkaClusterLoad() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetKafkaClusterLoad", reflect.TypeOf((*MockCruiseControlScaler)(nil).GetKafkaClusterLoad))
}

// GetKafkaClusterState mocks base method.
func (m *MockCruiseControlScaler) GetKafkaClusterState() (*types.KafkaClusterState, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetKafkaClusterState")
	ret0, _ := ret[0].(*types.KafkaClusterState)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetKafkaClusterState indicates an expected call of GetKafkaClusterState.
func (mr *MockCruiseControlScalerMockRecorder) GetKafkaClusterState() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetKafkaClusterState", reflect.TypeOf((*MockCruiseControlScaler)(nil).GetKafkaClusterState))
}

// GetNumMonitoredWin mocks base method.
func (m *MockCruiseControlScaler) GetNumMonitoredWin() (float32, types.MonitorState, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNumMonitoredWin")
	ret0, _ := ret[0].(float32)
	ret1, _ := ret[1].(types.MonitorState)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetNumMonitoredWin indicates an expected call of GetNumMonitoredWin.
func (mr *MockCruiseControlScalerMockRecorder) GetNumMonitoredWin() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNumMonitoredWin", reflect.TypeOf((*MockCruiseControlScaler)(nil).GetNumMonitoredWin))
}

// GetUserTasks mocks base method.
func (m *MockCruiseControlScaler) GetUserTasks(arg0 ...string) ([]*Result, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetUserTasks", varargs...)
	ret0, _ := ret[0].([]*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserTasks indicates an expected call of GetUserTasks.
func (mr *MockCruiseControlScalerMockRecorder) GetUserTasks(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserTasks", reflect.TypeOf((*MockCruiseControlScaler)(nil).GetUserTasks), arg0...)
}

// IsReady mocks base method.
func (m *MockCruiseControlScaler) IsReady() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsReady")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsReady indicates an expected call of IsReady.
func (mr *MockCruiseControlScalerMockRecorder) IsReady() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsReady", reflect.TypeOf((*MockCruiseControlScaler)(nil).IsReady))
}

// IsUp mocks base method.
func (m *MockCruiseControlScaler) IsUp() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsUp")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsUp indicates an expected call of IsUp.
func (mr *MockCruiseControlScalerMockRecorder) IsUp() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsUp", reflect.TypeOf((*MockCruiseControlScaler)(nil).IsUp))
}

// LogDirsByBroker mocks base method.
func (m *MockCruiseControlScaler) LogDirsByBroker() (map[string]map[LogDirState][]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LogDirsByBroker")
	ret0, _ := ret[0].(map[string]map[LogDirState][]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LogDirsByBroker indicates an expected call of LogDirsByBroker.
func (mr *MockCruiseControlScalerMockRecorder) LogDirsByBroker() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LogDirsByBroker", reflect.TypeOf((*MockCruiseControlScaler)(nil).LogDirsByBroker))
}

// PartitionReplicasByBroker mocks base method.
func (m *MockCruiseControlScaler) PartitionReplicasByBroker() (map[string]int32, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PartitionReplicasByBroker")
	ret0, _ := ret[0].(map[string]int32)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PartitionReplicasByBroker indicates an expected call of PartitionReplicasByBroker.
func (mr *MockCruiseControlScalerMockRecorder) PartitionReplicasByBroker() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PartitionReplicasByBroker", reflect.TypeOf((*MockCruiseControlScaler)(nil).PartitionReplicasByBroker))
}

// RebalanceDisks mocks base method.
func (m *MockCruiseControlScaler) RebalanceDisks(arg0 ...string) (*Result, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RebalanceDisks", varargs...)
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RebalanceDisks indicates an expected call of RebalanceDisks.
func (mr *MockCruiseControlScalerMockRecorder) RebalanceDisks(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RebalanceDisks", reflect.TypeOf((*MockCruiseControlScaler)(nil).RebalanceDisks), arg0...)
}

// RebalanceWithParams mocks base method.
func (m *MockCruiseControlScaler) RebalanceWithParams(arg0 map[string]string) (*Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RebalanceWithParams", arg0)
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RebalanceWithParams indicates an expected call of RebalanceWithParams.
func (mr *MockCruiseControlScalerMockRecorder) RebalanceWithParams(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RebalanceWithParams", reflect.TypeOf((*MockCruiseControlScaler)(nil).RebalanceWithParams), arg0)
}

// RemoveBrokers mocks base method.
func (m *MockCruiseControlScaler) RemoveBrokers(arg0 ...string) (*Result, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RemoveBrokers", varargs...)
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveBrokers indicates an expected call of RemoveBrokers.
func (mr *MockCruiseControlScalerMockRecorder) RemoveBrokers(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveBrokers", reflect.TypeOf((*MockCruiseControlScaler)(nil).RemoveBrokers), arg0...)
}

// RemoveBrokersWithParams mocks base method.
func (m *MockCruiseControlScaler) RemoveBrokersWithParams(arg0 map[string]string) (*Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveBrokersWithParams", arg0)
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveBrokersWithParams indicates an expected call of RemoveBrokersWithParams.
func (mr *MockCruiseControlScalerMockRecorder) RemoveBrokersWithParams(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveBrokersWithParams", reflect.TypeOf((*MockCruiseControlScaler)(nil).RemoveBrokersWithParams), arg0)
}

// Status mocks base method.
func (m *MockCruiseControlScaler) Status() (CruiseControlStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(CruiseControlStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Status indicates an expected call of Status.
func (mr *MockCruiseControlScalerMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockCruiseControlScaler)(nil).Status))
}

// StopExecution mocks base method.
func (m *MockCruiseControlScaler) StopExecution() (*Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopExecution")
	ret0, _ := ret[0].(*Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StopExecution indicates an expected call of StopExecution.
func (mr *MockCruiseControlScalerMockRecorder) StopExecution() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopExecution", reflect.TypeOf((*MockCruiseControlScaler)(nil).StopExecution))
}
