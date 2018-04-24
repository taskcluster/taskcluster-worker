package dockerengine

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	m           sync.Mutex
	payload     *payloadType
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
	mounts      []docker.HostMount
}

func newSandboxBuilder(payload *payloadType, e *engine, monitor runtime.Monitor,
	ctx *runtime.TaskContext) *sandboxBuilder {
	sb := &sandboxBuilder{
		payload:   payload,
		image:     payload.Image,
		monitor:   monitor,
		e:         e,
		taskCtx:   ctx,
		env:       &docker.Env{},
		proxies:   make(map[string]http.Handler),
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

// mountPointPattern is a pattern all mount-points must match. Picked to avoid
// characters that are illegal on Windows / OS X as well as Linux.
var mountPointPattern = regexp.MustCompile(`^(?:/[^/\0\\:*"<>|]+)+/$`)

func validateMountPoint(mountPoint string) error {
	// We require all mount-points to be absolute paths
	if !strings.HasPrefix(mountPoint, "/") {
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"mount-point: '%s' does not start with slash, all mount-points must be absolute",
			mountPoint,
		))
	}

	// In ExtractFolder we require paths to folders to end in slash, so for
	// consistency we require the same here.
	if !strings.HasSuffix(mountPoint, "/") {
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"mount-point: '%s' does not end with slash, all mount-points must end with "+
				"a slash to indicate a folder being mounted", mountPoint,
		))
	}

	// Restrict arbitrary characters, notably \0 will cause problems. But forbidding
	// other characters is just good preparation for future Windows / OS X support.
	// It's also a good sanity protection from evil tasks trying to trick docker
	// into doing something we don't want it to do.
	if !mountPointPattern.MatchString(mountPoint) {
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"mount-point: '%s' is not allowed for docker engine, mount-points must match: %s",
			mountPoint, mountPointPattern.String(),
		))
	}

	// For sanity we forbid mount-points that contain /./ and /../, who knows what
	// that would do to docker (which is not designed to handle evil input)
	if strings.Contains(mountPoint, "/./") || strings.Contains(mountPoint, "/../") {
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"mount-point: '%s' may not contain '/./' or '/../'", mountPoint,
		))
	}

	return nil
}

func (sb *sandboxBuilder) AttachVolume(mountPoint string, vol engines.Volume, readOnly bool) error {
	// We may assert that vol is a result from engine.NewVolume()
	v, ok := vol.(*volume)
	if !ok {
		sb.monitor.Panicf("AttachVolume() was passed volume of type: %T", vol)
	}

	// Validate mount-point
	if err := validateMountPoint(mountPoint); err != nil {
		return err
	}

	// Obtain an exclusive lock
	sb.m.Lock()
	defer sb.m.Unlock()

	// remove the last slash from mountPoint and we have target as supplied to docker
	target := mountPoint[:len(mountPoint)-1]

	// Check for naming conflicts
	for _, mount := range sb.mounts {
		// If mount-point is the same as another mount, then we have a conflict
		if target == mount.Target {
			return engines.ErrNamingConflict
		}
		// If mount-point is a strict prefix for the mount-point of an earlier
		// volume, then this volume will completely overwrite the previous one.
		// That seems bad... If these calls make from different plugins this could
		// happen intermittently depending on who calls AttachVolume() first.
		// Hence, we check if target is inside another mount-point, if as this
		// causes an error regardless of the AttachVolume() ordering.
		// The cache plugin calls AttachVolume in the order caches are given, so
		// we could loosen up this restriction with a slight risk of intermittence.
		if strings.HasPrefix(mount.Target, mountPoint) || strings.HasPrefix(target, mount.Target+"/") {
			return engines.ErrNamingConflict
		}
	}

	// Add a HostMount
	sb.mounts = append(sb.mounts, docker.HostMount{
		Target:   target,
		Source:   v.GetName(),
		Type:     "volume",
		ReadOnly: readOnly,
	})

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
