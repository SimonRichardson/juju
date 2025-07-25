// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/apiserver/facades/agent/agent (interfaces: CredentialService,AgentPasswordService)
//
// Generated by this command:
//
//	mockgen -typed -package agent -destination service_mock_test.go github.com/juju/juju/apiserver/facades/agent/agent CredentialService,AgentPasswordService
//

// Package agent is a generated GoMock package.
package agent

import (
	context "context"
	reflect "reflect"

	cloud "github.com/juju/juju/cloud"
	credential "github.com/juju/juju/core/credential"
	machine "github.com/juju/juju/core/machine"
	unit "github.com/juju/juju/core/unit"
	watcher "github.com/juju/juju/core/watcher"
	gomock "go.uber.org/mock/gomock"
)

// MockCredentialService is a mock of CredentialService interface.
type MockCredentialService struct {
	ctrl     *gomock.Controller
	recorder *MockCredentialServiceMockRecorder
}

// MockCredentialServiceMockRecorder is the mock recorder for MockCredentialService.
type MockCredentialServiceMockRecorder struct {
	mock *MockCredentialService
}

// NewMockCredentialService creates a new mock instance.
func NewMockCredentialService(ctrl *gomock.Controller) *MockCredentialService {
	mock := &MockCredentialService{ctrl: ctrl}
	mock.recorder = &MockCredentialServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCredentialService) EXPECT() *MockCredentialServiceMockRecorder {
	return m.recorder
}

// CloudCredential mocks base method.
func (m *MockCredentialService) CloudCredential(arg0 context.Context, arg1 credential.Key) (cloud.Credential, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudCredential", arg0, arg1)
	ret0, _ := ret[0].(cloud.Credential)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CloudCredential indicates an expected call of CloudCredential.
func (mr *MockCredentialServiceMockRecorder) CloudCredential(arg0, arg1 any) *MockCredentialServiceCloudCredentialCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudCredential", reflect.TypeOf((*MockCredentialService)(nil).CloudCredential), arg0, arg1)
	return &MockCredentialServiceCloudCredentialCall{Call: call}
}

// MockCredentialServiceCloudCredentialCall wrap *gomock.Call
type MockCredentialServiceCloudCredentialCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockCredentialServiceCloudCredentialCall) Return(arg0 cloud.Credential, arg1 error) *MockCredentialServiceCloudCredentialCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockCredentialServiceCloudCredentialCall) Do(f func(context.Context, credential.Key) (cloud.Credential, error)) *MockCredentialServiceCloudCredentialCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockCredentialServiceCloudCredentialCall) DoAndReturn(f func(context.Context, credential.Key) (cloud.Credential, error)) *MockCredentialServiceCloudCredentialCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// WatchCredential mocks base method.
func (m *MockCredentialService) WatchCredential(arg0 context.Context, arg1 credential.Key) (watcher.Watcher[struct{}], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchCredential", arg0, arg1)
	ret0, _ := ret[0].(watcher.Watcher[struct{}])
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchCredential indicates an expected call of WatchCredential.
func (mr *MockCredentialServiceMockRecorder) WatchCredential(arg0, arg1 any) *MockCredentialServiceWatchCredentialCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchCredential", reflect.TypeOf((*MockCredentialService)(nil).WatchCredential), arg0, arg1)
	return &MockCredentialServiceWatchCredentialCall{Call: call}
}

// MockCredentialServiceWatchCredentialCall wrap *gomock.Call
type MockCredentialServiceWatchCredentialCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockCredentialServiceWatchCredentialCall) Return(arg0 watcher.Watcher[struct{}], arg1 error) *MockCredentialServiceWatchCredentialCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockCredentialServiceWatchCredentialCall) Do(f func(context.Context, credential.Key) (watcher.Watcher[struct{}], error)) *MockCredentialServiceWatchCredentialCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockCredentialServiceWatchCredentialCall) DoAndReturn(f func(context.Context, credential.Key) (watcher.Watcher[struct{}], error)) *MockCredentialServiceWatchCredentialCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockAgentPasswordService is a mock of AgentPasswordService interface.
type MockAgentPasswordService struct {
	ctrl     *gomock.Controller
	recorder *MockAgentPasswordServiceMockRecorder
}

// MockAgentPasswordServiceMockRecorder is the mock recorder for MockAgentPasswordService.
type MockAgentPasswordServiceMockRecorder struct {
	mock *MockAgentPasswordService
}

// NewMockAgentPasswordService creates a new mock instance.
func NewMockAgentPasswordService(ctrl *gomock.Controller) *MockAgentPasswordService {
	mock := &MockAgentPasswordService{ctrl: ctrl}
	mock.recorder = &MockAgentPasswordServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAgentPasswordService) EXPECT() *MockAgentPasswordServiceMockRecorder {
	return m.recorder
}

