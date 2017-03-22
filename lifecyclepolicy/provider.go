package lifecyclepolicy

import (
	"fmt"
	"sync"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var (
	mProviders = sync.Mutex{}
	providers  = make(map[string]Provider)
)

// Options for creating a LifeCyclePolicy
type Options struct {
	Worker  runtime.Stoppable
	Monitor runtime.Monitor
	Config  interface{}
}

// A Provider is a factory for a LifeCyclePolicy
type Provider interface {
	NewLifeCyclePolicy(Options) LifeCyclePolicy
	ConfigSchema() schematypes.Object
}

// Register will register an Provider, this is intended to be called
// from func init() {}, to register providers as an import side-effect.
//
// If an provider with the given name is already registered this will panic.
func Register(name string, provider Provider) {
	mProviders.Lock()
	defer mProviders.Unlock()

	// A few simple schema restrictions that people probably won't hit.
	// Notably they allow us to flatten the config structure a bit, so that's nice.
	if provider.ConfigSchema().AdditionalProperties {
		panic(fmt.Sprintf("lifecyclepolicy.Provider implementation '%s' "+
			"allows additionalProperties in ConfigSchema()", name))
	}
	if _, ok := provider.ConfigSchema().Properties["provider"]; ok {
		panic(fmt.Sprintf("lifecyclepolicy.Provider implementation '%s' "+
			"defines property 'provider' in ConfigSchema()", name))
	}

	// Panic, if name is in use. This is okay as we always call this from init()
	// so it'll happen before any tests or code runs.
	if _, ok := providers[name]; ok {
		panic(fmt.Sprintf(
			"a lifecyclepolicy.Provider with the name '%s' is already registered", name,
		))
	}

	// Register the engine
	providers[name] = provider
}
