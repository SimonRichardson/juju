// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/apiserver/authentication (interfaces: ExpirableStorageBakery)
//
// Generated by this command:
//
//	mockgen -typed -package mocks -destination mocks/authentication_mock.go github.com/juju/juju/apiserver/authentication ExpirableStorageBakery
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	bakery "github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	checkers "github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery/checkers"
	authentication "github.com/juju/juju/apiserver/authentication"
	gomock "go.uber.org/mock/gomock"
	macaroon "gopkg.in/macaroon.v2"
)

// MockExpirableStorageBakery is a mock of ExpirableStorageBakery interface.
type MockExpirableStorageBakery struct {
	ctrl     *gomock.Controller
	recorder *MockExpirableStorageBakeryMockRecorder
}

// MockExpirableStorageBakeryMockRecorder is the mock recorder for MockExpirableStorageBakery.
type MockExpirableStorageBakeryMockRecorder struct {
	mock *MockExpirableStorageBakery
}

// NewMockExpirableStorageBakery creates a new mock instance.
func NewMockExpirableStorageBakery(ctrl *gomock.Controller) *MockExpirableStorageBakery {
	mock := &MockExpirableStorageBakery{ctrl: ctrl}
	mock.recorder = &MockExpirableStorageBakeryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockExpirableStorageBakery) EXPECT() *MockExpirableStorageBakeryMockRecorder {
	return m.recorder
}

// Auth mocks base method.
func (m *MockExpirableStorageBakery) Auth(arg0 context.Context, arg1 ...macaroon.Slice) *bakery.AuthChecker {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Auth", varargs...)
	ret0, _ := ret[0].(*bakery.AuthChecker)
	return ret0
}

// Auth indicates an expected call of Auth.
func (mr *MockExpirableStorageBakeryMockRecorder) Auth(arg0 any, arg1 ...any) *MockExpirableStorageBakeryAuthCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Auth", reflect.TypeOf((*MockExpirableStorageBakery)(nil).Auth), varargs...)
	return &MockExpirableStorageBakeryAuthCall{Call: call}
}

// MockExpirableStorageBakeryAuthCall wrap *gomock.Call
type MockExpirableStorageBakeryAuthCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockExpirableStorageBakeryAuthCall) Return(arg0 *bakery.AuthChecker) *MockExpirableStorageBakeryAuthCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockExpirableStorageBakeryAuthCall) Do(f func(context.Context, ...macaroon.Slice) *bakery.AuthChecker) *MockExpirableStorageBakeryAuthCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockExpirableStorageBakeryAuthCall) DoAndReturn(f func(context.Context, ...macaroon.Slice) *bakery.AuthChecker) *MockExpirableStorageBakeryAuthCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// ExpireStorageAfter mocks base method.
func (m *MockExpirableStorageBakery) ExpireStorageAfter(arg0 time.Duration) (authentication.ExpirableStorageBakery, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExpireStorageAfter", arg0)
	ret0, _ := ret[0].(authentication.ExpirableStorageBakery)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExpireStorageAfter indicates an expected call of ExpireStorageAfter.
func (mr *MockExpirableStorageBakeryMockRecorder) ExpireStorageAfter(arg0 any) *MockExpirableStorageBakeryExpireStorageAfterCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExpireStorageAfter", reflect.TypeOf((*MockExpirableStorageBakery)(nil).ExpireStorageAfter), arg0)
	return &MockExpirableStorageBakeryExpireStorageAfterCall{Call: call}
}

// MockExpirableStorageBakeryExpireStorageAfterCall wrap *gomock.Call
type MockExpirableStorageBakeryExpireStorageAfterCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockExpirableStorageBakeryExpireStorageAfterCall) Return(arg0 authentication.ExpirableStorageBakery, arg1 error) *MockExpirableStorageBakeryExpireStorageAfterCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockExpirableStorageBakeryExpireStorageAfterCall) Do(f func(time.Duration) (authentication.ExpirableStorageBakery, error)) *MockExpirableStorageBakeryExpireStorageAfterCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockExpirableStorageBakeryExpireStorageAfterCall) DoAndReturn(f func(time.Duration) (authentication.ExpirableStorageBakery, error)) *MockExpirableStorageBakeryExpireStorageAfterCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// NewMacaroon mocks base method.
func (m *MockExpirableStorageBakery) NewMacaroon(arg0 context.Context, arg1 bakery.Version, arg2 []checkers.Caveat, arg3 ...bakery.Op) (*bakery.Macaroon, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "NewMacaroon", varargs...)
	ret0, _ := ret[0].(*bakery.Macaroon)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewMacaroon indicates an expected call of NewMacaroon.
func (mr *MockExpirableStorageBakeryMockRecorder) NewMacaroon(arg0, arg1, arg2 any, arg3 ...any) *MockExpirableStorageBakeryNewMacaroonCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewMacaroon", reflect.TypeOf((*MockExpirableStorageBakery)(nil).NewMacaroon), varargs...)
	return &MockExpirableStorageBakeryNewMacaroonCall{Call: call}
}

// MockExpirableStorageBakeryNewMacaroonCall wrap *gomock.Call
type MockExpirableStorageBakeryNewMacaroonCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockExpirableStorageBakeryNewMacaroonCall) Return(arg0 *bakery.Macaroon, arg1 error) *MockExpirableStorageBakeryNewMacaroonCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockExpirableStorageBakeryNewMacaroonCall) Do(f func(context.Context, bakery.Version, []checkers.Caveat, ...bakery.Op) (*bakery.Macaroon, error)) *MockExpirableStorageBakeryNewMacaroonCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockExpirableStorageBakeryNewMacaroonCall) DoAndReturn(f func(context.Context, bakery.Version, []checkers.Caveat, ...bakery.Op) (*bakery.Macaroon, error)) *MockExpirableStorageBakeryNewMacaroonCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
