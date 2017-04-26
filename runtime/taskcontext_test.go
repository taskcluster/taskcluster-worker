package runtime

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
)

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		panic(fmt.Sprintln(append(a, err)...))
	}
}

func TestTaskContextLogging(t *testing.T) {
	t.Parallel()
	path := filepath.Join(os.TempDir(), slugid.Nice())
	context, control, err := NewTaskContext(path, TaskInfo{})
	nilOrPanic(err, "Failed to create context")

	context.Log("Hello World")
	err = control.CloseLog()
	nilOrPanic(err, "Failed to close log file")

	reader, err := context.NewLogReader()
	nilOrPanic(err, "Failed to open log file")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log file")

	if !strings.Contains(string(data), "Hello World") {
		panic("Couldn't find 'Hello World' in the log")
	}
	nilOrPanic(reader.Close(), "Failed to close log file")
	err = context.logStream.Remove()
	nilOrPanic(err, "Failed to remove logStream")
}

func TestTaskContextConcurrentLogging(t *testing.T) {
	t.Parallel()
	path := filepath.Join(os.TempDir(), slugid.Nice())
	context, control, err := NewTaskContext(path, TaskInfo{})
	nilOrPanic(err, "Failed to create context")

	wg := sync.WaitGroup{}
	wg.Add(5) // This could trigger errors with race condition detector
	go func() { context.Log("Hello World 2"); wg.Done() }()
	go func() { context.Log("Hello World 1"); wg.Done() }()
	go func() { context.Log("Hello World 3 - Cheese"); wg.Done() }()
	go func() { context.Log("Hello World 4"); wg.Done() }()
	go func() { context.Log("Hello World 5"); wg.Done() }()
	wg.Wait()
	err = control.CloseLog()
	nilOrPanic(err, "Failed to close log file")

	reader, err := context.NewLogReader()
	nilOrPanic(err, "Failed to open log file")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log file")

	if !strings.Contains(string(data), "Cheese") {
		panic("Couldn't find 'Cheese' in the log")
	}
	nilOrPanic(reader.Close(), "Failed to close log file")
	err = context.logStream.Remove()
	nilOrPanic(err, "Failed to remove logStream")
}
