// Package livelog implements a webhook handler for serving up livelogs of a task
// sandbox.
package livelog

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
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
	context     *runtime.TaskContext
	url         string
	log         *logrus.Entry
	environment *runtime.Environment
	expiration  tcclient.Time
	monitor     runtime.Monitor
	uploaded    atomics.Bool
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{
		monitor:     options.Monitor,
		environment: options.Environment,
	}, nil
}

func (p plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		monitor:        options.Monitor,
		environment:    p.environment,
		uploaded:       atomics.NewBool(false),
	}, nil
}

func (tp *taskPlugin) Prepare(context *runtime.TaskContext) error {
	tp.context = context

	tp.url = tp.context.AttachWebHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO (garndt): add support for range headers.  Might not be used at all currently
		logReader, err := tp.context.NewLogReader()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Error opening up live log"))
			return
		}
		defer logReader.Close()

		// Get an HTTP flusher if supported in the current context, or wrap in
		// a NopFlusher, if flushing isn't available.
		wf, ok := w.(ioext.WriteFlusher)
		if !ok {
			wf = ioext.NopFlusher(w)
		}

		ioext.CopyAndFlush(wf, logReader, 100*time.Millisecond)
	}))

	err := tp.context.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain; charset=utf-8",
		URL:      tp.url,
		Expires:  tp.context.TaskInfo.Expires,
	})
	if err != nil {
		tp.context.LogError(fmt.Sprintf("Could not initialize live log plugin. Error: %s", err))
	}

	return err
}

func (tp *taskPlugin) Finished(success bool) error {
	return tp.uploadLog()
}

func (tp *taskPlugin) Exception(runtime.ExceptionReason) error {
	return tp.uploadLog()
}

func (tp *taskPlugin) uploadLog() error {
	if tp.uploaded.Get() {
		return nil
	}

	file, err := tp.context.ExtractLog()
	if err != nil {
		return err
	}
	defer file.Close()

	tempFile, err := tp.environment.TemporaryStorage.NewFile()
	if err != nil {
		return err
	}

	defer tempFile.Close()

	zip := gzip.NewWriter(tempFile)
	if _, err = io.Copy(zip, file); err != nil {
		return err
	}

	if err = zip.Close(); err != nil {
		return err
	}

	_, err = tempFile.Seek(0, 0)
	if err != nil {
		return err
	}

	err = tp.context.UploadS3Artifact(runtime.S3Artifact{
		Name:     "public/logs/live_backing.log",
		Mimetype: "text/plain; charset=utf-8",
		Expires:  tp.context.TaskInfo.Expires,
		Stream:   tempFile,
		AdditionalHeaders: map[string]string{
			"Content-Encoding": "gzip",
		},
	})

	if err != nil {
		return err
	}

	backingURL := fmt.Sprintf("https://queue.taskcluster.net/v1/task/%s/runs/%d/artifacts/public/logs/live_backing.log", tp.context.TaskInfo.TaskID, tp.context.TaskInfo.RunID)
	err = tp.context.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain; charset=utf-8",
		URL:      backingURL,
		Expires:  tp.context.TaskInfo.Expires,
	})
	if err != nil {
		tp.monitor.Error(err)
		return err
	}

	tp.uploaded.Set(true)

	return nil
}

func init() {
	plugins.Register("livelog", &pluginProvider{})
}
