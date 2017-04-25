// Package nativetest provides integration tests for a few common configuration
// of native and common plugins.
package nativetest

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("nativetest")

const engineConfig = `{
  "createUser": false
}`

const pluginConfig = `{
  "disabled": [],
  "artifacts": {},
  "env": {
    "extra": {"MY_STATIC_VAR": "static-value"}
  },
  "maxruntime": {
    "perTaskLimit": "require",
    "maxRunTime": "3 hours"
  },
  "livelog": {},
  "reboot":  {
    "allowTaskReboots": true
  },
  "success": {}
}`
