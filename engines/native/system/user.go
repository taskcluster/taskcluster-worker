package system

// User abstraction across platforms.
type User interface {
	// Remove user account, killing all processes owned by the user and removing
	// all resources outside the homeDirectory
	Remove() error
}
