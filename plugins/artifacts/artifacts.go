// Package artifacts is responsible for uploading artifacts after builds
package artifacts

import (
	"mime"
	"path"
	"path/filepath"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

func (pluginProvider) NewPlugin(plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

type plugin struct {
	plugins.PluginBase
}

func (plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var p payloadType
	err := payloadSchema.Map(options.Payload, &p)
	if err == schematypes.ErrTypeMismatch {
		panic("internal error -- type mismatch")
	} else if err != nil {
		return nil, engines.ErrContractViolation
	}

	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		artifacts:      p.Artifacts,
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	context   *runtime.TaskContext
	artifacts []artifact
}

func (tp *taskPlugin) Prepare(context *runtime.TaskContext) error {
	tp.context = context
	return nil
}

func (tp *taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	nonFatalErrs := []runtime.MalformedPayloadError{}

	for _, artifact := range tp.artifacts {
		// If expires is set to this time it's the default value
		if artifact.Expires.IsZero() {
			artifact.Expires = time.Time(tp.context.TaskInfo.Expires)
		}
		switch artifact.Type {
		case "directory":
			err := result.ExtractFolder(artifact.Path, tp.createUploadHandler(artifact.Name, artifact.Expires))
			if err != nil {
				if tp.errorHandled(artifact.Name, artifact.Expires, err) {
					nonFatalErrs = append(nonFatalErrs, runtime.NewMalformedPayloadError(err.Error()))
					continue
				}
				return false, err
			}
		case "file":
			fileReader, err := result.ExtractFile(artifact.Path)
			if err != nil {
				if tp.errorHandled(artifact.Name, artifact.Expires, err) {
					nonFatalErrs = append(nonFatalErrs, runtime.NewMalformedPayloadError(err.Error()))
					continue
				}
				return false, err
			}
			err = tp.attemptUpload(fileReader, artifact.Path, artifact.Name, artifact.Expires)
			if err != nil {
				return false, err
			}
		}
	}

	if len(nonFatalErrs) > 0 {
		// Only report an exception if the command executed successfully.
		// The logic behind this it is that no artifact upload failure is
		// expected for a succeeded run, but failed tasks might be missing
		// some artifacts.
		if result.Success() {
			return false, runtime.MergeMalformedPayload(nonFatalErrs...)
		}

		return false, nil
	}
	return true, nil
}

func (tp taskPlugin) errorHandled(name string, expires time.Time, err error) bool {
	var reason string
	if _, ok := runtime.IsMalformedPayloadError(err); ok {
		reason = "invalid-resource-on-worker"
	} else if err == engines.ErrFeatureNotSupported || err == runtime.ErrNonFatalInternalError || err == engines.ErrHandlerInterrupt {
		reason = "invalid-resource-on-worker"
	} else if err == engines.ErrResourceNotFound {
		reason = "file-missing-on-worker"
	}

	if reason != "" {
		tp.context.Log("Artifact upload error handled. Continuing...", name, err.Error())
		tp.context.CreateErrorArtifact(runtime.ErrorArtifact{
			Name:    name,
			Message: err.Error(),
			Reason:  reason,
			Expires: tcclient.Time(expires),
		})
		return true
	}
	return false
}

func (tp taskPlugin) createUploadHandler(name string, expires time.Time) func(string, ioext.ReadSeekCloser) error {
	return func(artifactPath string, stream ioext.ReadSeekCloser) error {
		return tp.attemptUpload(stream, artifactPath, path.Join(name, artifactPath), expires)
	}
}

func (tp taskPlugin) attemptUpload(fileReader ioext.ReadSeekCloser, path string, name string, expires time.Time) error {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		// application/octet-stream is the mime type for "unknown"
		mimeType = "application/octet-stream"
	}
	return tp.context.UploadS3Artifact(runtime.S3Artifact{
		Name:     name,
		Mimetype: mimeType,
		Stream:   fileReader,
		Expires:  tcclient.Time(expires),
	})
}

func init() {
	plugins.Register("artifacts", pluginProvider{})
}
