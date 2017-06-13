package runtime

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
)

func TestTaskContextLogging(t *testing.T) {
	t.Parallel()
	path := filepath.Join(os.TempDir(), slugid.Nice())
	context, control, err := NewTaskContext(path, TaskInfo{})
	require.NoError(t, err, "Failed to create context")

	context.Log("Hello World")
	err = control.CloseLog()
	require.NoError(t, err, "Failed to close log file")

	reader, err := context.NewLogReader()
	require.NoError(t, err, "Failed to open log file")
	data, err := ioutil.ReadAll(reader)
	require.NoError(t, err, "Failed to read log file")

	if !strings.Contains(string(data), "Hello World") {
		panic("Couldn't find 'Hello World' in the log")
	}
	require.NoError(t, reader.Close(), "Failed to close log file")
	err = context.logStream.Remove()
	require.NoError(t, err, "Failed to remove logStream")
}

func TestTaskContextConcurrentLogging(t *testing.T) {
	t.Parallel()
	path := filepath.Join(os.TempDir(), slugid.Nice())
	context, control, err := NewTaskContext(path, TaskInfo{})
	require.NoError(t, err, "Failed to create context")

	wg := sync.WaitGroup{}
	wg.Add(5) // This could trigger errors with race condition detector
	go func() { context.Log("Hello World 2"); wg.Done() }()
	go func() { context.Log("Hello World 1"); wg.Done() }()
	go func() { context.Log("Hello World 3 - Cheese"); wg.Done() }()
	go func() { context.Log("Hello World 4"); wg.Done() }()
	go func() { context.Log("Hello World 5"); wg.Done() }()
	wg.Wait()
	err = control.CloseLog()
	require.NoError(t, err, "Failed to close log file")

	reader, err := context.NewLogReader()
	require.NoError(t, err, "Failed to open log file")
	data, err := ioutil.ReadAll(reader)
	require.NoError(t, err, "Failed to read log file")

	if !strings.Contains(string(data), "Cheese") {
		panic("Couldn't find 'Cheese' in the log")
	}
	require.NoError(t, reader.Close(), "Failed to close log file")
	err = context.logStream.Remove()
	require.NoError(t, err, "Failed to remove logStream")
}

func TestTaskContextHasScopes(t *testing.T) {
	path := filepath.Join(os.TempDir(), slugid.Nice())
	ctx, control, err := NewTaskContext(path, TaskInfo{
		Scopes: []string{
			"queue:api",
			"queue:*",
			"test:scope",
		},
	})
	require.NoError(t, err, "Failed to create context")
	defer control.Dispose()
	defer control.CloseLog()

	assert.True(t, ctx.HasScopes([]string{}), "empty scope-set")
	assert.True(t, ctx.HasScopes([]string{
		"queue:api",
	}), "plain scope")
	assert.True(t, ctx.HasScopes([]string{
		"queue:*",
	}), "star scope")
	assert.True(t, ctx.HasScopes([]string{
		"queue:*", "queue:test",
	}), "two star scopes")
	assert.True(t, ctx.HasScopes([]string{
		"queue:api", "test:scope",
	}), "two plain scopes")
	assert.True(t, ctx.HasScopes([]string{
		"queue:api", "test:scope",
	}, []string{
		"queue:*", "queue:test",
	}), "two sets")

	assert.True(t, ctx.HasScopes([]string{
		"false",
	}, []string{
		"queue:*", "queue:test",
	}), "one set satisfied")
	assert.False(t, ctx.HasScopes([]string{
		"false",
	}), "plain false")
	assert.False(t, ctx.HasScopes([]string{
		"false:*",
	}), "star false")
}
