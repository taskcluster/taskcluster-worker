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

func (provider) Transform(config map[string]interface{}) error {
	injectEnv(config)
	return nil
}

func injectEnv(val interface{}) interface{} {
	switch val := val.(type) {
	case []interface{}:
		for i, v := range val {
			val[i] = injectEnv(v)
		}
	case map[string]interface{}:
		env, ok := val["$env"].(string)
		if ok && len(val) == 1 {
			return os.Getenv(env)
		}
		for k, v := range val {
			val[k] = injectEnv(v)
		}
	}
	return val
}