// IsMachineController mocks base method.
func (m *MockAgentPasswordService) IsMachineController(arg0 context.Context, arg1 machine.Name) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsMachineController", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsMachineController indicates an expected call of IsMachineController.
func (mr *MockAgentPasswordServiceMockRecorder) IsMachineController(arg0, arg1 any) *MockAgentPasswordServiceIsMachineControllerCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsMachineController", reflect.TypeOf((*MockAgentPasswordService)(nil).IsMachineController), arg0, arg1)
	return &MockAgentPasswordServiceIsMachineControllerCall{Call: call}
}

// MockAgentPasswordServiceIsMachineControllerCall wrap *gomock.Call
type MockAgentPasswordServiceIsMachineControllerCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockAgentPasswordServiceIsMachineControllerCall) Return(arg0 bool, arg1 error) *MockAgentPasswordServiceIsMachineControllerCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockAgentPasswordServiceIsMachineControllerCall) Do(f func(context.Context, machine.Name) (bool, error)) *MockAgentPasswordServiceIsMachineControllerCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockAgentPasswordServiceIsMachineControllerCall) DoAndReturn(f func(context.Context, machine.Name) (bool, error)) *MockAgentPasswordServiceIsMachineControllerCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetControllerNodePassword mocks base method.
func (m *MockAgentPasswordService) SetControllerNodePassword(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetControllerNodePassword", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetControllerNodePassword indicates an expected call of SetControllerNodePassword.
func (mr *MockAgentPasswordServiceMockRecorder) SetControllerNodePassword(arg0, arg1, arg2 any) *MockAgentPasswordServiceSetControllerNodePasswordCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetControllerNodePassword", reflect.TypeOf((*MockAgentPasswordService)(nil).SetControllerNodePassword), arg0, arg1, arg2)
	return &MockAgentPasswordServiceSetControllerNodePasswordCall{Call: call}
}

// MockAgentPasswordServiceSetControllerNodePasswordCall wrap *gomock.Call
type MockAgentPasswordServiceSetControllerNodePasswordCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockAgentPasswordServiceSetControllerNodePasswordCall) Return(arg0 error) *MockAgentPasswordServiceSetControllerNodePasswordCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockAgentPasswordServiceSetControllerNodePasswordCall) Do(f func(context.Context, string, string) error) *MockAgentPasswordServiceSetControllerNodePasswordCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockAgentPasswordServiceSetControllerNodePasswordCall) DoAndReturn(f func(context.Context, string, string) error) *MockAgentPasswordServiceSetControllerNodePasswordCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetMachinePassword mocks base method.
func (m *MockAgentPasswordService) SetMachinePassword(arg0 context.Context, arg1 machine.Name, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetMachinePassword", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetMachinePassword indicates an expected call of SetMachinePassword.
func (mr *MockAgentPasswordServiceMockRecorder) SetMachinePassword(arg0, arg1, arg2 any) *MockAgentPasswordServiceSetMachinePasswordCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetMachinePassword", reflect.TypeOf((*MockAgentPasswordService)(nil).SetMachinePassword), arg0, arg1, arg2)
	return &MockAgentPasswordServiceSetMachinePasswordCall{Call: call}
}

// MockAgentPasswordServiceSetMachinePasswordCall wrap *gomock.Call
type MockAgentPasswordServiceSetMachinePasswordCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockAgentPasswordServiceSetMachinePasswordCall) Return(arg0 error) *MockAgentPasswordServiceSetMachinePasswordCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockAgentPasswordServiceSetMachinePasswordCall) Do(f func(context.Context, machine.Name, string) error) *MockAgentPasswordServiceSetMachinePasswordCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockAgentPasswordServiceSetMachinePasswordCall) DoAndReturn(f func(context.Context, machine.Name, string) error) *MockAgentPasswordServiceSetMachinePasswordCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetUnitPassword mocks base method.
func (m *MockAgentPasswordService) SetUnitPassword(arg0 context.Context, arg1 unit.Name, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetUnitPassword", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetUnitPassword indicates an expected call of SetUnitPassword.
func (mr *MockAgentPasswordServiceMockRecorder) SetUnitPassword(arg0, arg1, arg2 any) *MockAgentPasswordServiceSetUnitPasswordCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetUnitPassword", reflect.TypeOf((*MockAgentPasswordService)(nil).SetUnitPassword), arg0, arg1, arg2)
	return &MockAgentPasswordServiceSetUnitPasswordCall{Call: call}
}

// MockAgentPasswordServiceSetUnitPasswordCall wrap *gomock.Call
type MockAgentPasswordServiceSetUnitPasswordCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockAgentPasswordServiceSetUnitPasswordCall) Return(arg0 error) *MockAgentPasswordServiceSetUnitPasswordCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockAgentPasswordServiceSetUnitPasswordCall) Do(f func(context.Context, unit.Name, string) error) *MockAgentPasswordServiceSetUnitPasswordCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockAgentPasswordServiceSetUnitPasswordCall) DoAndReturn(f func(context.Context, unit.Name, string) error) *MockAgentPasswordServiceSetUnitPasswordCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
