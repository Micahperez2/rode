// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/rode/rode/protodeps/grafeas/proto/v1beta1/project_go_proto (interfaces: ProjectsClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	project_go_proto "github.com/rode/rode/protodeps/grafeas/proto/v1beta1/project_go_proto"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
)

// MockGrafeasProjectsClient is a mock of ProjectsClient interface
type MockGrafeasProjectsClient struct {
	ctrl     *gomock.Controller
	recorder *MockGrafeasProjectsClientMockRecorder
}

// MockGrafeasProjectsClientMockRecorder is the mock recorder for MockGrafeasProjectsClient
type MockGrafeasProjectsClientMockRecorder struct {
	mock *MockGrafeasProjectsClient
}

// NewMockGrafeasProjectsClient creates a new mock instance
func NewMockGrafeasProjectsClient(ctrl *gomock.Controller) *MockGrafeasProjectsClient {
	mock := &MockGrafeasProjectsClient{ctrl: ctrl}
	mock.recorder = &MockGrafeasProjectsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockGrafeasProjectsClient) EXPECT() *MockGrafeasProjectsClientMockRecorder {
	return m.recorder
}

// CreateProject mocks base method
func (m *MockGrafeasProjectsClient) CreateProject(arg0 context.Context, arg1 *project_go_proto.CreateProjectRequest, arg2 ...grpc.CallOption) (*project_go_proto.Project, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateProject", varargs...)
	ret0, _ := ret[0].(*project_go_proto.Project)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateProject indicates an expected call of CreateProject
func (mr *MockGrafeasProjectsClientMockRecorder) CreateProject(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateProject", reflect.TypeOf((*MockGrafeasProjectsClient)(nil).CreateProject), varargs...)
}

// DeleteProject mocks base method
func (m *MockGrafeasProjectsClient) DeleteProject(arg0 context.Context, arg1 *project_go_proto.DeleteProjectRequest, arg2 ...grpc.CallOption) (*emptypb.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteProject", varargs...)
	ret0, _ := ret[0].(*emptypb.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteProject indicates an expected call of DeleteProject
func (mr *MockGrafeasProjectsClientMockRecorder) DeleteProject(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteProject", reflect.TypeOf((*MockGrafeasProjectsClient)(nil).DeleteProject), varargs...)
}

// GetProject mocks base method
func (m *MockGrafeasProjectsClient) GetProject(arg0 context.Context, arg1 *project_go_proto.GetProjectRequest, arg2 ...grpc.CallOption) (*project_go_proto.Project, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetProject", varargs...)
	ret0, _ := ret[0].(*project_go_proto.Project)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetProject indicates an expected call of GetProject
func (mr *MockGrafeasProjectsClientMockRecorder) GetProject(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProject", reflect.TypeOf((*MockGrafeasProjectsClient)(nil).GetProject), varargs...)
}

// ListProjects mocks base method
func (m *MockGrafeasProjectsClient) ListProjects(arg0 context.Context, arg1 *project_go_proto.ListProjectsRequest, arg2 ...grpc.CallOption) (*project_go_proto.ListProjectsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListProjects", varargs...)
	ret0, _ := ret[0].(*project_go_proto.ListProjectsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListProjects indicates an expected call of ListProjects
func (mr *MockGrafeasProjectsClientMockRecorder) ListProjects(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListProjects", reflect.TypeOf((*MockGrafeasProjectsClient)(nil).ListProjects), varargs...)
}
