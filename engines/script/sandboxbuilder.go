package scriptengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	payload map[string]interface{}
	engine  *engine
	context *runtime.TaskContext
	monitor runtime.Monitor
}

func (b *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	script := b.engine.config.Command
	cmd := exec.Command(script[0], script[1:]...)
	folder, err := b.engine.environment.TemporaryStorage.NewFolder()
	if err != nil {
		return nil, errors.Wrap(err, "Error creating temporary folder")
	}
	err = os.Mkdir(filepath.Join(folder.Path(), artifactFolder), 0777)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create artifact folder")
	}
	data, err := json.Marshal(b.payload)
	if err != nil {
		panic(errors.Wrap(err, "Error serializing json payload"))
	}

	cmd.Dir = folder.Path()
	cmd.Stdin = bytes.NewBuffer(data)
	cmd.Stdout = b.context.LogDrain()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(errors.Wrap(err, "failed to created cmd.StderrPipe()")) // should never happen
	}
	cmd.Env = formatEnv(map[string]string{
		"TASK_ID": b.context.TaskID,
		"RUN_ID":  fmt.Sprintf("%d", b.context.RunID),
	})

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "Internal error invalid script")
	}
	s := &sandbox{
		cmd:     cmd,
		stderr:  stderr,
		folder:  folder,
		monitor: b.monitor,
		context: b.context,
		engine:  b.engine,
		done:    make(chan struct{}),
	}
	go s.run()
	return s, nil
}

func (b *sandboxBuilder) Discard() error {
	return nil
}

func formatEnv(env map[string]string) []string {
	e := []string{}
	for k, v := range env {
		e = append(e, fmt.Sprintf("%s=%s", k, v))
	}
	return e
}
