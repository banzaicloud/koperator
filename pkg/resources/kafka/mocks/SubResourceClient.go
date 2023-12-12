// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package mocks

import (
	"context"
	"reflect"

	"go.uber.org/mock/gomock"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockSubResourceClient is a mock of SubResourceClient interface.
type MockSubResourceClient struct {
	ctrl     *gomock.Controller
	recorder *MockSubResourceClientMockRecorder
}

// MockSubResourceClientMockRecorder is the mock recorder for MockSubResourceClient.
type MockSubResourceClientMockRecorder struct {
	mock *MockSubResourceClient
}

// NewMockSubResourceClient creates a new mock instance.
func NewMockSubResourceClient(ctrl *gomock.Controller) *MockSubResourceClient {
	mock := &MockSubResourceClient{ctrl: ctrl}
	mock.recorder = &MockSubResourceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSubResourceClient) EXPECT() *MockSubResourceClientMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockSubResourceClient) Create(arg0 context.Context, arg1, arg2 client.Object, arg3 ...client.SubResourceCreateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockSubResourceClientMockRecorder) Create(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockSubResourceClient)(nil).Create), varargs...)
}

// Get mocks base method.
func (m *MockSubResourceClient) Get(arg0 context.Context, arg1, arg2 client.Object, arg3 ...client.SubResourceGetOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Get", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *MockSubResourceClientMockRecorder) Get(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockSubResourceClient)(nil).Get), varargs...)
}

// Patch mocks base method.
func (m *MockSubResourceClient) Patch(arg0 context.Context, arg1 client.Object, arg2 client.Patch, arg3 ...client.SubResourcePatchOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Patch", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Patch indicates an expected call of Patch.
func (mr *MockSubResourceClientMockRecorder) Patch(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Patch", reflect.TypeOf((*MockSubResourceClient)(nil).Patch), varargs...)
}

// Update mocks base method.
func (m *MockSubResourceClient) Update(arg0 context.Context, arg1 client.Object, arg2 ...client.SubResourceUpdateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockSubResourceClientMockRecorder) Update(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockSubResourceClient)(nil).Update), varargs...)
}
