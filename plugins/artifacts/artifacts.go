//go:generate go-composite-schema --unexported --required artifacts payload-schema.yml generated_payloadschema.go

// Package artifacts is responsible for uploading artifacts after builds
package artifacts

import (
	"fmt"
	"mime"
	"path/filepath"
	"time"

	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type pluginProvider struct {
	extpoints.PluginProviderBase
}

func (pluginProvider) NewPlugin(extpoints.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

type plugin struct {
	plugins.PluginBase
}

func (plugin) PayloadSchema() (runtime.CompositeSchema, error) {
	return payloadSchema, nil
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		payload:        *(options.Payload.(*payload)),
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	context *runtime.TaskContext
	payload payload
}

func (tp *taskPlugin) Prepare(context *runtime.TaskContext) error {
	tp.context = context
	return nil
}

func (tp *taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	var err error
	for _, artifact := range tp.payload {

		// If expires is set to this time it's either the default value or has been set to an invalid time anyway
		if time.Time(artifact.Expires).IsZero() {
			artifact.Expires = tp.context.TaskInfo.Expires
		}

		switch artifact.Type {
		case "directory":
			err = result.ExtractFolder(artifact.Path, tp.createUploadHandler(artifact.Name, artifact.Expires))
			if err != nil {
				runtime.CreateErrorArtifact(runtime.ErrorArtifact{
					Message: fmt.Sprintf("Could not open directory '%s'", artifact.Path),
					Reason:  "invalid-resource-on-worker",
					Expires: artifact.Expires,
				}, tp.context)
			}
		case "file":
			fileReader, err := result.ExtractFile(artifact.Path)
			if err != nil {
				runtime.CreateErrorArtifact(runtime.ErrorArtifact{
					Message: fmt.Sprintf("Could not read file '%s'", artifact.Path),
					Reason:  "file-missing-on-worker",
					Expires: artifact.Expires,
				}, tp.context)
			}
			err = tp.attemptUpload(fileReader, artifact.Path, artifact.Name, artifact.Expires)
		}
	}
	// TODO: Don't always return true?
	return true, nil
}

func (tp taskPlugin) createUploadHandler(name string, expires tcclient.Time) func(string, ioext.ReadSeekCloser) error {
	return func(path string, stream ioext.ReadSeekCloser) error {
		return tp.attemptUpload(stream, path, name, expires)
	}
}

func (tp taskPlugin) attemptUpload(fileReader ioext.ReadSeekCloser, path string, name string, expires tcclient.Time) error {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		// application/octet-stream is the mime type for "unknown"
		mimeType = "application/octet-stream"
	}
	return runtime.UploadS3Artifact(runtime.S3Artifact{
		Name:     name,
		Mimetype: mimeType,
		Stream:   fileReader,
		Expires:  expires,
	}, tp.context)
}

func init() {
	extpoints.PluginProviders.Register(new(pluginProvider), "artifacts")
}
