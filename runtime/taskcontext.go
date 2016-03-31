package runtime

import (
	"fmt"
	"io"
	"net/http"

	"sync"

	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"

	"gopkg.in/djherbis/stream.v1"
)

// An ExceptionReason specifies the reason a task reached an exception state.
type ExceptionReason string

// Reasons why a task can reach an exception state. Implementors should be
// warned that additional entries may be added in the future.
const (
	MalformedPayload ExceptionReason = "malformed-payload"
	WorkerShutdown   ExceptionReason = "worker-shutdown"
	InternalError    ExceptionReason = "internal-error"
)

// TaskStatus represents the current status of the task.
type TaskStatus string

// Enumerate task status to aid life-cycle decision making
// Use strings for benefit of simple logging/reporting
const (
	Aborted   TaskStatus = "Aborted"
	Cancelled TaskStatus = "Cancelled"
	Succeeded TaskStatus = "Succeeded"
	Failed    TaskStatus = "Failed"
	Errored   TaskStatus = "Errored"
	Claimed   TaskStatus = "Claimed"
	Reclaimed TaskStatus = "Reclaimed"
)

// The TaskInfo struct exposes generic properties from a task definition.
//
// Note, do not be tempted to add task definition or status here in its entirety
// as it can encourage undesired behaviors.  Instead only the data necessary
// should be exposed and nothing more.  One such anti-pattern could be for a
// plugin to look at task.extra instead of adding data to task.payload.
type TaskInfo struct {
	TaskID   string
	RunID    int
	Created  tcclient.Time
	Deadline tcclient.Time
	Expires  tcclient.Time
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
	mu         sync.RWMutex
	queue      client.Queue
	status     TaskStatus
	cancelled  bool
}

// TaskContextController exposes logic for controlling the TaskContext.
//
// Spliting this out from TaskContext ensures that engines and plugins doesn't
// accidentally Dispose() the TaskContext.
type TaskContextController struct {
	*TaskContext
}

// NewTaskContext creates a TaskContext and associated TaskContextController
func NewTaskContext(tempLogFile string, task TaskInfo) (*TaskContext, *TaskContextController, error) {
	logStream, err := stream.New(tempLogFile)
	if err != nil {
		return nil, nil, err
	}
	ctx := &TaskContext{
		logStream: logStream,
		TaskInfo:  task,
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

// SetQueueClient will set a client for the TaskCluster Queue.  This client
// can then be used by others that have access to the task context and require
// interaction with the queue.
func (c *TaskContextController) SetQueueClient(client client.Queue) {
	c.mu.Lock()
	c.queue = client
	c.mu.Unlock()
}

// Queue will return a client for the TaskCluster Queue.  This client
// is useful for plugins that require interactions with the queue, such as creating
// artifacts.
func (c *TaskContext) Queue() client.Queue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.queue
}

// Abort sets the status to aborted
func (c *TaskContext) Abort() {
	// TODO (garndt): add abort/cancel channels for plugins to listen on
	c.mu.Lock()
	c.status = Aborted
	c.mu.Unlock()
	return
}

// IsAborted returns true if the current status is Aborted
func (c *TaskContext) IsAborted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == Aborted
}

// Cancel sets the status to cancelled
func (c *TaskContext) Cancel() {
	c.mu.Lock()
	c.status = Cancelled
	c.mu.Unlock()
	return
}

// IsCancelled returns true if the current status is Cancelled
func (c *TaskContext) IsCancelled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == Cancelled
}

// Log writes a log message from the worker
//
// These log messages will be prefixed "[taskcluster]" so it's easy to see to
// that they are worker logs.
func (c *TaskContext) Log(a ...interface{}) {
	c.log("[taskcluster] ", a...)
}

// LogError writes a log error message from the worker
//
// These log messages will be prefixed "[taskcluster:error]" so it's easy to see to
// that they are worker logs.  These errors are also easy to grep from the logs in
// case of failure.
func (c *TaskContext) LogError(a ...interface{}) {
	c.log("[taskcluster:error] ", a...)
}

func (c *TaskContext) log(prefix string, a ...interface{}) {
	a = append([]interface{}{prefix}, a...)
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
