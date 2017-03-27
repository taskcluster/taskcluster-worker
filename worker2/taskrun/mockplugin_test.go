package taskrun

import (
	"fmt"

	"github.com/stretchr/testify/mock"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func check(cond bool, a ...interface{}) {
	if !cond {
		panic(fmt.Sprint(a...))
	}
}

type mockPlugin struct {
	mock.Mock
}

func (m *mockPlugin) PayloadSchema() (schema schematypes.Object) {
	args := m.Called()
	if v, ok := args.Get(0).(func() schematypes.Object); ok {
		schema = v()
	} else {
		schema = args.Get(0).(schematypes.Object)
	}
	return
}

func (m *mockPlugin) NewTaskPlugin(options plugins.TaskPluginOptions) (p plugins.TaskPlugin, err error) {
	check(options.Monitor != nil, "options.Monitor is nil in NewTaskPlugin()")
	check(options.Payload != nil, "options.Payload is nil in NewTaskPlugin()")
	check(options.TaskContext != nil, "options.TaskContext is nil in NewTaskPlugin()")
	check(options.TaskInfo != nil, "options.TaskInfo is nil in NewTaskPlugin()")
	args := m.Called(options)
	if v, ok := args.Get(0).(func(plugins.TaskPluginOptions) plugins.TaskPlugin); ok {
		p = v(options)
	} else {
		p = args.Get(0).(plugins.TaskPlugin)
	}
	if v, ok := args.Get(1).(func(plugins.TaskPluginOptions) error); ok {
		err = v(options)
	} else {
		err = args.Error(1)
	}
	return
}

func (m *mockPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) (err error) {
	check(sandboxBuilder != nil, "sandboxBuilder is nil in BuildSandbox()")
	args := m.Called(sandboxBuilder)
	if v, ok := args.Get(0).(func(engines.SandboxBuilder) error); ok {
		err = v(sandboxBuilder)
	} else {
		err = args.Error(0)
	}
	return
}

func (m *mockPlugin) Started(sandbox engines.Sandbox) (err error) {
	check(sandbox != nil, "sandbox is nil in Started()")
	args := m.Called(sandbox)
	if v, ok := args.Get(0).(func(engines.Sandbox) error); ok {
		err = v(sandbox)
	} else {
		err = args.Error(0)
	}
	return
}

func (m *mockPlugin) Stopped(result engines.ResultSet) (success bool, err error) {
	check(result != nil, "result is nil in Stopped()")
	args := m.Called(result)
	if v, ok := args.Get(0).(func(engines.ResultSet) bool); ok {
		success = v(result)
	} else {
		success = args.Bool(0)
	}
	if v, ok := args.Get(1).(func(engines.ResultSet) error); ok {
		err = v(result)
	} else {
		err = args.Error(1)
	}
	return
}

func (m *mockPlugin) Finished(success bool) (err error) {
	args := m.Called(success)
	if v, ok := args.Get(0).(func(bool) error); ok {
		err = v(success)
	} else {
		err = args.Error(0)
	}
	return
}

func (m *mockPlugin) Exception(reason runtime.ExceptionReason) (err error) {
	args := m.Called(reason)
	if v, ok := args.Get(0).(func(runtime.ExceptionReason) error); ok {
		err = v(reason)
	} else {
		err = args.Error(0)
	}
	return
}

func (m *mockPlugin) Dispose() (err error) {
	args := m.Called()
	if v, ok := args.Get(0).(func() error); ok {
		err = v()
	} else {
		err = args.Error(0)
	}
	return
}
