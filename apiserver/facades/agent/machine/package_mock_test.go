// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/apiserver/facades/agent/machine (interfaces: NetworkService,MachineService,ApplicationService,StatusService,RemovalService)
//
// Generated by this command:
//
//	mockgen -typed -package machine_test -destination package_mock_test.go github.com/juju/juju/apiserver/facades/agent/machine NetworkService,MachineService,ApplicationService,StatusService,RemovalService
//

// Package machine_test is a generated GoMock package.
package machine_test

import (
	context "context"
	reflect "reflect"

	instance "github.com/juju/juju/core/instance"
	life "github.com/juju/juju/core/life"
	machine "github.com/juju/juju/core/machine"
	network "github.com/juju/juju/core/network"
	status "github.com/juju/juju/core/status"
	unit "github.com/juju/juju/core/unit"
	watcher "github.com/juju/juju/core/watcher"
	network0 "github.com/juju/juju/domain/network"
	gomock "go.uber.org/mock/gomock"
)

// MockNetworkService is a mock of NetworkService interface.
type MockNetworkService struct {
	ctrl     *gomock.Controller
	recorder *MockNetworkServiceMockRecorder
}

// MockNetworkServiceMockRecorder is the mock recorder for MockNetworkService.
type MockNetworkServiceMockRecorder struct {
	mock *MockNetworkService
}

// NewMockNetworkService creates a new mock instance.
func NewMockNetworkService(ctrl *gomock.Controller) *MockNetworkService {
	mock := &MockNetworkService{ctrl: ctrl}
	mock.recorder = &MockNetworkServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNetworkService) EXPECT() *MockNetworkServiceMockRecorder {
	return m.recorder
}

// AddSubnet mocks base method.
func (m *MockNetworkService) AddSubnet(arg0 context.Context, arg1 network.SubnetInfo) (network.Id, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddSubnet", arg0, arg1)
	ret0, _ := ret[0].(network.Id)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddSubnet indicates an expected call of AddSubnet.
func (mr *MockNetworkServiceMockRecorder) AddSubnet(arg0, arg1 any) *MockNetworkServiceAddSubnetCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSubnet", reflect.TypeOf((*MockNetworkService)(nil).AddSubnet), arg0, arg1)
	return &MockNetworkServiceAddSubnetCall{Call: call}
}

// MockNetworkServiceAddSubnetCall wrap *gomock.Call
type MockNetworkServiceAddSubnetCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockNetworkServiceAddSubnetCall) Return(arg0 network.Id, arg1 error) *MockNetworkServiceAddSubnetCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockNetworkServiceAddSubnetCall) Do(f func(context.Context, network.SubnetInfo) (network.Id, error)) *MockNetworkServiceAddSubnetCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockNetworkServiceAddSubnetCall) DoAndReturn(f func(context.Context, network.SubnetInfo) (network.Id, error)) *MockNetworkServiceAddSubnetCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetAllSpaces mocks base method.
func (m *MockNetworkService) GetAllSpaces(arg0 context.Context) (network.SpaceInfos, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllSpaces", arg0)
	ret0, _ := ret[0].(network.SpaceInfos)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllSpaces indicates an expected call of GetAllSpaces.
func (mr *MockNetworkServiceMockRecorder) GetAllSpaces(arg0 any) *MockNetworkServiceGetAllSpacesCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllSpaces", reflect.TypeOf((*MockNetworkService)(nil).GetAllSpaces), arg0)
	return &MockNetworkServiceGetAllSpacesCall{Call: call}
}

// MockNetworkServiceGetAllSpacesCall wrap *gomock.Call
type MockNetworkServiceGetAllSpacesCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockNetworkServiceGetAllSpacesCall) Return(arg0 network.SpaceInfos, arg1 error) *MockNetworkServiceGetAllSpacesCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockNetworkServiceGetAllSpacesCall) Do(f func(context.Context) (network.SpaceInfos, error)) *MockNetworkServiceGetAllSpacesCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockNetworkServiceGetAllSpacesCall) DoAndReturn(f func(context.Context) (network.SpaceInfos, error)) *MockNetworkServiceGetAllSpacesCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetAllSubnets mocks base method.
func (m *MockNetworkService) GetAllSubnets(arg0 context.Context) (network.SubnetInfos, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllSubnets", arg0)
	ret0, _ := ret[0].(network.SubnetInfos)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllSubnets indicates an expected call of GetAllSubnets.
func (mr *MockNetworkServiceMockRecorder) GetAllSubnets(arg0 any) *MockNetworkServiceGetAllSubnetsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllSubnets", reflect.TypeOf((*MockNetworkService)(nil).GetAllSubnets), arg0)
	return &MockNetworkServiceGetAllSubnetsCall{Call: call}
}

