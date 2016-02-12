//go:generate schema-gen

// Package mockengine implements a MockEngine that doesn't really do anything,
// but allows us to test plugins without having to run a real engine.
package mockengine

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	Log *logrus.Entry
}

func init() {
	// Register the mock engine as an import side-effect
	extpoints.EngineProviders.Register(func(
		options extpoints.EngineOptions,
	) (engines.Engine, error) {
		fmt.Println(options.Log)
		return engine{Log: options.Log}, nil
	}, "mock")
}

// task.payload.start when engine is "mock"
type payload struct {
	Function string `json:"function"`
	Argument string `json:"argument"`
	Delay    int64  `json:"delay"`
}

// mock config contains no fields
func (e engine) ConfigSchema() []byte {
	return []byte("{}")
}

func (e engine) PayloadSchema() runtime.CompositeSchema {
	// Declare the schema for the "task.payload.start" property
	schema, err := runtime.NewCompositeSchema("start", `{
    "type": "object",
    "properties": {
      "delay": {"type": "integer"},
      "function": {
        "type": "string",
        "enum": [
          "true",
          "false",
          "set-volume",
          "get-volume",
          "ping-proxy",
          "write-log",
					"write-error-log"
        ]
      },
      "argument": {"type": "string"}
    },
    "required": ["delay", "function", "argument"],
    "additionalProperties": false
  }`, true, func() interface{} { return &payload{} })
	if err != nil {
		// Any errors here are supposed to be static
		panic(err)
	}
	return schema
}

func (e engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	// We know that payload was created with CompositeSchema.Parse() from the
	// schema returned by PayloadSchema(), so here we type assert that it is
	// indeed a pointer to such a thing.
	e.Log.Debug("Building Sandbox")
	p, valid := options.Payload.(*payload)
	if !valid {
		// TODO: Write to some sort of log if the type assertion fails
		return nil, engines.ErrContractViolation
	}
	return &sandbox{
		payload: p,
		context: options.TaskContext,
		mounts:  make(map[string]*mount),
		proxies: make(map[string]http.Handler),
	}, nil
}

func (engine) NewCacheFolder() (engines.Volume, error) {
	// Create a new cache folder
	return &volume{}, nil
}
