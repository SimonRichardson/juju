// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/state (interfaces: FilesystemAttachment,VolumeAttachment,Lifer)
//
// Generated by this command:
//
//	mockgen -typed -package storageprovisioner -destination state_mock_test.go github.com/juju/juju/state FilesystemAttachment,VolumeAttachment,Lifer
//

// Package storageprovisioner is a generated GoMock package.
package storageprovisioner

import (
	reflect "reflect"

	state "github.com/juju/juju/state"
	names "github.com/juju/names/v6"
	gomock "go.uber.org/mock/gomock"
)

// MockFilesystemAttachment is a mock of FilesystemAttachment interface.
type MockFilesystemAttachment struct {
	ctrl     *gomock.Controller
	recorder *MockFilesystemAttachmentMockRecorder
}

// MockFilesystemAttachmentMockRecorder is the mock recorder for MockFilesystemAttachment.
type MockFilesystemAttachmentMockRecorder struct {
	mock *MockFilesystemAttachment
}

// NewMockFilesystemAttachment creates a new mock instance.
func NewMockFilesystemAttachment(ctrl *gomock.Controller) *MockFilesystemAttachment {
	mock := &MockFilesystemAttachment{ctrl: ctrl}
	mock.recorder = &MockFilesystemAttachmentMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFilesystemAttachment) EXPECT() *MockFilesystemAttachmentMockRecorder {
	return m.recorder
}

// Filesystem mocks base method.
func (m *MockFilesystemAttachment) Filesystem() names.FilesystemTag {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filesystem")
	ret0, _ := ret[0].(names.FilesystemTag)
	return ret0
}

// Filesystem indicates an expected call of Filesystem.
func (mr *MockFilesystemAttachmentMockRecorder) Filesystem() *MockFilesystemAttachmentFilesystemCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filesystem", reflect.TypeOf((*MockFilesystemAttachment)(nil).Filesystem))
	return &MockFilesystemAttachmentFilesystemCall{Call: call}
}

// MockFilesystemAttachmentFilesystemCall wrap *gomock.Call
type MockFilesystemAttachmentFilesystemCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFilesystemAttachmentFilesystemCall) Return(arg0 names.FilesystemTag) *MockFilesystemAttachmentFilesystemCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFilesystemAttachmentFilesystemCall) Do(f func() names.FilesystemTag) *MockFilesystemAttachmentFilesystemCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFilesystemAttachmentFilesystemCall) DoAndReturn(f func() names.FilesystemTag) *MockFilesystemAttachmentFilesystemCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Host mocks base method.
func (m *MockFilesystemAttachment) Host() names.Tag {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Host")
	ret0, _ := ret[0].(names.Tag)
	return ret0
}

// Host indicates an expected call of Host.
func (mr *MockFilesystemAttachmentMockRecorder) Host() *MockFilesystemAttachmentHostCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Host", reflect.TypeOf((*MockFilesystemAttachment)(nil).Host))
	return &MockFilesystemAttachmentHostCall{Call: call}
}

// MockFilesystemAttachmentHostCall wrap *gomock.Call
type MockFilesystemAttachmentHostCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFilesystemAttachmentHostCall) Return(arg0 names.Tag) *MockFilesystemAttachmentHostCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFilesystemAttachmentHostCall) Do(f func() names.Tag) *MockFilesystemAttachmentHostCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFilesystemAttachmentHostCall) DoAndReturn(f func() names.Tag) *MockFilesystemAttachmentHostCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Info mocks base method.
func (m *MockFilesystemAttachment) Info() (state.FilesystemAttachmentInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Info")
	ret0, _ := ret[0].(state.FilesystemAttachmentInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Info indicates an expected call of Info.
func (mr *MockFilesystemAttachmentMockRecorder) Info() *MockFilesystemAttachmentInfoCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockFilesystemAttachment)(nil).Info))
	return &MockFilesystemAttachmentInfoCall{Call: call}
}

// MockFilesystemAttachmentInfoCall wrap *gomock.Call
type MockFilesystemAttachmentInfoCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFilesystemAttachmentInfoCall) Return(arg0 state.FilesystemAttachmentInfo, arg1 error) *MockFilesystemAttachmentInfoCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFilesystemAttachmentInfoCall) Do(f func() (state.FilesystemAttachmentInfo, error)) *MockFilesystemAttachmentInfoCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFilesystemAttachmentInfoCall) DoAndReturn(f func() (state.FilesystemAttachmentInfo, error)) *MockFilesystemAttachmentInfoCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Life mocks base method.
func (m *MockFilesystemAttachment) Life() state.Life {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Life")
	ret0, _ := ret[0].(state.Life)
	return ret0
}

