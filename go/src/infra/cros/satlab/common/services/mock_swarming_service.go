// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package services

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	grpc "google.golang.org/grpc"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

// MockISwarmingService is a mock of ISwarmingService interface.
type MockISwarmingService struct {
	ctrl     *gomock.Controller
	recorder *MockISwarmingServiceMockRecorder
}

// MockISwarmingServiceMockRecorder is the mock recorder for MockISwarmingService.
type MockISwarmingServiceMockRecorder struct {
	mock *MockISwarmingService
}

// NewMockISwarmingService creates a new mock instance.
func NewMockISwarmingService(ctrl *gomock.Controller) *MockISwarmingService {
	mock := &MockISwarmingService{ctrl: ctrl}
	mock.recorder = &MockISwarmingServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockISwarmingService) EXPECT() *MockISwarmingServiceMockRecorder {
	return m.recorder
}

// CountTasks mocks base method.
func (m *MockISwarmingService) CountTasks(ctx context.Context, in *apipb.TasksCountRequest) (*apipb.TasksCount, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CountTasks", ctx, in)
	ret0, _ := ret[0].(*apipb.TasksCount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CountTasks indicates an expected call of CountTasks.
func (mr *MockISwarmingServiceMockRecorder) CountTasks(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CountTasks", reflect.TypeOf((*MockISwarmingService)(nil).CountTasks), ctx, in)
}

// GetBot mocks base method.
func (m *MockISwarmingService) GetBot(ctx context.Context, hostname string) (*apipb.BotInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBot", ctx, hostname)
	ret0, _ := ret[0].(*apipb.BotInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBot indicates an expected call of GetBot.
func (mr *MockISwarmingServiceMockRecorder) GetBot(ctx, hostname any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBot", reflect.TypeOf((*MockISwarmingService)(nil).GetBot), ctx, hostname)
}

// ListBotEvents mocks base method.
func (m *MockISwarmingService) ListBotEvents(ctx context.Context, hostname, cursor string, pageSize int) (*BotEventsIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBotEvents", ctx, hostname, cursor, pageSize)
	ret0, _ := ret[0].(*BotEventsIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListBotEvents indicates an expected call of ListBotEvents.
func (mr *MockISwarmingServiceMockRecorder) ListBotEvents(ctx, hostname, cursor, pageSize any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBotEvents", reflect.TypeOf((*MockISwarmingService)(nil).ListBotEvents), ctx, hostname, cursor, pageSize)
}

// ListBotTasks mocks base method.
func (m *MockISwarmingService) ListBotTasks(ctx context.Context, hostname, cursor string, pageSize int) (*TasksIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBotTasks", ctx, hostname, cursor, pageSize)
	ret0, _ := ret[0].(*TasksIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListBotTasks indicates an expected call of ListBotTasks.
func (mr *MockISwarmingServiceMockRecorder) ListBotTasks(ctx, hostname, cursor, pageSize any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBotTasks", reflect.TypeOf((*MockISwarmingService)(nil).ListBotTasks), ctx, hostname, cursor, pageSize)
}

// ListTasks mocks base method.
func (m *MockISwarmingService) ListTasks(ctx context.Context, in *apipb.TasksWithPerfRequest) (*apipb.TaskListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTasks", ctx, in)
	ret0, _ := ret[0].(*apipb.TaskListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTasks indicates an expected call of ListTasks.
func (mr *MockISwarmingServiceMockRecorder) ListTasks(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTasks", reflect.TypeOf((*MockISwarmingService)(nil).ListTasks), ctx, in)
}

// ListBots mocks base method.
func (m *MockISwarmingService) ListBots(ctx context.Context, in *apipb.BotsRequest) (*apipb.BotInfoListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBots", ctx, in)
	ret0, _ := ret[0].(*apipb.BotInfoListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListBots indicates an expected call of ListBots.
func (mr *MockISwarmingServiceMockRecorder) ListBots(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBots", reflect.TypeOf((*MockISwarmingService)(nil).ListBots), ctx, in)
}

// CancelTasks mocks base method
func (m *MockISwarmingService) CancelTasks(ctx context.Context, req CancelTasksRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelTasks", ctx, req)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelTasks indicates an expected call of CancelTasks
func (mr *MockISwarmingServiceMockRecorder) CancelTasks(ctx context.Context, req CancelTasksRequest) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelTasks", reflect.TypeOf((*MockISwarmingService)(nil).CancelTasks), ctx, req)
}

// MockTasksClient is a mock of TasksClient interface.
type MockTasksClient struct {
	ctrl     *gomock.Controller
	recorder *MockTasksClientMockRecorder
}

// MockTasksClientMockRecorder is the mock recorder for MockTasksClient.
type MockTasksClientMockRecorder struct {
	mock *MockTasksClient
}

// NewMockTasksClient creates a new mock instance.
func NewMockTasksClient(ctrl *gomock.Controller) *MockTasksClient {
	mock := &MockTasksClient{ctrl: ctrl}
	mock.recorder = &MockTasksClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTasksClient) EXPECT() *MockTasksClientMockRecorder {
	return m.recorder
}

// ListTasks mocks base method.
func (m *MockTasksClient) ListTasks(ctx context.Context, in *apipb.TasksWithPerfRequest, opts ...grpc.CallOption) (*apipb.TaskListResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListTasks", varargs...)
	ret0, _ := ret[0].(*apipb.TaskListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTasks indicates an expected call of ListTasks.
func (mr *MockTasksClientMockRecorder) ListTasks(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTasks", reflect.TypeOf((*MockTasksClient)(nil).ListTasks), varargs...)
}

// CountTasks mocks base method.
func (m *MockTasksClient) CountTasks(ctx context.Context, in *apipb.TasksCountRequest, opts ...grpc.CallOption) (*apipb.TasksCount, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CountTasks", varargs...)
	ret0, _ := ret[0].(*apipb.TasksCount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CountTasks indicates an expected call of CountTasks.
func (mr *MockTasksClientMockRecorder) CountTasks(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CountTasks", reflect.TypeOf((*MockTasksClient)(nil).CountTasks), varargs...)
}
