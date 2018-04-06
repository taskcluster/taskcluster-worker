package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-client-go/tcpurgecache"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// Default max delay before purge-cache requests takes effect. This is in other
// words the most out of date purge-cache requests can be.
//
// Note: this is a variable as it enables tests to set it zero.
var defaultMaxPurgeCacheDelay = 3 * time.Minute

type provider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	m              sync.Mutex
	engine         engines.Engine
	environment    *runtime.Environment
	monitor        runtime.Monitor
	sharedCache    *caching.Cache
	exclusiveCache *caching.Cache
	lastPurged     time.Time
	config         config
}

type taskPlugin struct {
	plugins.TaskPluginBase
	plugin         *plugin
	monitor        runtime.Monitor
	context        *runtime.TaskContext
	payloadEntries []payloadEntry
	cacheHandles   []*caching.Handle // pointing to *cacheVolume
	cachesError    error
	cachesReady    atomics.Once
	cachesDisposed atomics.Once
}

func init() {
	plugins.Register("cache", &provider{})
}

func (p *provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (p *provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)
	if c.MaxPurgeCacheDelay == 0 {
		c.MaxPurgeCacheDelay = defaultMaxPurgeCacheDelay
	}

	// Added some sanity checks to ensure this plugin won't run without these.
	// These strings should always be specified, so panic is very appropriate.
	if options.Environment.ProvisionerID == "" {
		panic("EngineOptions.Environment.ProvisionerID is empty string, this is a contract violation")
	}
	if options.Environment.WorkerType == "" {
		panic("EngineOptions.Environment.WorkerType is empty string, this is a contract violation")
	}

	return &plugin{
		engine:         options.Engine,
		environment:    options.Environment,
		monitor:        options.Monitor,
		sharedCache:    caching.New(constructor, false, options.Environment.GarbageCollector),
		exclusiveCache: caching.New(constructor, false, options.Environment.GarbageCollector),
		lastPurged:     time.Now(),
		config:         c,
	}, nil
}

func (p *plugin) PayloadSchema() schematypes.Object {
	return schematypes.Object{
		Properties: schematypes.Properties{
			"caches": schematypes.Array{
				Items: schematypes.Object{
					Properties: schematypes.Properties{
						"name": schematypes.String{ // name in the cache plugin
							Pattern: "^[\\x20-\\x7e]{1,255}$", // printable ascii
						},
						"mountPoint": schematypes.String{},    // path for the engine
						"options":    p.engine.VolumeSchema(), // engine options
						"preload":    preloadFetcher.Schema(), // data to be preloaded
					},
					Required: []string{"mountPoint", "options"},
				},
			},
		},
	}
}

func (p *plugin) getVolume(ctx *runtime.TaskContext, options payloadEntry) (*caching.Handle, error) {
	readOnly := options.Name == ""

	var err error
	if options.Name != "" && !ctx.HasScopes([]string{cacheScope(options.Name)}) {
		err = runtime.NewMalformedPayloadError(fmt.Sprintf(
			"task.scopes must cover '%s' in-order for the task to use the '%s' cache",
			cacheScope(options.Name), options.Name,
		))
		// Resolve with err
		return nil, err
	}

	// Create a context with progress reporting capability
	progressCtx := progressContext{ctx, options.Name}

	// First we resolve the reference, if there is any
	var ref fetcher.Reference
	var refHash string
	if options.Preload != nil {
		ref, err = preloadFetcher.NewReference(&progressCtx, options.Preload)
		if err != nil {
			if fetcher.IsBrokenReferenceError(err) {
				err = runtime.NewMalformedPayloadError(fmt.Sprintf(
					"cache preloading error: %s", err.Error(),
				))
			} else {
				err = errors.Wrap(err, "failed to fetch cache preload data")
			}
			return nil, err
		}
		// Check that task has scopes for the reference
		if !ctx.HasScopes(ref.Scopes()...) {
			if options.Name != "" {
				err = runtime.NewMalformedPayloadError(fmt.Sprintf(
					"Can't pre-load cache '%s' as data that requires one of the scope-sets: %s",
					options.Name, formatScopeSetRequirements(ref.Scopes()),
				))
			} else {
				err = runtime.NewMalformedPayloadError(fmt.Sprintf(
					"Can't pre-load cache as data requires one of the scope-sets: %s",
					formatScopeSetRequirements(ref.Scopes()),
				))
			}
			// Resolve with err
			return nil, err
		}
		// Find hash of reference for use in cacheOptions...
		refHash = ref.HashKey()
	}

	// Pick a cache
	cache := p.exclusiveCache
	if readOnly {
		cache = p.sharedCache
	}

	return cache.Require(&progressCtx, cacheOptions{
		Name:               options.Name,
		Options:            options.Options,
		Preload:            options.Preload,
		ReferenceHash:      refHash,
		Reference:          ref, // Not used as part of KEY for the hash
		InitialTaskContext: ctx, // Not used as part of KEY for the hash
		Plugin:             p,   // Not used as part of KEY for the hash
	})
}

