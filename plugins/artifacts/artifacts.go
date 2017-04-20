package artifacts

import (
	"context"
	"fmt"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"sync"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

const unknownMimetype = "application/octet-stream"

// Maximum concurrent uploads, note that we might do concurrent uploads for
// folder artifacts too, causing a total of:
//   maxUploadConcurrency * maxUploadConcurrency
const maxUploadConcurrency = 5

type pluginProvider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
	context     *runtime.TaskContext
	artifacts   []artifact
	monitor     runtime.Monitor
	failed      atomics.Bool                    // If true, Stopped() returns false
	mErrors     sync.Mutex                      // Guards errors
	errors      []runtime.MalformedPayloadError // errors to be returned from Stopped()
	nonFatalErr atomics.Bool                    // if true, Stopped() returns non-fatal error
	fatalErr    atomics.Bool                    // if true, Stopped() returns fatal error
}

func init() {
	plugins.Register("artifacts", pluginProvider{})
}

func (pluginProvider) NewPlugin(plugins.PluginOptions) (plugins.Plugin, error) {
	return &plugin{}, nil
}

func (p *plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P payload
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &P)

	return &taskPlugin{
		artifacts: P.Artifacts,
		context:   options.TaskContext,
		monitor:   options.Monitor,
	}, nil
}

func (tp *taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	debug("Extracting artifacts")
	util.SpawnWithLimit(len(tp.artifacts), maxUploadConcurrency, func(i int) {
		// Abort, if task context is cancelled
		if tp.context.Err() != nil {
			return
		}
		a := tp.artifacts[i]
		if a.Expires.IsZero() {
			a.Expires = tp.context.TaskInfo.Expires
		}
		switch a.Type {
		case typeFile:
			tp.processFile(result, a)
		case typeDirectory:
			tp.processDirectory(result, a)
		}
	})
	debug("Artifacts extracted and uploaded")

	// Find error condition
	var err error
	if len(tp.errors) > 0 {
		err = runtime.MergeMalformedPayload(tp.errors...)
	}
	if tp.nonFatalErr.Get() {
		err = runtime.ErrNonFatalInternalError
	}
	if tp.fatalErr.Get() {
		err = runtime.ErrFatalInternalError
	}
	return !tp.failed.Get(), err
}

const (
	reasonFileMissing     = "file-missing-on-worker"
	reasonInvalidResource = "invalid-resource-on-worker"
	reasonTooLarge        = "too-large-file-on-worker"
)

func (tp *taskPlugin) processFile(result engines.ResultSet, a artifact) {
	debug("extracting file from path: %s", a.Path)
	r, err := result.ExtractFile(a.Path)
	// Always close reader, if one is returned
	defer func() {
		if r != nil {
			r.Close()
		}
	}()

	// If feature isn't supported this is malformed-payload
	if err == engines.ErrFeatureNotSupported {
		e := runtime.NewMalformedPayloadError(
			"Artifact extraction is not supported in current configuration of this workerType",
		)
		tp.mErrors.Lock()
		tp.errors = append(tp.errors, e)
		tp.mErrors.Unlock()
		return
	}

	// If resource isn't found, task should fail and we print a message to task log
	if err == engines.ErrResourceNotFound {
		tp.failed.Set(true)
		if result.Success() {
			// Only complain about missing artifacts, if the task was successful
			tp.context.LogError(fmt.Sprintf("Artifact '%s' was not found.", a.Path))
		}
		tp.context.CreateErrorArtifact(runtime.ErrorArtifact{
			Name:    a.Name,
			Reason:  reasonFileMissing,
			Message: fmt.Sprintf("No file was found at path: '%s' on worker", a.Path),
			Expires: a.Expires,
		})
		return
	}

	// If malformed-payload error, then path is invalid we wrap and return
	if e, ok := runtime.IsMalformedPayloadError(err); ok {
		tp.mErrors.Lock()
		tp.errors = append(tp.errors, runtime.NewMalformedPayloadError(
			"Invalid path: '", a.Path, "' reason: ", strings.Join(e.Messages(), ";"),
		))
		tp.mErrors.Unlock()
		return
	}

	// If non-fatal internal error, we want to propagate (what else can we do)
	if err == runtime.ErrNonFatalInternalError {
		tp.nonFatalErr.Set(true) // Propagate the non-fatal error
		tp.monitor.Warn("Received ErrNonFatalInternalError from ResultSet.ExtractFile()")
		return
	}

	// If fatal internal error, we want to propagate (what else can we do)
	if err == runtime.ErrFatalInternalError {
		tp.fatalErr.Set(true) // Propagate the fatal error
		tp.monitor.Error("Received ErrFatalInternalError from ResultSet.ExtractFile()")
		return
	}

	// If we have an unhandled error
	if err != nil {
		tp.fatalErr.Set(true)
		i := tp.monitor.ReportError(err, "Unhandled error from ResultSet.ExtractFile()")
		tp.context.LogError("Failed to extract artifact unhandled error, incidentId:", i)
		return
	}

	// Guess the mimetype
	mtype := mime.TypeByExtension(filepath.Ext(a.Path))
	if mtype == "" {
		mtype = mime.TypeByExtension(filepath.Ext(a.Name))
	}
	if mtype == "" {
		mtype = unknownMimetype
	}

	// Let's upload from r
	err = tp.context.UploadS3Artifact(runtime.S3Artifact{
		Name:     a.Name,
		Mimetype: mtype,
		Stream:   r,
		Expires:  a.Expires,
	})

	if err != nil && err != context.Canceled {
		tp.nonFatalErr.Set(true)
		i := tp.monitor.ReportError(err, "Failed to upload artifact")
		tp.context.LogError("Failed to upload artifact unhandled error, incidentId:", i)
	}
}