// MockNetworkServiceGetAllSubnetsCall wrap *gomock.Call
type MockNetworkServiceGetAllSubnetsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockNetworkServiceGetAllSubnetsCall) Return(arg0 network.SubnetInfos, arg1 error) *MockNetworkServiceGetAllSubnetsCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockNetworkServiceGetAllSubnetsCall) Do(f func(context.Context) (network.SubnetInfos, error)) *MockNetworkServiceGetAllSubnetsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockNetworkServiceGetAllSubnetsCall) DoAndReturn(f func(context.Context) (network.SubnetInfos, error)) *MockNetworkServiceGetAllSubnetsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetMachineNetConfig mocks base method.
func (m *MockNetworkService) SetMachineNetConfig(arg0 context.Context, arg1 machine.UUID, arg2 []network0.NetInterface) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetMachineNetConfig", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetMachineNetConfig indicates an expected call of SetMachineNetConfig.
func (mr *MockNetworkServiceMockRecorder) SetMachineNetConfig(arg0, arg1, arg2 any) *MockNetworkServiceSetMachineNetConfigCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetMachineNetConfig", reflect.TypeOf((*MockNetworkService)(nil).SetMachineNetConfig), arg0, arg1, arg2)
	return &MockNetworkServiceSetMachineNetConfigCall{Call: call}
}

// MockNetworkServiceSetMachineNetConfigCall wrap *gomock.Call
type MockNetworkServiceSetMachineNetConfigCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockNetworkServiceSetMachineNetConfigCall) Return(arg0 error) *MockNetworkServiceSetMachineNetConfigCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockNetworkServiceSetMachineNetConfigCall) Do(f func(context.Context, machine.UUID, []network0.NetInterface) error) *MockNetworkServiceSetMachineNetConfigCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockNetworkServiceSetMachineNetConfigCall) DoAndReturn(f func(context.Context, machine.UUID, []network0.NetInterface) error) *MockNetworkServiceSetMachineNetConfigCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockMachineService is a mock of MachineService interface.
type MockMachineService struct {
	ctrl     *gomock.Controller
	recorder *MockMachineServiceMockRecorder
}

// MockMachineServiceMockRecorder is the mock recorder for MockMachineService.
type MockMachineServiceMockRecorder struct {
	mock *MockMachineService
}

// NewMockMachineService creates a new mock instance.
func NewMockMachineService(ctrl *gomock.Controller) *MockMachineService {
	mock := &MockMachineService{ctrl: ctrl}
	mock.recorder = &MockMachineServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMachineService) EXPECT() *MockMachineServiceMockRecorder {
	return m.recorder
}

// GetInstanceID mocks base method.
func (m *MockMachineService) GetInstanceID(arg0 context.Context, arg1 machine.UUID) (instance.Id, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstanceID", arg0, arg1)
	ret0, _ := ret[0].(instance.Id)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInstanceID indicates an expected call of GetInstanceID.
func (mr *MockMachineServiceMockRecorder) GetInstanceID(arg0, arg1 any) *MockMachineServiceGetInstanceIDCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstanceID", reflect.TypeOf((*MockMachineService)(nil).GetInstanceID), arg0, arg1)
	return &MockMachineServiceGetInstanceIDCall{Call: call}
}

