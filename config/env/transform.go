// Package configenv implements a TransformationProvider that replaces objects on
// the form: {$env: "VAR"} with the value of the environment variable VAR.
//
// An additional key 'type' may be given as {$env: "VAR", type: "TYPE"}, which
// determines how the environment variable value should be parsed. By default
// the TYPE is 'string', but the following types are supported:
//   - string, value is return as string (default),
//   - number, value is parsed a number,
//   - json, value is parsed as JSON,
//   - bool, value becomes true if it matches 'true' (case insensitive), and,
//   - list, value is parsed as space separated list of strings.
package configenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type provider struct{}

func init() {
	config.Register("env", provider{})
}

func (provider) Transform(cfg map[string]interface{}, monitor runtime.Monitor) error {
	return config.ReplaceObjects(cfg, "env", func(val map[string]interface{}) (interface{}, error) {
		env := val["$env"].(string)
		value := os.Getenv(env)
		typ, ok := val["type"]
		if !ok {
			typ = "string"
		}
		t, ok := typ.(string)
		if !ok {
			return nil, errors.New("'type' property in {$env, type} is not a string")
		}
		switch t {
		case "string":
			return value, nil
		case "number":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf(
					"Error parsing number from {$env: '%s'}, error: %s", env, err,
				)
			}
			return v, nil
		case "json":
			var retval interface{}
			err := json.Unmarshal([]byte(value), &retval)
			if err != nil {
				return nil, fmt.Errorf(
					"Error parsing JSON from {$env: '%s'}, error: %s", env, err,
				)
			}
			return retval, nil
		case "bool":
			return strings.ToLower(value) == "true", nil
		case "list":
			parts := strings.Split(value, " ")
			retval := make([]interface{}, len(parts))
			for i, s := range parts {
				retval[i] = s
			}
			return retval, nil
		default:
			return nil, fmt.Errorf("Unsupported type: '%s' in {$env, type}", t)
		}
	})
}
