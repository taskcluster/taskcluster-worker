package config

import (
	"fmt"
	"sync"

	schematypes "github.com/taskcluster/go-schematypes"
)

// A Provider knows how to MergeConfig into a config object given a set of
// options satisfying the OptionsSchema.
type Provider interface {
	OptionsSchema() schematypes.Schema
	MergeConfig(config map[string]interface{}, options interface{}) error
}

// ProviderBase provides an embeddedable struct to ensure forward compatibility
// when implementing the Provider interface
type ProviderBase struct{}

// OptionsSchema returns nil as the base implementation doesn't take any options
func (ProviderBase) OptionsSchema() schematypes.Schema {
	return nil
}

var (
	providers  = make(map[string]Provider)
	mProviders = sync.Mutex{}
)

// Register will register a config provider. This is intended to be called at
// static initialization time (in func init()), and will thus panic if the
// given name already is in use.
func Register(name string, provider Provider) {
	mProviders.Lock()
	defer mProviders.Unlock()

	if _, ok := providers[name]; ok {
		panic(fmt.Sprintf("config.Proider name '%s' is already in use!", name))
	}

	providers[name] = provider
}

// Providers returns a mapping of the registered config providers.
func Providers() map[string]Provider {
	mProviders.Lock()
	defer mProviders.Unlock()

	// Clone providers
	m := map[string]Provider{}
	for name, provider := range providers {
		m[name] = provider
	}

	return m
}
