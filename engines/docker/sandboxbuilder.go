package dockerengine

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"regexp"
	"time"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	command    []string
	image      imageType
	imageDone  chan struct{}
	imageError error
	monitor    runtime.Monitor
	e          *engine
	env        *docker.Env
	taskCtx    *runtime.TaskContext
	discarded  bool
}

func newSandboxBuilder(payload *payloadType, e *engine, monitor runtime.Monitor,
	ctx *runtime.TaskContext) *sandboxBuilder {
	sb := &sandboxBuilder{
		command:   payload.Command,
		image:     payload.Image,
		monitor:   monitor,
		e:         e,
		taskCtx:   ctx,
		env:       &docker.Env{},
		imageDone: make(chan struct{}, 1),
	}
	go sb.asyncFetchImage()
	return sb
}

func (sb *sandboxBuilder) generateDockerConfig() *docker.Config {
	debug("generating docker config for taskID: %s", sb.taskCtx.TaskID)
	conf := &docker.Config{
		Cmd:          sb.command,
		Image:        sb.image.Tag,
		Env:          *sb.env,
		AttachStdout: true,
		AttachStderr: true,
	}
	debug("config for taskID: %s, %v", sb.taskCtx.TaskID, conf)
	return conf
}

func (sb *sandboxBuilder) asyncFetchImage() {
	opts := docker.PullImageOptions{
		Repository:        sb.image.Repository,
		Tag:               sb.image.Tag,
		InactivityTimeout: 30 * time.Second,
	}

	err := sb.e.client.PullImage(opts, docker.AuthConfiguration{})
	sb.imageError = err
	close(sb.imageDone)
}

var envVarPattern = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func (sb *sandboxBuilder) SetEnvironmentVariable(name string, value string) error {
	if !envVarPattern.MatchString(name) {
		return runtime.NewMalformedPayloadError(
			"Environment variables name: '", name, "' doesn't match: ",
			envVarPattern.String(),
		)
	}
	if sb.env.Exists(name) {
		return engines.ErrNamingConflict
	}
	sb.env.Set(name, value)
	return nil
}

func (sb *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	if sb.discarded {
		return nil, engines.ErrSandboxBuilderDiscarded
	}

	<-sb.imageDone
	if sb.imageError != nil {
		return nil, runtime.NewMalformedPayloadError(
			"Could not fetch image: '", sb.image.Tag,
			"' from repository: '", sb.image.Repository,
			"'.",
		)
	}
	return newSandbox(sb)
}
