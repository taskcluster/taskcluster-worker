package scriptengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	payload map[string]interface{}
	engine  *engine
	context *runtime.TaskContext
}

func (b *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	script := b.engine.config.Command
	cmd := exec.Command(script[0], script[1:]...)
	folder, err := b.engine.environment.TemporaryStorage.NewFolder()
	if err != nil {
		return nil, fmt.Errorf("Error creating temporary folder: %s", err)
	}
	err = os.Mkdir(filepath.Join(folder.Path(), artifactFolder), 0777)
	if err != nil {
		return nil, fmt.Errorf("Failed to create artifact folder, error: %s", err)
	}
	data, err := json.Marshal(b.payload)
	if err != nil {
		panic(fmt.Sprintf("Error serializing json payload, error: %s", err))
	}

	cmd.Dir = folder.Path()
	cmd.Stdin = bytes.NewBuffer(data)
	log := b.context.LogDrain()
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Env = formatEnv(map[string]string{
		"TASK_ID": b.context.TaskID,
		"RUN_ID":  fmt.Sprintf("%d", b.context.RunID),
	})

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Internal error invalid script: %s", err)
	}
	s := &sandbox{
		cmd:     cmd,
		folder:  folder,
		log:     b.engine.Log,
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
