package worker

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// Options for creating a Worker
type Options struct {
	runtime.Environment
	webhookserver.Server
	engines.Engine
	plugins.Plugin
	client.Queue
	LifeCyclePolicyConfig interface{}
	ProvisionerID         string
	WorkerType            string
	WorkerGroup           string
	WorkerID              string
	PollingInterval       time.Duration
	ReclaimOffset         time.Duration
	Concurrency           int
}

// ConfigSchema returns the schema for configuration.
func ConfigSchema() schematypes.Schema {
	return schematypes.Object{}
}
