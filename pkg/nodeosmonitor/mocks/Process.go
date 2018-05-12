// Code generated by mockery v1.0.0
package mocks

import context "context"
import mock "github.com/stretchr/testify/mock"
import nodeosmonitor "github.com/activeeos/nodeos-monitor/pkg/nodeosmonitor"

// Process is an autogenerated mock type for the Process type
type Process struct {
	mock.Mock
}

// Activate provides a mock function with given fields: _a0, _a1
func (_m *Process) Activate(_a0 context.Context, _a1 nodeosmonitor.ProcessFailureHandler) {
	_m.Called(_a0, _a1)
}

// Shutdown provides a mock function with given fields: _a0
func (_m *Process) Shutdown(_a0 context.Context) {
	_m.Called(_a0)
}
