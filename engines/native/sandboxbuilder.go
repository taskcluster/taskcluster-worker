package nativeengine

import (
	"regexp"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	engine  *engine
	log     *logrus.Entry
	payload payload
	context *runtime.TaskContext
	env     map[string]string
}

func (b *sandboxBuilder) fetchContext() {
	// TODO: Download, cache and extract b.payload.Context to home folder
}

var envVarPattern = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func (b *sandboxBuilder) SetEnvironmentVariable(name string, value string) error {
	if !envVarPattern.MatchString(name) {
		return engines.NewMalformedPayloadError(
			"Environment variables name: '", name, "' doesn't match: ",
			envVarPattern.String(),
		)
	}
	if _, ok := b.env[name]; ok {
		return engines.ErrNamingConflict
	}
	b.env[name] = value
	return nil
}

func (b *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	return newSandbox(b)
}
