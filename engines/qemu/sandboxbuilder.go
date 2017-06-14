package qemuengine

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	m          sync.Mutex
	discarded  bool
	network    *network.Network
	command    []string
	machine    vm.Machine
	image      *image.Instance
	imageError error
	imageDone  <-chan struct{}
	proxies    map[string]http.Handler
	env        map[string]string
	context    *runtime.TaskContext
	engine     *engine
	monitor    runtime.Monitor
}

// newSandboxBuilder creates a new sandboxBuilder, the network and command
// properties must be set manually after calling this method.
func newSandboxBuilder(
	payload *payloadType, network *network.Network,
	c *runtime.TaskContext, e *engine, monitor runtime.Monitor,
) *sandboxBuilder {
	imageDone := make(chan struct{})
	sb := &sandboxBuilder{
		network:   network,
		command:   payload.Command,
		imageDone: imageDone,
		proxies:   make(map[string]http.Handler),
		env:       make(map[string]string),
		context:   c,
		engine:    e,
		monitor:   monitor,
	}
	if payload.Machine != nil {
		sb.machine = vm.NewMachine(payload.Machine)
	}

	// Start downloading and extracting the image
	go func() {
		var scopeSets [][]string
		var inst *image.Instance

		ctx := &fetchImageContext{c}
		ref, err := imageFetcher.NewReference(ctx, payload.Image)
		if err != nil {
			goto handleErr
		}

		// Check that task.scopes satisfies one of required scope-sets
		scopeSets = ref.Scopes()
		if !c.HasScopes(scopeSets...) {
			var options []string
			for _, scopes := range scopeSets {
				options = append(options, strings.Join(scopes, ", "))
			}
			err = runtime.NewMalformedPayloadError(
				`task.scopes must satisfy at-least one of the scope-sets: ` + strings.Join(options, " or "),
			)
			goto handleErr
		}

		debug("fetching image: %#v (if not already present)", payload.Image)
		inst, err = e.imageManager.Instance(ref.HashKey(), func(imageFile *os.File) error {
			return ref.Fetch(ctx, &fetcher.FileReseter{File: imageFile})
		})
		debug("fetched image: %#v", payload.Image)

	handleErr:
		// Transform broken reference to malformed payload
		if fetcher.IsBrokenReferenceError(err) {
			err = runtime.NewMalformedPayloadError("unable to fetch image, error:", err)
		}

		sb.m.Lock()
		// if already discarded then we don't set the image... instead we release it
		// immediately. We don't want to risk leaking an image and run out of
		// storage, as the GC won't be able to dispose it.
		if sb.discarded {
			if inst != nil {
				inst.Release()
			}
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
		return runtime.NewMalformedPayloadError("Proxy hostname: '", hostname, "'",
			" is not allowed for QEMU engine. The hostname must match: ",
			proxyNamePattern.String())
	}
	// Ensure that we're not using the magic "engine" hostname
	if hostname == "engine" {
		return runtime.NewMalformedPayloadError("Proxy hostname: 'engine' is " +
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
		return runtime.NewMalformedPayloadError("Environment variable name: '",
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

	// Create a sandbox
	s, err := newSandbox(
		sb.command, sb.env, sb.proxies, sb.machine, sb.image, sb.network,
		sb.context, sb.engine, sb.monitor,
	)
	if err != nil {
		sb.m.Unlock()
		// Free all resources
		sb.Discard()
		return nil, err
	}

	// Resources are now owned by the sandbox
	sb.network = nil
	sb.image = nil
	sb.m.Unlock()

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