func (p *plugin) PurgeCacheAsNeeded(ctx *runtime.TaskContext) {
	// Lock for concurrent access
	p.m.Lock()
	defer p.m.Unlock()

	// Skip if purge-cache have been checked less than maxPurgeCacheDelay time ago
	// make this a configuration option, defaulting to 3 minutes.
	if time.Since(p.lastPurged) < p.config.MaxPurgeCacheDelay {
		return
	}

	// Store now() for use as lastPurged later
	requestTime := time.Now()

	// Fetch purge-cache requests since last time purged
	purgeCache := tcpurgecache.New(nil)
	if p.config.PurgeCacheBaseURL != "" {
		purgeCache.BaseURL = p.config.PurgeCacheBaseURL
	}
	purgeCache.Authenticate = false
	purgeCache.Context = ctx
	result, err := purgeCache.PurgeRequests(
		p.environment.ProvisionerID, p.environment.WorkerType,
		p.lastPurged.UTC().Format("2006-01-02T15:04:05.000Z"),
	)
	if err != nil && err == ctx.Err() {
		return
	}
	if err != nil {
		incidentID := p.monitor.ReportWarning(err, "failed to fetch list of cache to purge from purge-cache service")
		ctx.Log("WARNING: Failed to fetch list of cache to purge from purge-cache service, incidentID: ", incidentID)
		ctx.Log("If cache purges were request they are being ignored, if not purges have been request this is of no implication.")
		return
	}

	// Purge entries as instructed by request
	filter := func(resource caching.Resource) bool {
		cache := resource.(*cacheVolume)

		for _, r := range result.Requests {
			if r.WorkerType != p.environment.WorkerType || r.ProvisionerID != p.environment.ProvisionerID {
				p.monitor.ReportWarning(fmt.Errorf(
					"received a purge-cache response for a different provisionerId/workerType: %#v", r,
				))
				continue
			}
			if cache.Name == r.CacheName && cache.Created.Before(time.Time(r.Before)) {
				p.monitor.Infof("purging cache: '%s'", cache.Name)
				return true
			}
		}
		return false
	}
	// Purge cache in parallel
	util.Parallel(func() {
		p.sharedCache.Purge(filter)
	}, func() {
		p.exclusiveCache.Purge(filter)
	})

	// Update the lastPurged time
	p.lastPurged = requestTime
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P struct {
		Caches []payloadEntry `json:"caches"`
	}
	schematypes.MustValidateAndMap(p.PayloadSchema(), options.Payload, &P)

	tp := &taskPlugin{
		plugin:         p,
		monitor:        options.Monitor,
		context:        options.TaskContext,
		payloadEntries: P.Caches,
	}
	go tp.cachesReady.Do(tp.getCaches)

	return tp, nil
}

func (p *plugin) Dispose() error {
	// Purge everything from caches
	err1 := p.sharedCache.PurgeAll()
	err2 := p.exclusiveCache.PurgeAll()
	if err1 != nil {
		return errors.Wrap(err1, "unable to purge cache, disposing shared resource failed")
	}
	return errors.Wrap(err2, "unable to purge cache, disposing exclusive resource failed")
}

