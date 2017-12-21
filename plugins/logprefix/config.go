package logprefix

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var configSchema = schematypes.Map{
	Values: schematypes.String{},
	Title:  "Log Prefix",
	Description: util.Markdown(`
		Set of key-value pairs to be printed at the top of all tasks logs.

		Note. values given here can overwrite built-in key-value pairs.
	`),
}
