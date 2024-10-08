// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/cmd/juju/application/bundle (interfaces: ModelExtractor)
//
// Generated by this command:
//
//	mockgen -typed -package mocks -destination mocks/modelextractor_mock.go github.com/juju/juju/cmd/juju/application/bundle ModelExtractor
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	constraints "github.com/juju/juju/core/constraints"
	params "github.com/juju/juju/rpc/params"
	gomock "go.uber.org/mock/gomock"
)

// MockModelExtractor is a mock of ModelExtractor interface.
type MockModelExtractor struct {
	ctrl     *gomock.Controller
	recorder *MockModelExtractorMockRecorder
}

// MockModelExtractorMockRecorder is the mock recorder for MockModelExtractor.
type MockModelExtractorMockRecorder struct {
	mock *MockModelExtractor
}

// NewMockModelExtractor creates a new mock instance.
func NewMockModelExtractor(ctrl *gomock.Controller) *MockModelExtractor {
	mock := &MockModelExtractor{ctrl: ctrl}
	mock.recorder = &MockModelExtractorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockModelExtractor) EXPECT() *MockModelExtractorMockRecorder {
	return m.recorder
}

// GetAnnotations mocks base method.
func (m *MockModelExtractor) GetAnnotations(arg0 context.Context, arg1 []string) ([]params.AnnotationsGetResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAnnotations", arg0, arg1)
	ret0, _ := ret[0].([]params.AnnotationsGetResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAnnotations indicates an expected call of GetAnnotations.
func (mr *MockModelExtractorMockRecorder) GetAnnotations(arg0, arg1 any) *MockModelExtractorGetAnnotationsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAnnotations", reflect.TypeOf((*MockModelExtractor)(nil).GetAnnotations), arg0, arg1)
	return &MockModelExtractorGetAnnotationsCall{Call: call}
}

// MockModelExtractorGetAnnotationsCall wrap *gomock.Call
type MockModelExtractorGetAnnotationsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockModelExtractorGetAnnotationsCall) Return(arg0 []params.AnnotationsGetResult, arg1 error) *MockModelExtractorGetAnnotationsCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockModelExtractorGetAnnotationsCall) Do(f func(context.Context, []string) ([]params.AnnotationsGetResult, error)) *MockModelExtractorGetAnnotationsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockModelExtractorGetAnnotationsCall) DoAndReturn(f func(context.Context, []string) ([]params.AnnotationsGetResult, error)) *MockModelExtractorGetAnnotationsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetConfig mocks base method.
func (m *MockModelExtractor) GetConfig(arg0 context.Context, arg1 ...string) ([]map[string]any, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetConfig", varargs...)
	ret0, _ := ret[0].([]map[string]any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConfig indicates an expected call of GetConfig.
func (mr *MockModelExtractorMockRecorder) GetConfig(arg0 any, arg1 ...any) *MockModelExtractorGetConfigCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConfig", reflect.TypeOf((*MockModelExtractor)(nil).GetConfig), varargs...)
	return &MockModelExtractorGetConfigCall{Call: call}
}

// MockModelExtractorGetConfigCall wrap *gomock.Call
type MockModelExtractorGetConfigCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockModelExtractorGetConfigCall) Return(arg0 []map[string]any, arg1 error) *MockModelExtractorGetConfigCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockModelExtractorGetConfigCall) Do(f func(context.Context, ...string) ([]map[string]any, error)) *MockModelExtractorGetConfigCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockModelExtractorGetConfigCall) DoAndReturn(f func(context.Context, ...string) ([]map[string]any, error)) *MockModelExtractorGetConfigCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetConstraints mocks base method.
func (m *MockModelExtractor) GetConstraints(arg0 context.Context, arg1 ...string) ([]constraints.Value, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetConstraints", varargs...)
	ret0, _ := ret[0].([]constraints.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConstraints indicates an expected call of GetConstraints.
func (mr *MockModelExtractorMockRecorder) GetConstraints(arg0 any, arg1 ...any) *MockModelExtractorGetConstraintsCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConstraints", reflect.TypeOf((*MockModelExtractor)(nil).GetConstraints), varargs...)
	return &MockModelExtractorGetConstraintsCall{Call: call}
}

// MockModelExtractorGetConstraintsCall wrap *gomock.Call
type MockModelExtractorGetConstraintsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockModelExtractorGetConstraintsCall) Return(arg0 []constraints.Value, arg1 error) *MockModelExtractorGetConstraintsCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockModelExtractorGetConstraintsCall) Do(f func(context.Context, ...string) ([]constraints.Value, error)) *MockModelExtractorGetConstraintsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockModelExtractorGetConstraintsCall) DoAndReturn(f func(context.Context, ...string) ([]constraints.Value, error)) *MockModelExtractorGetConstraintsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Sequences mocks base method.
func (m *MockModelExtractor) Sequences(arg0 context.Context) (map[string]int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sequences", arg0)
	ret0, _ := ret[0].(map[string]int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Sequences indicates an expected call of Sequences.
func (mr *MockModelExtractorMockRecorder) Sequences(arg0 any) *MockModelExtractorSequencesCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sequences", reflect.TypeOf((*MockModelExtractor)(nil).Sequences), arg0)
	return &MockModelExtractorSequencesCall{Call: call}
}

// MockModelExtractorSequencesCall wrap *gomock.Call
type MockModelExtractorSequencesCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockModelExtractorSequencesCall) Return(arg0 map[string]int, arg1 error) *MockModelExtractorSequencesCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockModelExtractorSequencesCall) Do(f func(context.Context) (map[string]int, error)) *MockModelExtractorSequencesCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockModelExtractorSequencesCall) DoAndReturn(f func(context.Context) (map[string]int, error)) *MockModelExtractorSequencesCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