// Life indicates an expected call of Life.
func (mr *MockFilesystemAttachmentMockRecorder) Life() *MockFilesystemAttachmentLifeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Life", reflect.TypeOf((*MockFilesystemAttachment)(nil).Life))
	return &MockFilesystemAttachmentLifeCall{Call: call}
}

// MockFilesystemAttachmentLifeCall wrap *gomock.Call
type MockFilesystemAttachmentLifeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFilesystemAttachmentLifeCall) Return(arg0 state.Life) *MockFilesystemAttachmentLifeCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFilesystemAttachmentLifeCall) Do(f func() state.Life) *MockFilesystemAttachmentLifeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFilesystemAttachmentLifeCall) DoAndReturn(f func() state.Life) *MockFilesystemAttachmentLifeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Params mocks base method.
func (m *MockFilesystemAttachment) Params() (state.FilesystemAttachmentParams, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Params")
	ret0, _ := ret[0].(state.FilesystemAttachmentParams)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Params indicates an expected call of Params.
func (mr *MockFilesystemAttachmentMockRecorder) Params() *MockFilesystemAttachmentParamsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Params", reflect.TypeOf((*MockFilesystemAttachment)(nil).Params))
	return &MockFilesystemAttachmentParamsCall{Call: call}
}

// MockFilesystemAttachmentParamsCall wrap *gomock.Call
type MockFilesystemAttachmentParamsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFilesystemAttachmentParamsCall) Return(arg0 state.FilesystemAttachmentParams, arg1 bool) *MockFilesystemAttachmentParamsCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFilesystemAttachmentParamsCall) Do(f func() (state.FilesystemAttachmentParams, bool)) *MockFilesystemAttachmentParamsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFilesystemAttachmentParamsCall) DoAndReturn(f func() (state.FilesystemAttachmentParams, bool)) *MockFilesystemAttachmentParamsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockVolumeAttachment is a mock of VolumeAttachment interface.
type MockVolumeAttachment struct {
	ctrl     *gomock.Controller
	recorder *MockVolumeAttachmentMockRecorder
}

// MockVolumeAttachmentMockRecorder is the mock recorder for MockVolumeAttachment.
type MockVolumeAttachmentMockRecorder struct {
	mock *MockVolumeAttachment
}

// NewMockVolumeAttachment creates a new mock instance.
func NewMockVolumeAttachment(ctrl *gomock.Controller) *MockVolumeAttachment {
	mock := &MockVolumeAttachment{ctrl: ctrl}
	mock.recorder = &MockVolumeAttachmentMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVolumeAttachment) EXPECT() *MockVolumeAttachmentMockRecorder {
	return m.recorder
}

// Host mocks base method.
func (m *MockVolumeAttachment) Host() names.Tag {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Host")
	ret0, _ := ret[0].(names.Tag)
	return ret0
}

// Host indicates an expected call of Host.
func (mr *MockVolumeAttachmentMockRecorder) Host() *MockVolumeAttachmentHostCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Host", reflect.TypeOf((*MockVolumeAttachment)(nil).Host))
	return &MockVolumeAttachmentHostCall{Call: call}
}

// MockVolumeAttachmentHostCall wrap *gomock.Call
type MockVolumeAttachmentHostCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVolumeAttachmentHostCall) Return(arg0 names.Tag) *MockVolumeAttachmentHostCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVolumeAttachmentHostCall) Do(f func() names.Tag) *MockVolumeAttachmentHostCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVolumeAttachmentHostCall) DoAndReturn(f func() names.Tag) *MockVolumeAttachmentHostCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Info mocks base method.
func (m *MockVolumeAttachment) Info() (state.VolumeAttachmentInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Info")
	ret0, _ := ret[0].(state.VolumeAttachmentInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Info indicates an expected call of Info.
func (mr *MockVolumeAttachmentMockRecorder) Info() *MockVolumeAttachmentInfoCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockVolumeAttachment)(nil).Info))
	return &MockVolumeAttachmentInfoCall{Call: call}
}

