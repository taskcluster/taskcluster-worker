package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"sync"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"

	"gopkg.in/djherbis/stream.v1"
)

// ErrLogNotClosed represents an invalid attempt to extract a log
// while it is still open.
var ErrLogNotClosed = errors.New("Log is still open")

// TaskStatus represents the current status of the task.
type TaskStatus string // TODO: (jonasfj) TaskContext shouldn't track status

// Enumerate task status to aid life-cycle decision making
// Use strings for benefit of simple logging/reporting
const (
	Aborted   TaskStatus = "Aborted" // TODO: (jonasfj) Don't distinguish between cancel/abort
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
	Created  time.Time
	Deadline time.Time
	Expires  time.Time
	Scopes   []string
	Task     interface{} // task definition in map[string]interface{} types..
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
	logStream   *stream.Stream
	logLocation string // Absolute path to log file
	logClosed   bool
	mu          sync.RWMutex
	queue       client.Queue
	status      TaskStatus
	done        chan struct{}
	authorizer  client.Authorizer
	clientID    string
	accessToken string
	certificate string
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
		return nil, nil, errors.Wrap(err, "failed to create temporary file for storing log")
	}
	ctx := &TaskContext{
		logStream:   logStream,
		logLocation: tempLogFile,
		TaskInfo:    task,
		done:        make(chan struct{}),
	}
	ctx.authorizer = client.NewAuthorizer(func() (string, string, string, error) {
		ctx.mu.RLock()
		defer ctx.mu.RUnlock()

		if ctx.clientID == "" || ctx.accessToken == "" {
			panic(errors.New("TaskContext doesn't have clientID and accessToken"))
		}
		return ctx.clientID, ctx.accessToken, ctx.certificate, nil
	})
	return ctx, &TaskContextController{ctx}, nil
}

// CloseLog will close the log so no more messages can be written.
func (c *TaskContextController) CloseLog() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logClosed {
		return nil
	}

	debug("closing log on TaskContext")
	c.logClosed = true
	return c.logStream.Close()
}

// Dispose will clean-up all resources held by the TaskContext
func (c *TaskContextController) Dispose() error {
	debug("disposing TaskContext")
	return c.logStream.Remove()
}

// SetQueueClient will set a client for the TaskCluster Queue.  This client
// can then be used by others that have access to the task context and require
// interaction with the queue.
func (c *TaskContextController) SetQueueClient(client client.Queue) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.queue = client
}

// Queue will return a client for the TaskCluster Queue.  This client
// is useful for plugins that require interactions with the queue, such as creating
// artifacts.
func (c *TaskContext) Queue() client.Queue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// TODO: Remove this method when client library has been rewritten to consume
	// 			 a Authorizor implementation
	return c.queue
}

// Authorizer can sign requests with temporary credentials associated with the
// task.
//
// Notice, when blindly forwarding requests task.scopes should be set as
// authorizedScopes, otherwise artifact upload and resolution will possible.
func (c *TaskContext) Authorizer() client.Authorizer {
	return c.authorizer
}

// SetCredentials is used to provide the task-specific temporary credentials,
// and update these whenever they change.
func (c *TaskContextController) SetCredentials(clientID, accessToken, certificate string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clientID = clientID
	c.accessToken = accessToken
	c.certificate = certificate
}

// Deadline returns empty time and false, this is implemented to satisfy
// context.Context.
func (c *TaskContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done returns a channel that is closed when to TaskContext is aborted or
// canceled.
//
// Implemented in compliance with context.Context.
func (c *TaskContext) Done() <-chan struct{} {
	return c.done
}

// Err returns context.Canceled, if task as canceled or aborted.
//
// Implemented in compliance with context.Context.
func (c *TaskContext) Err() error {
	// NOTE: This method is implemented to support the context.Context interface
	//       and may not return anything but context.Canceled or
	//       context.DeadlineExceeded.
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status == Aborted || c.status == Cancelled {
		return context.Canceled
	}
	return nil
}

// Value returns nil, this is implemented to satisfy context.Context
func (c *TaskContext) Value(key interface{}) interface{} {
	return nil
}

// Abort sets the status to aborted
func (c *TaskContext) Abort() {
	// TODO: (jonasfj): Remove this method TaskContext
	// TODO (garndt): add abort/cancel channels for plugins to listen on
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = Aborted
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

// IsAborted returns true if the current status is Aborted
func (c *TaskContext) IsAborted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == Aborted
}

// Cancel sets the status to cancelled
func (c *TaskContext) Cancel() {
	// TODO: (jonasfj): Remove this method TaskContext, add to TaskContextController
	c.mu.Lock()
	c.status = Cancelled
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.mu.Unlock()
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
		_ = err //TODO: Forward this to the system log, it's not a critical error
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

// ExtractLog returns an IO object to read the log.
func (c *TaskContext) ExtractLog() (ioext.ReadSeekCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.logClosed {
		return nil, ErrLogNotClosed
	}

	file, err := os.Open(c.logLocation)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// HasScopes returns true, if task.scopes covers one of the scopeSets given
func (c *TaskContext) HasScopes(scopeSets ...[]string) bool {
	for _, scopes := range scopeSets {
		satisfied := true
		for _, required := range scopes {
			satisfied = false
			for _, scope := range c.Scopes {
				if required == scope || strings.HasSuffix(scope, "*") && strings.HasPrefix(required, scope[0:len(scope)-1]) {
					satisfied = true
					break
				}
			}
			if !satisfied {
				break
			}
		}
		if satisfied {
			return true
		}
	}
	return false
}
