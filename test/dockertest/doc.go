// Package dockertest provides integration tests for a few common configuration
// of docker engine and common plugins.
package dockertest

import (
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var debug = util.Debug("dockertest")

var engineConfig = `{
	"privileged": "never"
}`

const pluginConfig = `{
	"disabled": ["reboot"],
	"artifacts": {},
	"env": {},
	"livelog": {},
	"success": {},
	"watchdog": {},
	"cache": {},
	"maxruntime": {
    "perTaskLimit": "require",
    "maxRunTime": "3 hours"
  }
}`

const dockerImageName = "alpine:3.6"
