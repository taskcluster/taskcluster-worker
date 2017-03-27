package lifecyclepolicy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

func TestConfigSchema(t *testing.T) {
	c := map[string]interface{}{
		"provider": "forever",
	}
	assert.NoError(t, ConfigSchema().Validate(c))
}

func TestNewForeverPolicy(t *testing.T) {
	c := map[string]interface{}{
		"provider":    "forever",
		"stopOnError": true,
	}
	s := &mockStoppable{
		stopNow:        make(chan struct{}),
		stopGracefully: make(chan struct{}),
	}
	policy := New(Options{
		Monitor: mocks.NewMockMonitor(true),
		Config:  c,
	})

	controller := policy.NewController(s)

	// Let's just try some random operations...
	for i := 0; i < 100; i++ {
		controller.ReportIdle(500 * time.Second)
		controller.ReportTaskClaimed(10)
		controller.ReportTaskResolved(500 * time.Second)
	}
	assert.False(t, isClosed(s.stopGracefully))
	assert.False(t, isClosed(s.stopNow))

	controller.ReportNonFatalError()
	assert.True(t, isClosed(s.stopGracefully))
	assert.False(t, isClosed(s.stopNow))

}
