package lifecyclepolicy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		Worker: s,
		Config: c,
	})

	// Let's just try some random operations...
	for i := 0; i < 100; i++ {
		policy.ReportIdle(500 * time.Second)
		policy.ReportTaskClaimed(10)
		policy.ReportTaskResolved(500 * time.Second)
	}
	assert.False(t, isClosed(s.stopGracefully))
	assert.False(t, isClosed(s.stopNow))

	policy.ReportNonFatalError()
	assert.True(t, isClosed(s.stopGracefully))
	assert.False(t, isClosed(s.stopNow))

}
