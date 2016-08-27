// +build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd

package shell

// SetupRawTerminal does nothing on unsupported platforms
func SetupRawTerminal(setSize func(cols, row uint16) error) func() {
	// Set default size, this is a somewhat sane thing to do on windows
	if setSize != nil {
		setSize(80, 20)
	}
	return func() {}
}