func (tp *taskPlugin) getCaches() {
	// ensure that purge cache requests have been fetched recently
	tp.plugin.PurgeCacheAsNeeded(tp.context)

	// For each cachePayloadEntry we call takeVolumeEntry (concurrently)
	N := len(tp.payloadEntries)
	tp.cacheHandles = make([]*caching.Handle, N)
	errs := make([]error, N)
	util.Spawn(N, func(i int) {
		tp.cacheHandles[i], errs[i] = tp.plugin.getVolume(tp.context, tp.payloadEntries[i])
	})

	// Find malformedPayloadErrors and report internal errors
	var malformedPayloadErrors []runtime.MalformedPayloadError
	for _, err := range errs {
		if e, ok := runtime.IsMalformedPayloadError(err); ok {
			malformedPayloadErrors = append(malformedPayloadErrors, e)
		} else if err != nil && tp.context.Err() == nil {
			incidentID := tp.monitor.ReportError(err, "failed to created cache volume")
			tp.context.LogError("internal error creating cache volume, incidentID:", incidentID)
			tp.cachesError = runtime.ErrNonFatalInternalError
		}
	}
	// Merge malformedPayloadErrors, if there is no other more sever error
	if tp.cachesError == nil && len(malformedPayloadErrors) > 0 {
		tp.cachesError = runtime.MergeMalformedPayload(malformedPayloadErrors...)
	}

	// If task was canceled, we use the error to signal that it was canceled
	if tp.context.Err() != nil {
		tp.cachesError = tp.context.Err()
	}

	// Release volumes, if there is an error
	if tp.cachesError != nil {
		tp.cachesDisposed.Do(func() {
			for i, handle := range tp.cacheHandles {
				if handle != nil {
					handle.Release()
				}
				tp.cacheHandles[i] = nil
			}
		})
	}
}

func (tp *taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	// Wait for volumes to be populated
	select {
	case <-tp.cachesReady.Done():
	case <-tp.context.Done():
		return nil // TODO: In some future consider return tp.context.Err()
	}

	// if there was an error, we return it
	if tp.cachesError != nil {
		return tp.cachesError
	}

	// Attach volumes to sandboxBuilder
	var internalError error
	var malformedPayloadErrors []runtime.MalformedPayloadError
	for i, entry := range tp.payloadEntries {
		volume := tp.cacheHandles[i].Resource().(*cacheVolume).Volume
		readOnly := entry.Name == ""
		err := sandboxBuilder.AttachVolume(entry.MountPoint, volume, readOnly)

		// Handle potential errors
		switch err {
		case runtime.ErrFatalInternalError:
			internalError = runtime.ErrFatalInternalError
			err = nil
		case runtime.ErrNonFatalInternalError:
			if internalError == nil {
				internalError = runtime.ErrNonFatalInternalError
			}
			err = nil
		case engines.ErrMutableMountNotSupported:
			err = runtime.NewMalformedPayloadError("this workerType doesn't support read-write caches, " +
				"omit the cache 'name' property to make the cache read-only")
		case engines.ErrImmutableMountNotSupported:
			err = runtime.NewMalformedPayloadError("this workerType doesn't support read-only caches, " +
				"you must specify a cache 'name' property to make the cache read-write")
		case engines.ErrNamingConflict:
			err = runtime.NewMalformedPayloadError(fmt.Sprintf(
				"cache mountPoint '%s' is already in use", entry.MountPoint,
			))
		case engines.ErrFeatureNotSupported:
			err = runtime.NewMalformedPayloadError("this workerType doesn't support caches")
		}
		if e, ok := runtime.IsMalformedPayloadError(err); ok {
			malformedPayloadErrors = append(malformedPayloadErrors, e)
		} else if err != nil {
			incidentID := tp.monitor.ReportError(err, "SandboxBuilder.AttachVolume() failed")
			tp.context.LogError("internal error attaching cache volume, incidentID:", incidentID)
			internalError = runtime.ErrFatalInternalError
		}
	}

	// Return internal error, if we have one
	if internalError != nil {
		return internalError
	}

	// Merge malformed payload errors
	if len(malformedPayloadErrors) > 0 {
		return runtime.MergeMalformedPayload(malformedPayloadErrors...)
	}

	return nil
}

func (tp *taskPlugin) Dispose() error {
	tp.cachesReady.Wait()
	tp.cachesDisposed.Do(func() {
		for i, handle := range tp.cacheHandles {
			if handle != nil {
				handle.Release()
			}
			tp.cacheHandles[i] = nil
		}
	})
	return nil
}
