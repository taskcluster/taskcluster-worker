package system

// System defines methods provided by a system implementation.
// This allows for easy mocking of the system, which is useful in tests.
type System interface {
	CreateUser(homeFolder string, groups []Group) (User, error)
	NewProcess(options ProcessOptions) Process
	CreateGroup() (Group, error)
	FindGroup(name string) (Group, error)
	Link(target, source string) error
	Unlink(target string) error
}

// default system type, platform specific methods are implemented in platform
// specific files.
type system struct{}

// Default system implementation
var Default System = system{}

// CreateUser will create a new user that can only write to the given
// homeDirectory.
func CreateUser(homeDirectory string) (User, error) {
	return Default.CreateUser(homeDirectory)
}

// NewProcess creates a new process with given arguments, environment variables,
// and current working folder, running as given user.
func NewProcess(options ProcessOptions) Process {
	return Default.NewProcess(options)
}
