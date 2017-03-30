package stoponerror

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

func TestStopOnError(t *testing.T) {
	s := &runtime.LifeCycleTracker{}
	p, err := plugins.Plugins()["stoponerror"].NewPlugin(plugins.PluginOptions{
		Environment: &runtime.Environment{
			Worker: s,
		},
		Monitor: mocks.NewMockMonitor(true).WithTag("plugin", "stoponerror"),
	})
	require.NoError(t, err)

	assert.False(t, s.StoppingGracefully.IsFallen())
	assert.False(t, s.StoppingNow.IsFallen())
	p.ReportNonFatalError()
	assert.True(t, s.StoppingGracefully.IsFallen())
	assert.False(t, s.StoppingNow.IsFallen())
}
