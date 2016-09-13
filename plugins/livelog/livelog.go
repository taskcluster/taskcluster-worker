// Package livelog implements a webhook handler for serving up livelogs of a task
// sandbox.
package livelog

import (
	"bufio"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	log *logrus.Entry
}

type taskPlugin struct {
	plugins.TaskPluginBase
	context    *runtime.TaskContext
	url        string
	expiration tcclient.Time
	log        *logrus.Entry
}

func (pluginProvider) NewPlugin(opts plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{log: opts.Log}, nil
}

func (p plugin) NewTaskPlugin(opts plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		log: p.log.WithFields(logrus.Fields{
			"taskID": opts.TaskInfo.TaskID,
			"runID":  opts.TaskInfo.RunID,
		}),
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
		}

		defer logReader.Close()

		flusher := w.(http.Flusher)
		reader := bufio.NewReader(logReader)

		defer flusher.Flush()
		now := time.Now()
		for {
			line, isPrefix, err := reader.ReadLine()
			if err != nil {
				return
			}
			w.Write(line)
			if !isPrefix {
				w.Write([]byte("\r\n"))
			}

			// if it has elapsed at least 500 milliseconds, flush it
			if time.Since(now) >= 500*time.Millisecond {
				flusher.Flush()
				now = time.Now()
			}
		}
	}))

	err := runtime.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain",
		URL:      tp.url,
		Expires:  tp.context.TaskInfo.Expires,
	}, tp.context)
	if err != nil {
		tp.context.LogError(fmt.Sprintf("Could not initialize live log plugin. Error: %s", err))
	}

	return err
}

func (tp *taskPlugin) Finished(success bool) error {
	file, err := tp.context.ExtractLog()
	if err != nil {
		return err
	}
	defer file.Close()
	err = runtime.UploadS3Artifact(runtime.S3Artifact{
		Name:     "public/logs/live_backing.log",
		Mimetype: "text/plain",
		Expires:  tp.context.TaskInfo.Expires,
		Stream:   file,
	}, tp.context)

	if err != nil {
		return err
	}

	backingURL := fmt.Sprintf("https://queue.taskcluster.net/v1/task/%s/runs/%d/artifacts/public/logs/live_backing.log", tp.context.TaskInfo.TaskID, tp.context.TaskInfo.RunID)
	err = runtime.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain",
		URL:      backingURL,
		Expires:  tp.context.TaskInfo.Expires,
	}, tp.context)
	if err != nil {
		tp.log.Error(err)
		return err
	}

	return nil
}

func init() {
	plugins.Register("livelog", &pluginProvider{})
}
