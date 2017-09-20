// Package livelog implements a webhook handler for serving up livelogs of a task
// sandbox.
package livelog

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
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
	detach      func()
	log         *logrus.Entry
	environment *runtime.Environment
	expiration  tcclient.Time
	monitor     runtime.Monitor
	uploaded    atomics.Once
	setupDone   sync.WaitGroup
	setupErr    error
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	debug("Created livelog plugin")
	return plugin{
		monitor:     options.Monitor,
		environment: options.Environment,
	}, nil
}

func (p plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	debug("Creating taskPlugin")
	tp := &taskPlugin{
		context:     options.TaskContext,
		monitor:     options.Monitor,
		environment: p.environment,
	}
	tp.setupDone.Add(1)
	go tp.setup()
	return tp, nil
}

func (tp *taskPlugin) setup() {
	defer tp.setupDone.Done()

	if tp.environment.WebHookServer == nil {
		tp.monitor.Info("livelog disabled when WebHookServer isn't provided")
		return
	}

	tp.url, tp.detach = tp.environment.WebHookServer.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "X-Streaming")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Get an HTTP flusher if supported in the current context, or wrap in
		// a NopFlusher, if flushing isn't available.
		wf, ok := w.(ioext.WriteFlusher)
		if ok {
			w.Header().Set("X-Streaming", "true") // Allow clients to detect that we're streaming
		} else {
			wf = ioext.NopFlusher(w)
		}

		// TODO (garndt): add support for range headers.  Might not be used at all currently
		logReader, err := tp.context.NewLogReader()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Error opening up live log"))
			return
		}
		defer logReader.Close()

		w.WriteHeader(http.StatusOK)
		ioext.CopyAndFlush(wf, logReader, 100*time.Millisecond)
	}))

	err := tp.context.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain; charset=utf-8",
		URL:      tp.url,
		Expires:  tp.context.TaskInfo.Expires,
	})
	if err != nil {
		incidentID := tp.monitor.ReportError(err, "Failed to setup live logging")
		tp.context.LogError("Failed to setup livelogging: ", incidentID)
		// This isn't good, but let's not consider it fatal...
		tp.setupErr = runtime.ErrNonFatalInternalError
	}
}

func (tp *taskPlugin) Finished(success bool) error {
	tp.setupDone.Wait()
	var err error
	tp.uploaded.Do(func() {
		err = tp.uploadLog()
	})
	if err == nil {
		return tp.setupErr
	}
	return err
}

func (tp *taskPlugin) Exception(runtime.ExceptionReason) error {
	tp.setupDone.Wait()
	var err error
	tp.uploaded.Do(func() {
		err = tp.uploadLog()
	})
	if err == nil {
		return tp.setupErr
	}
	return err
}

func (tp *taskPlugin) Dispose() error {
	// Detach livelog webhook, if not already done
	if tp.detach != nil {
		tp.detach()
		tp.detach = nil
	}
	return nil
}

func (tp *taskPlugin) uploadLog() error {
	// Detach livelog webhook
	if tp.detach != nil {
		tp.detach()
		tp.detach = nil
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

	debug("Uploading live_backing.log")
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
		err = errors.Wrap(err, "failed to upload live_backing.log")
		tp.monitor.Error(err)
		return err // Upload error isn't fatal
	}

	backingURL := fmt.Sprintf("https://queue.taskcluster.net/v1/task/%s/runs/%d/artifacts/public/logs/live_backing.log", tp.context.TaskInfo.TaskID, tp.context.TaskInfo.RunID)
	err = tp.context.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain; charset=utf-8",
		URL:      backingURL,
		Expires:  tp.context.TaskInfo.Expires,
	})
	if err != nil {
		err = errors.Wrap(err, "failed to update live.log")
		tp.monitor.Error(err)
		return runtime.ErrNonFatalInternalError // Upload error isn't fatal
	}

	return nil
}

func init() {
	plugins.Register("livelog", &pluginProvider{})
}
