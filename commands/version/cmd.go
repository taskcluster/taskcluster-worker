package version

import (
	"encoding/json"
	"fmt"

	"github.com/taskcluster/taskcluster-worker/commands"
)

func init() {
	commands.Register("version", cmd{})
}

type cmd struct{}

func (cmd) Summary() string {
	return "Display version information"
}

func (cmd) Usage() string {
	return `
taskcluster-worker version will display version information.

usage: taskcluster-worker version [options] [semver|revision]

options:
  -j --json     Print as JSON.
  -h --help     Show this screen.
`
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	formatJSON := arguments["--json"].(bool)
	semver := arguments["semver"].(bool)
	revision := arguments["revision"].(bool)

	// Determine what to print
	var result map[string]string
	switch {
	case semver:
		result = map[string]string{
			"version": Version(),
		}
	case revision:
		result = map[string]string{
			"revision": Revision(),
		}
	default:
		result = map[string]string{
			"version":  Version(),
			"revision": Revision(),
		}
	}

	// Add defaults
	if v, ok := result["version"]; ok && v == "" {
		result["version"] = "unknown"
	}
	if v, ok := result["revision"]; ok && v == "" {
		result["revision"] = "unknown"
	}

	// Print as JSON or text
	if formatJSON {
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	} else {
		if _, ok := result["version"]; ok {
			fmt.Printf("version:  %s\n", result["version"])
		}
		if _, ok := result["revision"]; ok {
			fmt.Printf("revision: %s\n", result["revision"])
		}
	}

	return true
}
