package watchdog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

func TestWatchdog(t *testing.T) {
	lifeCycle := &runtime.LifeCycleTracker{}

	p, err := provider{}.NewPlugin(plugins.PluginOptions{
		Config: map[string]interface{}{"timeout": 1},
		Environment: &runtime.Environment{
			Worker: lifeCycle,
		},
		Monitor: mocks.NewMockMonitor(false),
	})
	require.NoError(t, err)
	assert.False(t, lifeCycle.StoppingNow.IsDone())

	// Sleep 500ms is okay
	p.ReportIdle(0)
	time.Sleep(500 * time.Millisecond)
	assert.False(t, lifeCycle.StoppingNow.IsDone())

	// Sleep 500ms is okay
	p.ReportIdle(0)
	time.Sleep(500 * time.Millisecond)
	assert.False(t, lifeCycle.StoppingNow.IsDone())

	// Sleeping > 1s is not okay, when timeout is configured to 1s
	select {
	case <-lifeCycle.StoppingNow.Done():
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Expected watchdog plugin to initiate graceful shutdown")
	}
}

func TestWatchdogDefaultTimeout(t *testing.T) {
	lifeCycle := &runtime.LifeCycleTracker{}

	p, err := provider{}.NewPlugin(plugins.PluginOptions{
		Config: map[string]interface{}{},
		Environment: &runtime.Environment{
			Worker: lifeCycle,
		},
		Monitor: mocks.NewMockMonitor(false),
	})
	require.NoError(t, err)
	assert.False(t, lifeCycle.StoppingNow.IsDone())

	// Sleep 500ms is okay
	p.ReportIdle(0)
	time.Sleep(500 * time.Millisecond)
	assert.False(t, lifeCycle.StoppingNow.IsDone())
}

func TestWatchdogRunningIgnored(t *testing.T) {
	lifeCycle := &runtime.LifeCycleTracker{}

	p, err := provider{}.NewPlugin(plugins.PluginOptions{
		Config: map[string]interface{}{"timeout": 1},
		Environment: &runtime.Environment{
			Worker: lifeCycle,
		},
		Monitor: mocks.NewMockMonitor(false),
	})
	require.NoError(t, err)
	assert.False(t, lifeCycle.StoppingNow.IsDone())

	// Create new TaskPlugin
	tp, err := p.NewTaskPlugin(plugins.TaskPluginOptions{})
	require.NoError(t, err)

	tp.BuildSandbox(nil)
	tp.Started(nil)

	// Sleep 2s is okay because we're running
	time.Sleep(2 * time.Second)
	assert.False(t, lifeCycle.StoppingNow.IsDone())

	// Stopped, Dispose or Exception should all do...
	tp.Dispose()

	// Sleeping > 1s is not okay, when timeout is configured to 1s
	select {
	case <-lifeCycle.StoppingNow.Done():
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Expected watchdog plugin to initiate graceful shutdown")
	}
}
