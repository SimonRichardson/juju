// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/cmd/juju/application/refresher (interfaces: RefresherFactory,Refresher,CharmResolver,CharmRepository)
//
// Generated by this command:
//
//	mockgen -typed -package refresher -destination refresher_mock_test.go github.com/juju/juju/cmd/juju/application/refresher RefresherFactory,Refresher,CharmResolver,CharmRepository
//

// Package refresher is a generated GoMock package.
package refresher

import (
	context "context"
	reflect "reflect"

	charm "github.com/juju/juju/api/common/charm"
	base "github.com/juju/juju/core/base"
	charm0 "github.com/juju/juju/internal/charm"
	gomock "go.uber.org/mock/gomock"
)

// MockRefresherFactory is a mock of RefresherFactory interface.
type MockRefresherFactory struct {
	ctrl     *gomock.Controller
	recorder *MockRefresherFactoryMockRecorder
}

// MockRefresherFactoryMockRecorder is the mock recorder for MockRefresherFactory.
type MockRefresherFactoryMockRecorder struct {
	mock *MockRefresherFactory
}

// NewMockRefresherFactory creates a new mock instance.
func NewMockRefresherFactory(ctrl *gomock.Controller) *MockRefresherFactory {
	mock := &MockRefresherFactory{ctrl: ctrl}
	mock.recorder = &MockRefresherFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRefresherFactory) EXPECT() *MockRefresherFactoryMockRecorder {
	return m.recorder
}

// Run mocks base method.
func (m *MockRefresherFactory) Run(arg0 context.Context, arg1 RefresherConfig) (*CharmID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", arg0, arg1)
	ret0, _ := ret[0].(*CharmID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Run indicates an expected call of Run.
func (mr *MockRefresherFactoryMockRecorder) Run(arg0, arg1 any) *MockRefresherFactoryRunCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockRefresherFactory)(nil).Run), arg0, arg1)
	return &MockRefresherFactoryRunCall{Call: call}
}

// MockRefresherFactoryRunCall wrap *gomock.Call
type MockRefresherFactoryRunCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockRefresherFactoryRunCall) Return(arg0 *CharmID, arg1 error) *MockRefresherFactoryRunCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockRefresherFactoryRunCall) Do(f func(context.Context, RefresherConfig) (*CharmID, error)) *MockRefresherFactoryRunCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockRefresherFactoryRunCall) DoAndReturn(f func(context.Context, RefresherConfig) (*CharmID, error)) *MockRefresherFactoryRunCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockRefresher is a mock of Refresher interface.
type MockRefresher struct {
	ctrl     *gomock.Controller
	recorder *MockRefresherMockRecorder
}

// MockRefresherMockRecorder is the mock recorder for MockRefresher.
type MockRefresherMockRecorder struct {
	mock *MockRefresher
}

// NewMockRefresher creates a new mock instance.
func NewMockRefresher(ctrl *gomock.Controller) *MockRefresher {
	mock := &MockRefresher{ctrl: ctrl}
	mock.recorder = &MockRefresherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRefresher) EXPECT() *MockRefresherMockRecorder {
	return m.recorder
}

// Allowed mocks base method.
func (m *MockRefresher) Allowed(arg0 context.Context, arg1 RefresherConfig) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Allowed", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Allowed indicates an expected call of Allowed.
func (mr *MockRefresherMockRecorder) Allowed(arg0, arg1 any) *MockRefresherAllowedCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Allowed", reflect.TypeOf((*MockRefresher)(nil).Allowed), arg0, arg1)
	return &MockRefresherAllowedCall{Call: call}
}

// MockRefresherAllowedCall wrap *gomock.Call
type MockRefresherAllowedCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockRefresherAllowedCall) Return(arg0 bool, arg1 error) *MockRefresherAllowedCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockRefresherAllowedCall) Do(f func(context.Context, RefresherConfig) (bool, error)) *MockRefresherAllowedCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockRefresherAllowedCall) DoAndReturn(f func(context.Context, RefresherConfig) (bool, error)) *MockRefresherAllowedCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Refresh mocks base method.
func (m *MockRefresher) Refresh(arg0 context.Context) (*CharmID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Refresh", arg0)
	ret0, _ := ret[0].(*CharmID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Refresh indicates an expected call of Refresh.
func (mr *MockRefresherMockRecorder) Refresh(arg0 any) *MockRefresherRefreshCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Refresh", reflect.TypeOf((*MockRefresher)(nil).Refresh), arg0)
	return &MockRefresherRefreshCall{Call: call}
}

// MockRefresherRefreshCall wrap *gomock.Call
type MockRefresherRefreshCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockRefresherRefreshCall) Return(arg0 *CharmID, arg1 error) *MockRefresherRefreshCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockRefresherRefreshCall) Do(f func(context.Context) (*CharmID, error)) *MockRefresherRefreshCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockRefresherRefreshCall) DoAndReturn(f func(context.Context) (*CharmID, error)) *MockRefresherRefreshCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// String mocks base method.
func (m *MockRefresher) String() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "String")
	ret0, _ := ret[0].(string)
	return ret0
}

