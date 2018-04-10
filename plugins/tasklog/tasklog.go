package tasklog

import (
	"compress/gzip"
	"io"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	monitor     runtime.Monitor
	environment *runtime.Environment
}

type taskPlugin struct {
	plugins.TaskPluginBase
	parent   *plugin
	context  *runtime.TaskContext
	monitor  runtime.Monitor
	uploaded atomics.Once // ensure we only upload once
}

func init() {
	plugins.Register("tasklog", &pluginProvider{})
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	debug("Created tasklog plugin")
	return &plugin{
		monitor:     options.Monitor,
		environment: options.Environment,
	}, nil
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	debug("Created tasklog taskPlugin")
	return &taskPlugin{
		parent:  p,
		context: options.TaskContext,
		monitor: options.Monitor,
	}, nil
}

func (tp *taskPlugin) Finished(success bool) error {
	var err error
	tp.uploaded.Do(func() {
		err = tp.uploadLog()
	})
	return err
}

func (tp *taskPlugin) Exception(runtime.ExceptionReason) error {
	var err error
	tp.uploaded.Do(func() {
		err = tp.uploadLog()
	})
	return err
}

func (tp *taskPlugin) uploadLog() error {
	// Get the log
	logFile, err := tp.context.ExtractLog()
	if err != nil {
		return errors.Wrap(err, "tasklog: TaskContext.extractLog() failed")
	}
	defer logFile.Close()

	// Create temporary file for gzipping
	tempFile, err := tp.parent.environment.TemporaryStorage.NewFile()
	if err != nil {
		return errors.Wrap(err, "tasklog: failed to create temporary file")
	}
	defer tempFile.Close()

	// gzip the log
	zip := gzip.NewWriter(tempFile)
	if _, err = io.Copy(zip, logFile); err != nil {
		return errors.Wrap(err, "tasklog: failed to read log-file")
	}
	if err = zip.Close(); err != nil {
		return errors.Wrap(err, "tasklog: failed to gzip log-file")
	}
	if _, err = tempFile.Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "tasklog: failed to seek start of temp file")
	}

	// Upload gzipped task.log
	debug("uploading 'public/logs/task.log'")
	err = tp.context.UploadS3Artifact(runtime.S3Artifact{
		Name:     "public/logs/task.log",
		Mimetype: "text/plain; charset=utf-8",
		Expires:  tp.context.TaskInfo.Expires,
		Stream:   tempFile,
		AdditionalHeaders: map[string]string{
			"Content-Encoding": "gzip",
		},
	})
	if err != nil {
		tp.monitor.Error(errors.Wrap(err, "failed to update task.log"))
		// Upload error isn't fatal, could just be bad network
		return runtime.ErrNonFatalInternalError
	}

	return nil
}
