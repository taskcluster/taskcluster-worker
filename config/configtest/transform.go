// Package configtest provides structs and logic for declarative configuration
// tests.
package configtest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

// Case allows declaration of a transformation to run on input and validate
// against declared tesult.
type Case struct {
	Transform string
	Input     map[string]interface{}
	Result    map[string]interface{}
}

// Test will execute the test case panicing if Input doesn't become Result
func (c Case) Test(t *testing.T) {
	monitor := mocks.NewMockMonitor(false)
	transform := config.Providers()[c.Transform]
	require.NotNil(t, transform, "unknown transform ", c.Transform)

	err := transform.Transform(c.Input, monitor)
	require.NoError(t, err, "Transform(Input) failed")

	require.Equal(t, c.Result, c.Input, "Unexpected result")
}
