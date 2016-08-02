// +build darwin

package osxnative

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"sync"
)

type sandboxbuilder struct {
	engines.SandboxBuilderBase
	env         map[string]string
	taskPayload *payload
	context     *runtime.TaskContext
	envMutex    *sync.Mutex
	engine      *engine
}

func (s sandboxbuilder) SetEnvironmentVariable(name string, value string) error {
	s.envMutex.Lock()
	defer s.envMutex.Unlock()

	_, exists := s.env[name]
	if exists {
		return engines.ErrNamingConflict
	}

	s.context.Log("Setting \"", name, "\" environment variable to \"", value, "\" \n")
	s.env[name] = value
	return nil
}

func (s sandboxbuilder) StartSandbox() (engines.Sandbox, error) {
	env := []string{}
	for name, value := range s.env {
		env = append(env, name+"="+value)
	}

	return newSandbox(s.context, s.taskPayload, env, s.engine), nil
}
