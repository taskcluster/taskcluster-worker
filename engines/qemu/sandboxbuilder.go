package qemuengine

import (
	"net/http"
	"regexp"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	m          sync.Mutex
	discarded  bool
	network    *network.Network
	command    []string
	image      *image.Instance
	imageError error
	imageDone  <-chan struct{}
	proxies    map[string]http.Handler
	env        map[string]string
	context    *runtime.TaskContext
	engine     *engine
}

// newSandboxBuilder creates a new sandboxBuilder, the network and command
// properties must be set manually after calling this method.
func newSandboxBuilder(payload *qemuPayload, network *network.Network, c *runtime.TaskContext, e *engine) *sandboxBuilder {
	imageDone := make(chan struct{})
	sb := &sandboxBuilder{
		network:   network,
		command:   payload.Command,
		imageDone: imageDone,
		proxies:   make(map[string]http.Handler),
		env:       make(map[string]string),
		context:   c,
		engine:    e,
	}
	// Start downloading and extracting the image
	go func() {
		inst, err := e.imageManager.Instance("URL:"+payload.Image, image.DownloadImage(payload.Image))
		sb.m.Lock()
		// if already discarded then we don't set the image... instead we release it
		// immediately. We don't want to risk leaking an image and run out of
		// storage, as the GC won't be able to dispose it.
		if sb.discarded {
			inst.Release()
		} else {
			sb.image = inst
			sb.imageError = err
		}
		sb.m.Unlock()
		close(imageDone)
	}()
	return sb
}

var proxyNamePattern = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func (sb *sandboxBuilder) AttachProxy(hostname string, handler http.Handler) error {
	// Validate hostname against allowed patterns
	if !proxyNamePattern.MatchString(hostname) {
		return engines.NewMalformedPayloadError("Proxy hostname: '", hostname, "'",
			" is not allowed for QEMU engine. The hostname must match: ",
			proxyNamePattern.String())
	}
	// Ensure that we're not using the magic "engine" hostname
	if hostname == "engine" {
		return engines.NewMalformedPayloadError("Proxy hostname: 'engine' is " +
			"reserved for internal use (meta-data service)")
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

// envVarPattern defines allowed environment variable names
var envVarPattern = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")

func (sb *sandboxBuilder) SetEnvironmentVariable(name, value string) error {
	// Simple sanity check of environment variable names
	if !envVarPattern.MatchString(name) {
		return engines.NewMalformedPayloadError("Environment variable name: '",
			name, "' is not allowed for QEMU engine. Environment variable names",
			" must be on the form: ", envVarPattern.String())
	}

	// Acquire the lock
	sb.m.Lock()
	defer sb.m.Unlock()

	// Check if the name is already used
	if _, ok := sb.env[name]; ok {
		return engines.ErrNamingConflict
	}

	// Set the env var
	sb.env[name] = value
	return nil
}

func (sb *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	// Wait for the image downloading to be done
	<-sb.imageDone

	// If we were discarded while waiting for the image we done
	sb.m.Lock()
	if sb.discarded {
		sb.m.Unlock()
		return nil, engines.ErrSandboxBuilderDiscarded
	}
	// Otherwise, set as discarded... Whatever happens here we free the resources
	sb.discarded = true

	// If we couldn't download the image, then we're done
	if sb.imageError != nil {
		err := sb.imageError
		sb.m.Unlock()
		// Free all resources
		sb.Discard()
		return nil, err
	}

	// No more errors, etc.
	defer sb.m.Unlock()

	// Create a sandbox
	s := newSandbox(sb.command, sb.env, sb.proxies, sb.image, sb.network, sb.context, sb.engine)
	sb.network = nil
	sb.image = nil

	return s, nil
}

func (sb *sandboxBuilder) Discard() error {
	sb.m.Lock()
	defer sb.m.Unlock()
	// Mark the SandboxBuilder as discarded, so things can't be started
	sb.discarded = true

	// Let's be defensive about release it... Here we don't complain about
	// releasing a resource twice. We'll set it nil, so that shouldn't happen
	if sb.image != nil {
		sb.image.Release()
		sb.image = nil
	}
	if sb.network != nil {
		sb.network.Release()
		sb.network = nil
	}
	return nil
}