// MockMachineServiceGetInstanceIDCall wrap *gomock.Call
type MockMachineServiceGetInstanceIDCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockMachineServiceGetInstanceIDCall) Return(arg0 instance.Id, arg1 error) *MockMachineServiceGetInstanceIDCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockMachineServiceGetInstanceIDCall) Do(f func(context.Context, machine.UUID) (instance.Id, error)) *MockMachineServiceGetInstanceIDCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockMachineServiceGetInstanceIDCall) DoAndReturn(f func(context.Context, machine.UUID) (instance.Id, error)) *MockMachineServiceGetInstanceIDCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetMachineLife mocks base method.
func (m *MockMachineService) GetMachineLife(arg0 context.Context, arg1 machine.Name) (life.Value, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMachineLife", arg0, arg1)
	ret0, _ := ret[0].(life.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMachineLife indicates an expected call of GetMachineLife.
func (mr *MockMachineServiceMockRecorder) GetMachineLife(arg0, arg1 any) *MockMachineServiceGetMachineLifeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMachineLife", reflect.TypeOf((*MockMachineService)(nil).GetMachineLife), arg0, arg1)
	return &MockMachineServiceGetMachineLifeCall{Call: call}
}

// MockMachineServiceGetMachineLifeCall wrap *gomock.Call
type MockMachineServiceGetMachineLifeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockMachineServiceGetMachineLifeCall) Return(arg0 life.Value, arg1 error) *MockMachineServiceGetMachineLifeCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockMachineServiceGetMachineLifeCall) Do(f func(context.Context, machine.Name) (life.Value, error)) *MockMachineServiceGetMachineLifeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockMachineServiceGetMachineLifeCall) DoAndReturn(f func(context.Context, machine.Name) (life.Value, error)) *MockMachineServiceGetMachineLifeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetMachineUUID mocks base method.
func (m *MockMachineService) GetMachineUUID(arg0 context.Context, arg1 machine.Name) (machine.UUID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMachineUUID", arg0, arg1)
	ret0, _ := ret[0].(machine.UUID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMachineUUID indicates an expected call of GetMachineUUID.
func (mr *MockMachineServiceMockRecorder) GetMachineUUID(arg0, arg1 any) *MockMachineServiceGetMachineUUIDCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMachineUUID", reflect.TypeOf((*MockMachineService)(nil).GetMachineUUID), arg0, arg1)
	return &MockMachineServiceGetMachineUUIDCall{Call: call}
}

// MockMachineServiceGetMachineUUIDCall wrap *gomock.Call
type MockMachineServiceGetMachineUUIDCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockMachineServiceGetMachineUUIDCall) Return(arg0 machine.UUID, arg1 error) *MockMachineServiceGetMachineUUIDCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockMachineServiceGetMachineUUIDCall) Do(f func(context.Context, machine.Name) (machine.UUID, error)) *MockMachineServiceGetMachineUUIDCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockMachineServiceGetMachineUUIDCall) DoAndReturn(f func(context.Context, machine.Name) (machine.UUID, error)) *MockMachineServiceGetMachineUUIDCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// IsMachineController mocks base method.
func (m *MockMachineService) IsMachineController(arg0 context.Context, arg1 machine.Name) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsMachineController", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsMachineController indicates an expected call of IsMachineController.
func (mr *MockMachineServiceMockRecorder) IsMachineController(arg0, arg1 any) *MockMachineServiceIsMachineControllerCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsMachineController", reflect.TypeOf((*MockMachineService)(nil).IsMachineController), arg0, arg1)
	return &MockMachineServiceIsMachineControllerCall{Call: call}
}

// MockMachineServiceIsMachineControllerCall wrap *gomock.Call
type MockMachineServiceIsMachineControllerCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockMachineServiceIsMachineControllerCall) Return(arg0 bool, arg1 error) *MockMachineServiceIsMachineControllerCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockMachineServiceIsMachineControllerCall) Do(f func(context.Context, machine.Name) (bool, error)) *MockMachineServiceIsMachineControllerCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockMachineServiceIsMachineControllerCall) DoAndReturn(f func(context.Context, machine.Name) (bool, error)) *MockMachineServiceIsMachineControllerCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetMachineHostname mocks base method.
func (m *MockMachineService) SetMachineHostname(arg0 context.Context, arg1 machine.UUID, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetMachineHostname", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetMachineHostname indicates an expected call of SetMachineHostname.
func (mr *MockMachineServiceMockRecorder) SetMachineHostname(arg0, arg1, arg2 any) *MockMachineServiceSetMachineHostnameCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetMachineHostname", reflect.TypeOf((*MockMachineService)(nil).SetMachineHostname), arg0, arg1, arg2)
	return &MockMachineServiceSetMachineHostnameCall{Call: call}
}

