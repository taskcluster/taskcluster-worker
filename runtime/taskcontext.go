package runtime

import (
	"fmt"
	"io"
	"net/http"

	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"

	"gopkg.in/djherbis/stream.v1"
)

// The TaskInfo struct exposes generic properties from a task definition.
type TaskInfo struct {
	// TODO: Add fields and getters to get them
}

// The TaskContext exposes generic properties and functionality related to a
// task that is currently being executed.
//
// This context is used to ensure that every component both engines and plugins
// that operates on a task have access to some common information about the
// task. This includes log drains, per-task credentials, generic task
// properties, and abortion notifications.
type TaskContext struct {
	TaskInfo
	webHookSet *webhookserver.WebHookSet
	logStream  *stream.Stream
}

// TaskContextController exposes logic for controlling the TaskContext.
//
// Spliting this out from TaskContext ensures that engines and plugins doesn't
// accidentally Dispose() the TaskContext.
type TaskContextController struct {
	*TaskContext
}

// NewTaskContext creates a TaskContext and associated TaskContextController
func NewTaskContext(tempLogFile string) (*TaskContext, *TaskContextController, error) {
	logStream, err := stream.New(tempLogFile)
	if err != nil {
		return nil, nil, err
	}
	ctx := &TaskContext{
		logStream: logStream,
	}
	return ctx, &TaskContextController{ctx}, nil
}

// CloseLog will close the log so no more messages can be written.
func (c *TaskContextController) CloseLog() error {
	return c.logStream.Close()
}

// Dispose will clean-up all resources held by the TaskContext
func (c *TaskContextController) Dispose() error {
	return c.logStream.Remove()
}

// Log writes a log message from the worker
//
// These log messages will be prefixed "[taskcluster]" so it's easy to see to
// that they are worker logs.
func (c *TaskContext) Log(a ...interface{}) {
	a = append([]interface{}{"[taskcluster] "}, a...)
	_, err := fmt.Fprintln(c.logStream, a...)
	if err != nil {
		//TODO: Forward this to the system log, it's not a critical error
	}
}

// LogDrain returns a drain to which log message can be written.
//
// Users should note that multiple writers are writing to this drain
// concurrently, and it is recommend that writers write in chunks of one line.
func (c *TaskContext) LogDrain() io.Writer {
	return c.logStream
}

// NewLogReader returns a ReadCloser that reads the log from the start as the
// log is written.
//
// Calls to Read() on the resulting ReadCloser are blocking. They will return
// when data is written or EOF is reached.
//
// Consumers should ensure the ReadCloser is closed before discarding it.
func (c *TaskContext) NewLogReader() (io.ReadCloser, error) {
	return c.logStream.NextReader()
}

// AttachWebHook will take an http.Handler and expose it to the internet such
// that requests to any sub-resource of url returned will be forwarded to the
// handler.
//
// Additionally, we promise that the URL contains a cryptographically random
// sequence of characters rendering it unpredictable. This can be used as a
// cheap form of access-control, and it is safe as task-specific web hooks
// are short-lived by nature.
//
// Example use-cases:
//  - livelog plugin
//  - plugins for interactive shell/display/debugger, etc.
//  - engines that send an email and await user confirmation
//  ...
//
// Implementors attaching a hook should take care to ensure that the handler
// is able to respond with a non-2xx response, if the data it is accessing isn't
// available anymore. All webhooks will be detached at the end of the
// task-cycle, but not until the very end.
func (c *TaskContext) AttachWebHook(handler http.Handler) string {
	return c.webHookSet.AttachHook(handler)
}
