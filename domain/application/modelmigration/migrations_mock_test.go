// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/domain/application/modelmigration (interfaces: ImportService)
//
// Generated by this command:
//
//	mockgen -typed -package modelmigration -destination migrations_mock_test.go github.com/juju/juju/domain/application/modelmigration ImportService
//

// Package modelmigration is a generated GoMock package.
package modelmigration

import (
	context "context"
	reflect "reflect"

	application "github.com/juju/juju/core/application"
	charm "github.com/juju/juju/core/charm"
	service "github.com/juju/juju/domain/application/service"
	charm0 "github.com/juju/juju/internal/charm"
	gomock "go.uber.org/mock/gomock"
)

// MockImportService is a mock of ImportService interface.
type MockImportService struct {
	ctrl     *gomock.Controller
	recorder *MockImportServiceMockRecorder
}

// MockImportServiceMockRecorder is the mock recorder for MockImportService.
type MockImportServiceMockRecorder struct {
	mock *MockImportService
}

// NewMockImportService creates a new mock instance.
func NewMockImportService(ctrl *gomock.Controller) *MockImportService {
	mock := &MockImportService{ctrl: ctrl}
	mock.recorder = &MockImportServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockImportService) EXPECT() *MockImportServiceMockRecorder {
	return m.recorder
}

// CreateApplication mocks base method.
func (m *MockImportService) CreateApplication(arg0 context.Context, arg1 string, arg2 charm0.Charm, arg3 charm.Origin, arg4 service.AddApplicationArgs, arg5 ...service.AddUnitArg) (application.ID, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2, arg3, arg4}
	for _, a := range arg5 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateApplication", varargs...)
	ret0, _ := ret[0].(application.ID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateApplication indicates an expected call of CreateApplication.
func (mr *MockImportServiceMockRecorder) CreateApplication(arg0, arg1, arg2, arg3, arg4 any, arg5 ...any) *MockImportServiceCreateApplicationCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2, arg3, arg4}, arg5...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateApplication", reflect.TypeOf((*MockImportService)(nil).CreateApplication), varargs...)
	return &MockImportServiceCreateApplicationCall{Call: call}
}

// MockImportServiceCreateApplicationCall wrap *gomock.Call
type MockImportServiceCreateApplicationCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockImportServiceCreateApplicationCall) Return(arg0 application.ID, arg1 error) *MockImportServiceCreateApplicationCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockImportServiceCreateApplicationCall) Do(f func(context.Context, string, charm0.Charm, charm.Origin, service.AddApplicationArgs, ...service.AddUnitArg) (application.ID, error)) *MockImportServiceCreateApplicationCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockImportServiceCreateApplicationCall) DoAndReturn(f func(context.Context, string, charm0.Charm, charm.Origin, service.AddApplicationArgs, ...service.AddUnitArg) (application.ID, error)) *MockImportServiceCreateApplicationCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
