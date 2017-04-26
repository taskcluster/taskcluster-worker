package config

import (
	"fmt"
	"sync"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

// A TransformationProvider provides a method Transform(config) that knows
// how to transform the configuration object. Typically, by replacing objects
// matching a specific pattern or overwriting specific values.
//
// The config object is a simple JSON compatible structure, and Transform()
// should maintain this property. That means it consists of the following types:
//   - map[string]interface{}
//   - []interface{}
//   - string
//   - float64
//   - bool
//   - nil
type TransformationProvider interface {
	Transform(config map[string]interface{}, monitor runtime.Monitor) error
}

var (
	providers  = make(map[string]TransformationProvider)
	mProviders = sync.Mutex{}
)

// Register will register a TransformationProvider. This is intended to be
// called at static initialization time (in func init()), and will thus panic
// if the given name already is in use.
func Register(name string, provider TransformationProvider) {
	mProviders.Lock()
	defer mProviders.Unlock()

	if _, ok := providers[name]; ok {
		panic(fmt.Sprintf("config.Provider name '%s' is already in use!", name))
	}

	providers[name] = provider
}

// Providers returns a map of the registered TransformationProvider.
func Providers() map[string]TransformationProvider {
	mProviders.Lock()
	defer mProviders.Unlock()

	// Clone providers
	m := map[string]TransformationProvider{}
	for name, provider := range providers {
		m[name] = provider
	}

	return m
}
