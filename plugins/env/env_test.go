package env

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"testing"
)

type mockSandboxBuilder struct {
	engines.SandboxBuilderBase
	vars map[string]string
}

func (s mockSandboxBuilder) SetEnvironmentVariable(k, v string) error {
	s.vars[k] = v
	return nil
}

func (s mockSandboxBuilder) GetEnvironmentVariable(k string) string {
	return s.vars[k]
}

func (s mockSandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	return nil, engines.ErrFeatureNotSupported
}

func TestPluginPayloadSchema(t *testing.T) {
	assert := assert.New(t)

	p := plugin{}

	schema, err := p.PayloadSchema()
	assert.Equal(err, nil, err)

	data := map[string]json.RawMessage{
		"env": json.RawMessage(`{
			"ENV1": "env1",
			"ENV2": "env2"
		}`),
	}

	parsedData, err := schema.Parse(data)
	assert.Equal(err, nil, fmt.Sprintf("%v", err))
	assert.NotEqual(parsedData, nil, "Parsed data must not be nil")

	badData := map[string]json.RawMessage{
		"env": json.RawMessage(`{
			"ENV1": 1
		}`),
	}

	_, err = schema.Parse(badData)
	assert.NotEqual(err, nil, "We should have an error here")
}

func TestTaskPluginPrepare(t *testing.T) {
	assert := assert.New(t)
	p := plugin{}
	tc, err := p.NewTaskPlugin(plugins.TaskPluginOptions{
		TaskInfo: &runtime.TaskInfo{},
		Payload:  envPayload{"ENV1": "env1", "ENV2": "env2"},
	})
	assert.Equal(err, nil, fmt.Sprintf("%v", err))

	sandboxBuilder := mockSandboxBuilder{
		engines.SandboxBuilderBase{},
		map[string]string{},
	}

	err = tc.BuildSandbox(sandboxBuilder)
	assert.Equal(err, nil, fmt.Sprintf("%v", err))

	assert.Equal(sandboxBuilder.GetEnvironmentVariable("ENV1"), "env1")
	assert.Equal(sandboxBuilder.GetEnvironmentVariable("ENV2"), "env2")
}
