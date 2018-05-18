package artifacts

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/clearsign"

	"github.com/pkg/errors"
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
	environment *runtime.Environment
	privateKey  *openpgp.Entity // nil, if COT is disabled
}

type taskPlugin struct {
	plugins.TaskPluginBase
	plugin       *plugin
	context      *runtime.TaskContext
	artifacts    []artifact
	createCOT    bool
	certifiedLog bool
	uploaded     map[string][]byte // Map from artifact to sha256 hash
	mUploaded    sync.Mutex
	monitor      runtime.Monitor
	failed       atomics.Bool                     // If true, Stopped() returns false
	mErrors      sync.Mutex                       // Guards errors
	errors       []*runtime.MalformedPayloadError // errors to be returned from Stopped()
	nonFatalErr  atomics.Bool                     // if true, Stopped() returns non-fatal error
	fatalErr     atomics.Bool                     // if true, Stopped() returns fatal error
}

func init() {
	plugins.Register("artifacts", pluginProvider{})
}

func (pluginProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	var key *openpgp.Entity
	if c.PrivateKey != "" {
		keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(c.PrivateKey))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to load private key")
		}
		if len(keyring) != 1 {
			return nil, fmt.Errorf("Expected exactly one private key, found: %d", len(keyring))
		}
		key = keyring[0]
	}

	return &plugin{
		environment: options.Environment,
		privateKey:  key,
	}, nil
}