func (tp *taskPlugin) processDirectory(result engines.ResultSet, a artifact) {
	debug("extracting directory from path: %s", a.Path)
	semaphore := make(chan struct{}, maxUploadConcurrency)
	err := result.ExtractFolder(a.Path, func(p string, r ioext.ReadSeekCloser) error {
		debug(" - Found artifact: %s in %s", p, a.Path)
		// Always close the reader
		defer r.Close()

		// Block until we can write to semaphore, then read when we're done uploading
		// This way the capacity o the semaphore channel limits concurrency.
		select {
		case semaphore <- struct{}{}:
		case <-tp.context.Done():
			return context.Canceled
		}
		defer func() {
			<-semaphore
		}()

		// Guess the mimetype
		mtype := mime.TypeByExtension(filepath.Ext(p))
		if mtype == "" {
			mtype = unknownMimetype
		}

		// Upload artifact
		debug(" - Uploading %s from %s -> %s", p, a.Path, path.Join(a.Name, p))
		uerr := tp.context.UploadS3Artifact(runtime.S3Artifact{
			Name:     path.Join(a.Name, p),
			Expires:  a.Expires,
			Stream:   r,
			Mimetype: mtype,
		})

		// If we have an upload error, that's just a internal non-fatal error.
		// We ignore the error, if TaskContext was canceled, as requests should be
		// aborted when that happens.
		if uerr != nil && tp.context.Err() != nil {
			tp.nonFatalErr.Set(true)
			i := tp.monitor.ReportError(uerr, "Failed to upload artifact")
			tp.context.LogError("Failed to upload artifact unhandled error, incidentId:", i)
		}

		// Return nil, if we're not aborted
		return tp.context.Err()
	})

	// If feature isn't supported this is malformed-payload
	if err == engines.ErrFeatureNotSupported {
		e := runtime.NewMalformedPayloadError(
			"Artifact extraction is not supported in current configuration of this workerType",
		)
		tp.mErrors.Lock()
		tp.errors = append(tp.errors, e)
		tp.mErrors.Unlock()
		return
	}

	// If resource isn't found, task should fail and we print a message to task log
	if err == engines.ErrResourceNotFound {
		tp.failed.Set(true)
		// Only complain about missing artifact folder if task is successful
		if result.Success() {
			tp.context.LogError(fmt.Sprintf("No folder was found at '%s', artifact upload failed.", a.Path))
		}
		tp.context.CreateErrorArtifact(runtime.ErrorArtifact{
			Name:    a.Name,
			Reason:  reasonFileMissing,
			Message: fmt.Sprintf("No folder was found at path: '%s' on worker", a.Path),
			Expires: a.Expires,
		})
		return
	}

	// If handler was interupted, then task was canceled or aborted...
	if err == engines.ErrHandlerInterrupt {
		tp.monitor.Debug("TaskContext cancellation interrupted artifact upload from folder")
		return
	}

	// If malformed-payload error, then path is invalid we wrap and return
	if e, ok := runtime.IsMalformedPayloadError(err); ok {
		tp.mErrors.Lock()
		tp.errors = append(tp.errors, runtime.NewMalformedPayloadError(
			"Invalid path: '", a.Path, "' reason: ", strings.Join(e.Messages(), ";"),
		))
		tp.mErrors.Unlock()
		return
	}

	// If non-fatal internal error, we want to propagate (what else can we do)
	if err == runtime.ErrNonFatalInternalError {
		tp.nonFatalErr.Set(true) // Propagate the non-fatal error
		tp.monitor.Warn("Received ErrNonFatalInternalError from ResultSet.ExtractFolder()")
		return
	}

	// If fatal internal error, we want to propagate (what else can we do)
	if err == runtime.ErrFatalInternalError {
		tp.fatalErr.Set(true) // Propagate the fatal error
		tp.monitor.Error("Received ErrFatalInternalError from ResultSet.ExtractFolder()")
		return
	}

	// If we have an unhandled error
	if err != nil {
		tp.fatalErr.Set(true)
		i := tp.monitor.ReportError(err, "Unhandled error from ResultSet.ExtractFolder()")
		tp.context.LogError("Failed to extract artifact unhandled error, incidentId:", i)
		return
	}
}
