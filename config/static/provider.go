package staticconfigprovider

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/worker"
)

type provider struct {
	config.ProviderBase
}

func init() {
	config.Register("static", provider{})
}

func (provider) OptionsSchema() schematypes.Schema {
	return worker.ConfigSchema()
}

func (provider) LoadConfig(options interface{}) (interface{}, error) {
	return options, nil
}
