// Package scripttest provides integration tests for a few common configuration
// of script and common plugins.
package scripttest

import (
	"encoding/json"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var debug = util.Debug("scripttest")

var engineConfig = `{
	"command": ["go", "run", ` + scriptPathJSON() + `],
	"schema": {
		"type": "object",
		"properties": {
			"delay": {"type": "integer", "minimum": 0},
			"message": {"type": "string"},
			"result": {"enum": ["pass", "fail", "malformed-payload", "non-fatal-error"]},
			"artifacts": {
				"type": "object",
				"additionalProperties": {"type": "string"}
			}
		},
		"required": ["result"]
	}
}`

const pluginConfig = `{
	"disabled": ["artifacts", "reboot", "env"],
	"maxruntime": {
		"perTaskLimit": "forbid",
		"maxRunTime": "1 m"
	},
	"success": {}
}`

// scriptPathJSON returns the path to ./testdata/script.go as JSON
func scriptPathJSON() string {
	// Get current working directory, tests always run from their current directory
	// so we can find ./testdata/ from there
	cwd, err := os.Getwd()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current working directory"))
	}
	// Join path and render as JSON
	result, _ := json.Marshal(path.Join(cwd, "testdata", "script.go"))
	return string(result)
}
