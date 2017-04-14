// Package maxruntime provides a plugin for taskcluster-worker which can enforce
// a maximum runtime upon tasks. This will kill tasks that exceeds it, causing
// such tasks to be resolved as failed.
package maxruntime
