package lifecyclepolicy

import (
	"fmt"

	schematypes "github.com/taskcluster/go-schematypes"
)

// ConfigSchema returns schema for config parameter passed to New()
//
// This will compose a schema of config options from all registered providers.
// For any providers to available users should import modules that register
// these as side-effects, typically, all sub-folders of this package.
func ConfigSchema() schematypes.Schema {
	options := make(schematypes.OneOf, 0, len(providers))
	for name, provider := range providers {
		option, err := schematypes.Merge(schematypes.Object{
			Properties: schematypes.Properties{
				"provider": schematypes.StringEnum{Options: []string{name}},
			},
			Required: []string{"provider"},
		}, provider.ConfigSchema())
		if err != nil {
			// This should never happen as we don't allow registring providers which
			// has AdditionalProperties: true, or defines a property "provider"
			panic(fmt.Sprintf("lifecyclepolicy.ConfigSchema(): cannot merge schemas, error: %s", err))
		}
		option.MetaData = provider.ConfigSchema().MetaData
		options = append(options, option)
	}
	return options
}

// New returns a new LifeCyclePolicy from config matching ConfigSchema().
func New(options Options) LifeCyclePolicy {
	schematypes.MustValidate(ConfigSchema(), options.Config)
	// This cast must pass as the config must match ConfigSchema, this is the
	// callers responsibility
	provider := options.Config.(map[string]interface{})["provider"].(string)

	mProviders.Lock()
	p := providers[provider]
	mProviders.Unlock()

	return p.NewLifeCyclePolicy(Options{
		Monitor: options.Monitor.WithTag("life-cycle-policy", provider),
		Config:  p.ConfigSchema().Filter(options.Config.(map[string]interface{})),
	})
}
