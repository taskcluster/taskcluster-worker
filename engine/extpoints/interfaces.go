// Package extpoints provides methods that engine plugins can register their
// implements with as an import side-effect.
package extpoints

import (
	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

// An EngineOptions is a wrapper for the set of options/arguments given when
// an Engine is created.
//
// We pass all options as a single argument, so that we can add additional
// properties without breaking source compatibility.
type EngineOptions struct {
	GarbageCollector *gc.GarbageCollector
	//TODO: Add some sort of interface to the system logger
	//TODO: Add some interface to submit statistics for influxdb/signalfx
}

// EngineProvider is implemented by plugins that can instantiate an Engine
// instance. Implementors shouldn't expose their implementation, but instantiate
// it and register it with extpoints.Register(instance, "name") as an import
// side-effect.
//
// Implementors maybe choose not to register their implementation based on
// build constraints, GOOS and GOARCH.
type EngineProvider interface {
	// NewEngine must return an Engine implementation, generally this will only
	// be called once in an application. But implementors should aim to write
	// reentrant code.
	//
	// Any error here will be fatal and likely cause the worker to stop working.
	// If an implementor can determine that the platform isn't supported at
	// compile-time it is recommended to not register the implementation.
	NewEngine(runtime EngineOptions) (engine.Engine, error)
}