// MockMachineServiceSetMachineHostnameCall wrap *gomock.Call
type MockMachineServiceSetMachineHostnameCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockMachineServiceSetMachineHostnameCall) Return(arg0 error) *MockMachineServiceSetMachineHostnameCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockMachineServiceSetMachineHostnameCall) Do(f func(context.Context, machine.UUID, string) error) *MockMachineServiceSetMachineHostnameCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockMachineServiceSetMachineHostnameCall) DoAndReturn(f func(context.Context, machine.UUID, string) error) *MockMachineServiceSetMachineHostnameCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// WatchMachineLife mocks base method.
func (m *MockMachineService) WatchMachineLife(arg0 context.Context, arg1 machine.Name) (watcher.Watcher[struct{}], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchMachineLife", arg0, arg1)
	ret0, _ := ret[0].(watcher.Watcher[struct{}])
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchMachineLife indicates an expected call of WatchMachineLife.
func (mr *MockMachineServiceMockRecorder) WatchMachineLife(arg0, arg1 any) *MockMachineServiceWatchMachineLifeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchMachineLife", reflect.TypeOf((*MockMachineService)(nil).WatchMachineLife), arg0, arg1)
	return &MockMachineServiceWatchMachineLifeCall{Call: call}
}

// MockMachineServiceWatchMachineLifeCall wrap *gomock.Call
type MockMachineServiceWatchMachineLifeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockMachineServiceWatchMachineLifeCall) Return(arg0 watcher.Watcher[struct{}], arg1 error) *MockMachineServiceWatchMachineLifeCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockMachineServiceWatchMachineLifeCall) Do(f func(context.Context, machine.Name) (watcher.Watcher[struct{}], error)) *MockMachineServiceWatchMachineLifeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockMachineServiceWatchMachineLifeCall) DoAndReturn(f func(context.Context, machine.Name) (watcher.Watcher[struct{}], error)) *MockMachineServiceWatchMachineLifeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockApplicationService is a mock of ApplicationService interface.
type MockApplicationService struct {
	ctrl     *gomock.Controller
	recorder *MockApplicationServiceMockRecorder
}

// MockApplicationServiceMockRecorder is the mock recorder for MockApplicationService.
type MockApplicationServiceMockRecorder struct {
	mock *MockApplicationService
}

// NewMockApplicationService creates a new mock instance.
func NewMockApplicationService(ctrl *gomock.Controller) *MockApplicationService {
	mock := &MockApplicationService{ctrl: ctrl}
	mock.recorder = &MockApplicationServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockApplicationService) EXPECT() *MockApplicationServiceMockRecorder {
	return m.recorder
}

// GetApplicationLifeByName mocks base method.
func (m *MockApplicationService) GetApplicationLifeByName(arg0 context.Context, arg1 string) (life.Value, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetApplicationLifeByName", arg0, arg1)
	ret0, _ := ret[0].(life.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetApplicationLifeByName indicates an expected call of GetApplicationLifeByName.
func (mr *MockApplicationServiceMockRecorder) GetApplicationLifeByName(arg0, arg1 any) *MockApplicationServiceGetApplicationLifeByNameCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetApplicationLifeByName", reflect.TypeOf((*MockApplicationService)(nil).GetApplicationLifeByName), arg0, arg1)
	return &MockApplicationServiceGetApplicationLifeByNameCall{Call: call}
}

// MockApplicationServiceGetApplicationLifeByNameCall wrap *gomock.Call
type MockApplicationServiceGetApplicationLifeByNameCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockApplicationServiceGetApplicationLifeByNameCall) Return(arg0 life.Value, arg1 error) *MockApplicationServiceGetApplicationLifeByNameCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockApplicationServiceGetApplicationLifeByNameCall) Do(f func(context.Context, string) (life.Value, error)) *MockApplicationServiceGetApplicationLifeByNameCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockApplicationServiceGetApplicationLifeByNameCall) DoAndReturn(f func(context.Context, string) (life.Value, error)) *MockApplicationServiceGetApplicationLifeByNameCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetUnitLife mocks base method.
func (m *MockApplicationService) GetUnitLife(arg0 context.Context, arg1 unit.Name) (life.Value, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUnitLife", arg0, arg1)
	ret0, _ := ret[0].(life.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUnitLife indicates an expected call of GetUnitLife.
func (mr *MockApplicationServiceMockRecorder) GetUnitLife(arg0, arg1 any) *MockApplicationServiceGetUnitLifeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnitLife", reflect.TypeOf((*MockApplicationService)(nil).GetUnitLife), arg0, arg1)
	return &MockApplicationServiceGetUnitLifeCall{Call: call}
}

// MockApplicationServiceGetUnitLifeCall wrap *gomock.Call
type MockApplicationServiceGetUnitLifeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockApplicationServiceGetUnitLifeCall) Return(arg0 life.Value, arg1 error) *MockApplicationServiceGetUnitLifeCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockApplicationServiceGetUnitLifeCall) Do(f func(context.Context, unit.Name) (life.Value, error)) *MockApplicationServiceGetUnitLifeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockApplicationServiceGetUnitLifeCall) DoAndReturn(f func(context.Context, unit.Name) (life.Value, error)) *MockApplicationServiceGetUnitLifeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockStatusService is a mock of StatusService interface.
type MockStatusService struct {
	ctrl     *gomock.Controller
	recorder *MockStatusServiceMockRecorder
}

