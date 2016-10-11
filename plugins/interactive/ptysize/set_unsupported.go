//+build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd

package ptysize

import (
	"os"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// Supported is true, if Set is supported
const Supported = false

// Set will set terminal size on pty, returns engines.ErrFeatureNotSupported on
// unsupported platforms.
func Set(pty *os.File, cols, rows uint16) error {
	return engines.ErrFeatureNotSupported
}
