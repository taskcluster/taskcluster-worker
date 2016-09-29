// Package configenv implements a TransformationProvider that replaces objects on
// the form: {$env: "VAR"} with the value of the environment variable VAR.
package configenv

import (
	"os"

	"github.com/taskcluster/taskcluster-worker/config"
)

type provider struct{}

func init() {
	config.Register("env", provider{})
}

func (provider) Transform(cfg map[string]interface{}) error {
	return config.ReplaceObjects(cfg, "env", func(val map[string]interface{}) (interface{}, error) {
		return os.Getenv(val["$env"].(string)), nil
	})
}