// String indicates an expected call of String.
func (mr *MockRefresherMockRecorder) String() *MockRefresherStringCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "String", reflect.TypeOf((*MockRefresher)(nil).String))
	return &MockRefresherStringCall{Call: call}
}

// MockRefresherStringCall wrap *gomock.Call
type MockRefresherStringCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockRefresherStringCall) Return(arg0 string) *MockRefresherStringCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockRefresherStringCall) Do(f func() string) *MockRefresherStringCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockRefresherStringCall) DoAndReturn(f func() string) *MockRefresherStringCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockCharmResolver is a mock of CharmResolver interface.
type MockCharmResolver struct {
	ctrl     *gomock.Controller
	recorder *MockCharmResolverMockRecorder
}

// MockCharmResolverMockRecorder is the mock recorder for MockCharmResolver.
type MockCharmResolverMockRecorder struct {
	mock *MockCharmResolver
}

// NewMockCharmResolver creates a new mock instance.
func NewMockCharmResolver(ctrl *gomock.Controller) *MockCharmResolver {
	mock := &MockCharmResolver{ctrl: ctrl}
	mock.recorder = &MockCharmResolverMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCharmResolver) EXPECT() *MockCharmResolverMockRecorder {
	return m.recorder
}

// ResolveCharm mocks base method.
func (m *MockCharmResolver) ResolveCharm(arg0 context.Context, arg1 *charm0.URL, arg2 charm.Origin, arg3 bool) (*charm0.URL, charm.Origin, []base.Base, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResolveCharm", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*charm0.URL)
	ret1, _ := ret[1].(charm.Origin)
	ret2, _ := ret[2].([]base.Base)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// ResolveCharm indicates an expected call of ResolveCharm.
func (mr *MockCharmResolverMockRecorder) ResolveCharm(arg0, arg1, arg2, arg3 any) *MockCharmResolverResolveCharmCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResolveCharm", reflect.TypeOf((*MockCharmResolver)(nil).ResolveCharm), arg0, arg1, arg2, arg3)
	return &MockCharmResolverResolveCharmCall{Call: call}
}

// MockCharmResolverResolveCharmCall wrap *gomock.Call
type MockCharmResolverResolveCharmCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockCharmResolverResolveCharmCall) Return(arg0 *charm0.URL, arg1 charm.Origin, arg2 []base.Base, arg3 error) *MockCharmResolverResolveCharmCall {
	c.Call = c.Call.Return(arg0, arg1, arg2, arg3)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockCharmResolverResolveCharmCall) Do(f func(context.Context, *charm0.URL, charm.Origin, bool) (*charm0.URL, charm.Origin, []base.Base, error)) *MockCharmResolverResolveCharmCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockCharmResolverResolveCharmCall) DoAndReturn(f func(context.Context, *charm0.URL, charm.Origin, bool) (*charm0.URL, charm.Origin, []base.Base, error)) *MockCharmResolverResolveCharmCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockCharmRepository is a mock of CharmRepository interface.
type MockCharmRepository struct {
	ctrl     *gomock.Controller
	recorder *MockCharmRepositoryMockRecorder
}

// MockCharmRepositoryMockRecorder is the mock recorder for MockCharmRepository.
type MockCharmRepositoryMockRecorder struct {
	mock *MockCharmRepository
}

// NewMockCharmRepository creates a new mock instance.
func NewMockCharmRepository(ctrl *gomock.Controller) *MockCharmRepository {
	mock := &MockCharmRepository{ctrl: ctrl}
	mock.recorder = &MockCharmRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCharmRepository) EXPECT() *MockCharmRepositoryMockRecorder {
	return m.recorder
}

// NewCharmAtPath mocks base method.
func (m *MockCharmRepository) NewCharmAtPath(arg0 string) (charm0.Charm, *charm0.URL, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewCharmAtPath", arg0)
	ret0, _ := ret[0].(charm0.Charm)
	ret1, _ := ret[1].(*charm0.URL)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// NewCharmAtPath indicates an expected call of NewCharmAtPath.
func (mr *MockCharmRepositoryMockRecorder) NewCharmAtPath(arg0 any) *MockCharmRepositoryNewCharmAtPathCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewCharmAtPath", reflect.TypeOf((*MockCharmRepository)(nil).NewCharmAtPath), arg0)
	return &MockCharmRepositoryNewCharmAtPathCall{Call: call}
}

// MockCharmRepositoryNewCharmAtPathCall wrap *gomock.Call
type MockCharmRepositoryNewCharmAtPathCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockCharmRepositoryNewCharmAtPathCall) Return(arg0 charm0.Charm, arg1 *charm0.URL, arg2 error) *MockCharmRepositoryNewCharmAtPathCall {
	c.Call = c.Call.Return(arg0, arg1, arg2)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockCharmRepositoryNewCharmAtPathCall) Do(f func(string) (charm0.Charm, *charm0.URL, error)) *MockCharmRepositoryNewCharmAtPathCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockCharmRepositoryNewCharmAtPathCall) DoAndReturn(f func(string) (charm0.Charm, *charm0.URL, error)) *MockCharmRepositoryNewCharmAtPathCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