func (p *plugin) PayloadSchema() schematypes.Object {
	schema := schematypes.Object{
		Properties: schematypes.Properties{
			"artifacts": artifactSchema,
		},
	}
	if p.privateKey != nil {
		schema.Properties["chainOfTrust"] = schematypes.Boolean{
			Title: "Create chain-of-trust Certificate",
			Description: util.Markdown(`
				Generate a 'public/chainOfTrust.json.asc' artifact with signed hashes
				of the artifacts generated from this task.
			`),
		}
		schema.Properties["certifiedLog"] = schematypes.Boolean{
			Title: "Create Certified Log",
			Description: util.Markdown(`
				Default log artifact is not covered by 'public/chainOfTrust.json.asc',
				if this is set to 'true' an artifact 'public/logs/certified.log' will
				be created and covered by chain-of-trust certificate.
			`),
		}
	}
	return schema
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P payload
	schematypes.MustValidateAndMap(p.PayloadSchema(), options.Payload, &P)

	return &taskPlugin{
		plugin:       p,
		artifacts:    P.Artifacts,
		createCOT:    p.privateKey != nil && P.CreateCOT,
		certifiedLog: p.privateKey != nil && P.CertifiedLog,
		uploaded:     make(map[string][]byte),
		context:      options.TaskContext,
		monitor:      options.Monitor,
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

func (tp *taskPlugin) hashArtifact(name string, r io.ReadSeeker) error {
	// Skip if no COT is to be generated
	if !tp.createCOT {
		return nil
	}

	var err error
	h := sha256.New()
	if _, err = io.Copy(h, r); err != nil {
		return errors.Wrap(err, "failed to hash artifact from reader")
	}
	if _, err = r.Seek(0, 0); err != nil {
		return errors.Wrap(err, "failed to seek artifact reader to start")
	}

	// Set artifact hash in uploaded for COT generation
	tp.mUploaded.Lock()
	defer tp.mUploaded.Unlock()
	tp.uploaded[name] = h.Sum(nil)

	return nil
}

func (tp *taskPlugin) Finished(success bool) error {
	// Skip if no COT is to be generated
	if !tp.createCOT {
		return nil
	}

	// Upload certified log if requested
	if tp.certifiedLog {
		const certifiedLogName = "public/logs/certified.log"

		r, err := tp.context.ExtractLog()
		if err != nil {
			return err
		}
		defer r.Close()

		// Compute artifact hash for chain-of-trust
		if err = tp.hashArtifact(certifiedLogName, r); err != nil {
			return err
		}

		compressed, err := tp.plugin.environment.TemporaryStorage.NewFile()
		if err != nil {
			return err
		}
		defer compressed.Close()

		// Compress log
		zipper := gzip.NewWriter(compressed)
		if _, err = io.Copy(zipper, r); err != nil {
			return errors.Wrap(err, "failed to compress log")
		}
		if err = zipper.Close(); err != nil {
			return errors.Wrap(err, "failed to close compressing of log")
		}
		_, err = compressed.Seek(0, 0)
		if err != nil {
			return errors.Wrap(err, "Failed to seek to start of compressed log")
		}

		// Let's upload from compressed
		err = tp.context.UploadS3Artifact(runtime.S3Artifact{
			Name:     certifiedLogName,
			Mimetype: "text/plain; charset=utf-8",
			Stream:   compressed,
			Expires:  tp.context.TaskInfo.Expires,
			AdditionalHeaders: map[string]string{
				"Content-Encoding": "gzip",
			},
		})
		if err != nil {
			err = errors.Wrap(err, "failed to upload certified.log")
			tp.monitor.Error(err)
			return runtime.ErrNonFatalInternalError // We don't expect upload errors to be fatal
		}
	}

	COT := chainOfTrust{
		Version:     1,
		TaskID:      tp.context.TaskID,
		RunID:       tp.context.RunID,
		WorkerGroup: tp.plugin.environment.WorkerGroup,
		WorkerID:    tp.plugin.environment.WorkerID,
		Environment: map[string]interface{}{},
		Task:        tp.context.Task,
		Artifacts:   make(map[string]cotArtifact),
	}
	for name, hash := range tp.uploaded {
		COT.Artifacts[name] = cotArtifact{
			Sha256: hex.EncodeToString(hash),
		}
	}
	data, err := json.MarshalIndent(COT, "", "  ")
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize COT certificate"))
	}
	cot := bytes.NewBuffer(nil)
	w, err := clearsign.Encode(cot, tp.plugin.privateKey.PrivateKey, nil)
	if err != nil {
		return errors.Wrap(err, "failed to setup signing of COT certificate")
	}
	_, err = w.Write(data)
	if err != nil {
		return errors.Wrap(err, "failed to write COT certificate")
	}
	err = w.Close()
	if err != nil {
		return errors.Wrap(err, "failed to sign COT certificate")
	}

	const cotCertificateName = "public/chainOfTrust.json.asc"
	err = tp.context.UploadS3Artifact(runtime.S3Artifact{
		Name:     cotCertificateName,
		Mimetype: "text/plain; charset=utf-8",
		Stream:   ioext.NopCloser(bytes.NewReader(cot.Bytes())),
		Expires:  tp.context.TaskInfo.Expires,
	})
	if err != nil {
		err = errors.Wrap(err, "failed to upload COT certificate")
		tp.monitor.Error(err)
		return runtime.ErrNonFatalInternalError // We don't expect upload errors to be fatal
	}
	return nil
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

	// Compute artifact hash for chain-of-trust
	if err = tp.hashArtifact(a.Name, r); err == nil {
		// Let's upload from r
		err = tp.context.UploadS3Artifact(runtime.S3Artifact{
			Name:     a.Name,
			Mimetype: mtype,
			Stream:   r,
			Expires:  a.Expires,
		})
	}

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

		// Construct artifact name
		name := path.Join(a.Name, p)

		var uerr error
		// Compute artifact hash for chain-of-trust
		if uerr = tp.hashArtifact(name, r); uerr == nil {
			// Upload artifact
			debug(" - Uploading %s from %s -> %s", p, a.Path, name)
			uerr = tp.context.UploadS3Artifact(runtime.S3Artifact{
				Name:     name,
				Expires:  a.Expires,
				Stream:   r,
				Mimetype: mtype,
			})
		}

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
