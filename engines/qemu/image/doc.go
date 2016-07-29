// Package image exposes methods and abstractions for extracting and managing
// virtual machine images. Amongst other things this involves securing that
// the images don't reference external files as backing store.
package image

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("image")
