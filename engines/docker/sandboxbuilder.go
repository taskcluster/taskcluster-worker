package dockerengine

import (
	"context"
	"net/http"
	"regexp"
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	m           sync.Mutex
	command     []string
	image       imageType
	imageDone   chan struct{}
	imageError  error
	monitor     runtime.Monitor
	e           *engine
	proxies     map[string]http.Handler
	env         *docker.Env
	taskCtx     *runtime.TaskContext
	discarded   bool
	cancelPull  context.CancelFunc
	imageHandle *caching.Handle
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
	// set image
	sb.image.engine = sb.e
	pctx, cancelPull := context.WithCancel(context.Background())
	sb.cancelPull = cancelPull
	go sb.asyncFetchImage(newCacheContext(pctx))
	return sb
}

func (sb *sandboxBuilder) asyncFetchImage(ctx caching.Context) {
	handle, err := sb.e.cache.Require(ctx, sb.image)
	if handle != nil {
		sb.imageHandle = handle
	}
	sb.imageError = err
	select {
	case <-sb.imageDone:
	default:
		close(sb.imageDone)
	}
}

var envVarPattern = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func (sb *sandboxBuilder) SetEnvironmentVariable(name string, value string) error {
	if !envVarPattern.MatchString(name) {
		return runtime.NewMalformedPayloadError(
			"Environment variables name: '", name, "' doesn't match: ",
			envVarPattern.String(),
		)
	}

	// Acquire the lock
	sb.m.Lock()
	defer sb.m.Unlock()

	if sb.env.Exists(name) {
		return engines.ErrNamingConflict
	}
	sb.env.Set(name, value)
	return nil
}

var proxyNamePattern = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func (sb *sandboxBuilder) AttachProxy(hostname string, handler http.Handler) error {
	// Validate hostname against allowed patterns
	if !proxyNamePattern.MatchString(hostname) {
		return runtime.NewMalformedPayloadError("Proxy hostname: '", hostname, "'",
			" is not allowed for docker engine. The hostname must match: ",
			proxyNamePattern.String())
	}

	// Acquire the lock
	sb.m.Lock()
	defer sb.m.Unlock()

	// Check that the hostname isn't already in use
	if _, ok := sb.proxies[hostname]; ok {
		return engines.ErrNamingConflict
	}

	// Otherwise set the handler
	sb.proxies[hostname] = handler
	return nil
}

func (sb *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	<-sb.imageDone
	if sb.imageError != nil {
		return nil, runtime.NewMalformedPayloadError(
			"Could not fetch image: '", sb.image.Tag,
			"' from repository: '", sb.image.Repository,
			"'.",
		)
	}
	sb.m.Lock()
	if sb.discarded {
		sb.m.Unlock()
		return nil, engines.ErrSandboxBuilderDiscarded
	}
	sb.m.Unlock()
	return newSandbox(sb)
}

func (sb *sandboxBuilder) Discard() error {
	sb.m.Lock()
	defer sb.m.Unlock()

	sb.discarded = true
	sb.cancelPull()
	// imageDone chan will be closed by asyncFetchImage
	<-sb.imageDone
	if sb.imageHandle != nil {
		sb.imageHandle.Release()
	}
	return nil
}
