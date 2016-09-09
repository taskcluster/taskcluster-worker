package engines

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var (
	mEngines = sync.Mutex{}
	engines  = make(map[string]EngineProvider)
)

// EngineOptions is a wrapper for the set of options/arguments given to
// an EngineProvider when an Engine is created.
//
// We pass all options as a single argument, so that we can add additional
// properties without breaking source compatibility.
type EngineOptions struct {
	Environment *runtime.Environment
	Log         *logrus.Entry
	Config      interface{}
	// Note: This is passed by-value for efficiency (and to prohibit nil), if
	// adding any large fields please consider adding them as pointers.
	// Note: This is intended to be a simple argument wrapper, do not add methods
	// to this struct.
}

// EngineProvider is the interface engine implementors must implement and
// register with engines.RegisterEngine("EngineName", provider)
//
// This function must return an Engine implementation, generally this will only
// be called once in an application. But implementors should aim to write
// reentrant code.
//
// Any error here will be fatal and likely cause the worker to stop working.
// If an implementor can determine that the platform isn't supported at
// compile-time it is recommended to not register the implementation.
type EngineProvider interface {
	NewEngine(options EngineOptions) (Engine, error)

	// ConfigSchema returns the schema for the engine configuration
	ConfigSchema() schematypes.Schema
}

// Register will register an EngineProvider, this is intended to be called
// from func init() {}, to register engines as an import side-effect.
//
// If an engine with the given name is already registered this will panic.
func Register(name string, provider EngineProvider) {
	mEngines.Lock()
	defer mEngines.Unlock()

	// Panic, if name is in use. This is okay as we always call this from init()
	// so it'll happen before any tests or code runs.
	if _, ok := engines[name]; ok {
		panic(fmt.Sprintf(
			"An engine with the name '%s' is already registered", name,
		))
	}

	// Register the engine
	engines[name] = provider
}

// Engines returns a map of registered EngineProviders.
func Engines() map[string]EngineProvider {
	mEngines.Lock()
	defer mEngines.Unlock()

	// Clone map before returning
	m := make(map[string]EngineProvider)
	for name, provider := range engines {
		m[name] = provider
	}
	return m
}

// EngineProviderBase is a base struct that provides empty implementations of
// some methods for EngineProvider
//
// Implementors of EngineProvider should embed this struct to ensure forward
// compatibility when we add new optional method to EngineProvider.
type EngineProviderBase struct{}

// ConfigSchema returns an empty object schema.
func (EngineProviderBase) ConfigSchema() schematypes.Schema {
	return schematypes.Object{}
}