// MockVolumeAttachmentInfoCall wrap *gomock.Call
type MockVolumeAttachmentInfoCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVolumeAttachmentInfoCall) Return(arg0 state.VolumeAttachmentInfo, arg1 error) *MockVolumeAttachmentInfoCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVolumeAttachmentInfoCall) Do(f func() (state.VolumeAttachmentInfo, error)) *MockVolumeAttachmentInfoCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVolumeAttachmentInfoCall) DoAndReturn(f func() (state.VolumeAttachmentInfo, error)) *MockVolumeAttachmentInfoCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Life mocks base method.
func (m *MockVolumeAttachment) Life() state.Life {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Life")
	ret0, _ := ret[0].(state.Life)
	return ret0
}

// Life indicates an expected call of Life.
func (mr *MockVolumeAttachmentMockRecorder) Life() *MockVolumeAttachmentLifeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Life", reflect.TypeOf((*MockVolumeAttachment)(nil).Life))
	return &MockVolumeAttachmentLifeCall{Call: call}
}

// MockVolumeAttachmentLifeCall wrap *gomock.Call
type MockVolumeAttachmentLifeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVolumeAttachmentLifeCall) Return(arg0 state.Life) *MockVolumeAttachmentLifeCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVolumeAttachmentLifeCall) Do(f func() state.Life) *MockVolumeAttachmentLifeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVolumeAttachmentLifeCall) DoAndReturn(f func() state.Life) *MockVolumeAttachmentLifeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Params mocks base method.
func (m *MockVolumeAttachment) Params() (state.VolumeAttachmentParams, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Params")
	ret0, _ := ret[0].(state.VolumeAttachmentParams)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Params indicates an expected call of Params.
func (mr *MockVolumeAttachmentMockRecorder) Params() *MockVolumeAttachmentParamsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Params", reflect.TypeOf((*MockVolumeAttachment)(nil).Params))
	return &MockVolumeAttachmentParamsCall{Call: call}
}

// MockVolumeAttachmentParamsCall wrap *gomock.Call
type MockVolumeAttachmentParamsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVolumeAttachmentParamsCall) Return(arg0 state.VolumeAttachmentParams, arg1 bool) *MockVolumeAttachmentParamsCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVolumeAttachmentParamsCall) Do(f func() (state.VolumeAttachmentParams, bool)) *MockVolumeAttachmentParamsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVolumeAttachmentParamsCall) DoAndReturn(f func() (state.VolumeAttachmentParams, bool)) *MockVolumeAttachmentParamsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Volume mocks base method.
func (m *MockVolumeAttachment) Volume() names.VolumeTag {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Volume")
	ret0, _ := ret[0].(names.VolumeTag)
	return ret0
}

// Volume indicates an expected call of Volume.
func (mr *MockVolumeAttachmentMockRecorder) Volume() *MockVolumeAttachmentVolumeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Volume", reflect.TypeOf((*MockVolumeAttachment)(nil).Volume))
	return &MockVolumeAttachmentVolumeCall{Call: call}
}

// MockVolumeAttachmentVolumeCall wrap *gomock.Call
type MockVolumeAttachmentVolumeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVolumeAttachmentVolumeCall) Return(arg0 names.VolumeTag) *MockVolumeAttachmentVolumeCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVolumeAttachmentVolumeCall) Do(f func() names.VolumeTag) *MockVolumeAttachmentVolumeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVolumeAttachmentVolumeCall) DoAndReturn(f func() names.VolumeTag) *MockVolumeAttachmentVolumeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockLifer is a mock of Lifer interface.
type MockLifer struct {
	ctrl     *gomock.Controller
	recorder *MockLiferMockRecorder
}

// MockLiferMockRecorder is the mock recorder for MockLifer.
type MockLiferMockRecorder struct {
	mock *MockLifer
}

// NewMockLifer creates a new mock instance.
func NewMockLifer(ctrl *gomock.Controller) *MockLifer {
	mock := &MockLifer{ctrl: ctrl}
	mock.recorder = &MockLiferMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLifer) EXPECT() *MockLiferMockRecorder {
	return m.recorder
}

// Life mocks base method.
func (m *MockLifer) Life() state.Life {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Life")
	ret0, _ := ret[0].(state.Life)
	return ret0
}

// Life indicates an expected call of Life.
func (mr *MockLiferMockRecorder) Life() *MockLiferLifeCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Life", reflect.TypeOf((*MockLifer)(nil).Life))
	return &MockLiferLifeCall{Call: call}
}

// MockLiferLifeCall wrap *gomock.Call
type MockLiferLifeCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockLiferLifeCall) Return(arg0 state.Life) *MockLiferLifeCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockLiferLifeCall) Do(f func() state.Life) *MockLiferLifeCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockLiferLifeCall) DoAndReturn(f func() state.Life) *MockLiferLifeCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