// MockStatusServiceMockRecorder is the mock recorder for MockStatusService.
type MockStatusServiceMockRecorder struct {
	mock *MockStatusService
}

// NewMockStatusService creates a new mock instance.
func NewMockStatusService(ctrl *gomock.Controller) *MockStatusService {
	mock := &MockStatusService{ctrl: ctrl}
	mock.recorder = &MockStatusServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStatusService) EXPECT() *MockStatusServiceMockRecorder {
	return m.recorder
}

// SetMachineStatus mocks base method.
func (m *MockStatusService) SetMachineStatus(arg0 context.Context, arg1 machine.Name, arg2 status.StatusInfo) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetMachineStatus", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetMachineStatus indicates an expected call of SetMachineStatus.
func (mr *MockStatusServiceMockRecorder) SetMachineStatus(arg0, arg1, arg2 any) *MockStatusServiceSetMachineStatusCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetMachineStatus", reflect.TypeOf((*MockStatusService)(nil).SetMachineStatus), arg0, arg1, arg2)
	return &MockStatusServiceSetMachineStatusCall{Call: call}
}

// MockStatusServiceSetMachineStatusCall wrap *gomock.Call
type MockStatusServiceSetMachineStatusCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockStatusServiceSetMachineStatusCall) Return(arg0 error) *MockStatusServiceSetMachineStatusCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockStatusServiceSetMachineStatusCall) Do(f func(context.Context, machine.Name, status.StatusInfo) error) *MockStatusServiceSetMachineStatusCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockStatusServiceSetMachineStatusCall) DoAndReturn(f func(context.Context, machine.Name, status.StatusInfo) error) *MockStatusServiceSetMachineStatusCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockRemovalService is a mock of RemovalService interface.
type MockRemovalService struct {
	ctrl     *gomock.Controller
	recorder *MockRemovalServiceMockRecorder
}

// MockRemovalServiceMockRecorder is the mock recorder for MockRemovalService.
type MockRemovalServiceMockRecorder struct {
	mock *MockRemovalService
}

// NewMockRemovalService creates a new mock instance.
func NewMockRemovalService(ctrl *gomock.Controller) *MockRemovalService {
	mock := &MockRemovalService{ctrl: ctrl}
	mock.recorder = &MockRemovalServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRemovalService) EXPECT() *MockRemovalServiceMockRecorder {
	return m.recorder
}

// MarkMachineAsDead mocks base method.
func (m *MockRemovalService) MarkMachineAsDead(arg0 context.Context, arg1 machine.UUID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkMachineAsDead", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// MarkMachineAsDead indicates an expected call of MarkMachineAsDead.
func (mr *MockRemovalServiceMockRecorder) MarkMachineAsDead(arg0, arg1 any) *MockRemovalServiceMarkMachineAsDeadCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkMachineAsDead", reflect.TypeOf((*MockRemovalService)(nil).MarkMachineAsDead), arg0, arg1)
	return &MockRemovalServiceMarkMachineAsDeadCall{Call: call}
}

// MockRemovalServiceMarkMachineAsDeadCall wrap *gomock.Call
type MockRemovalServiceMarkMachineAsDeadCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockRemovalServiceMarkMachineAsDeadCall) Return(arg0 error) *MockRemovalServiceMarkMachineAsDeadCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockRemovalServiceMarkMachineAsDeadCall) Do(f func(context.Context, machine.UUID) error) *MockRemovalServiceMarkMachineAsDeadCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockRemovalServiceMarkMachineAsDeadCall) DoAndReturn(f func(context.Context, machine.UUID) error) *MockRemovalServiceMarkMachineAsDeadCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
