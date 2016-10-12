// Package configabs implements a TransformationProvider that replaces objects on
// the form: {$abs: "path"} with the value of current working folder + path.
package configabs

import (
	"fmt"
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/config"
)

type provider struct{}

func init() {
	config.Register("abs", provider{})
}

func (provider) Transform(cfg map[string]interface{}) error {
	return config.ReplaceObjects(cfg, "abs", func(val map[string]interface{}) (interface{}, error) {
		p := val["$abs"].(string)
		result, err := filepath.Abs(filepath.FromSlash(p))
		if err != nil {
			return nil, fmt.Errorf("Unable to resolve absolute path for: %s, error: %s", p, err)
		}
		return result, nil
	})
}
