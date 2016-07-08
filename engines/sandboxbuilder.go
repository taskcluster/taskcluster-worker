package engines

import "net/http"

// The SandboxBuilder interface wraps the state required to start a Sandbox.
//
// Before returning a SandboxBuilder engine implementors should start
// downloading and setting up all the resources needed to start execution.
// A docker based engine may wish to ensure the docker image is downloaded, or
// lay a claim on it so the GarbageCollector won't remove it. A naive Windows
// engine may wish to create a new user account and setup a folder for the
// sandbox.
//
// Implementors can be sure that any instance of this interface will only be
// used to create a single Sandbox, that is StartSandbox() will atmost be called
// once. If StartSandbox() is called twice a sane implementor should return
// ErrContractViolation, and feel free to exhibit undefined behavior.
//
// All methods of this interface must be thread-safe.
type SandboxBuilder interface {
	// Attach a volume at given mountpoint.
	//
	// The volume given must have been created by this engine, using a method like
	// engine.NewVolume(). Implementors are free to make such a type assertion.
	//
	// The mountpoint is a string in engine-specific format. If the given
	// mountpoint violates the engine-specific format, a MalformedPayloadError
	// should be returned. For example a docker engine may expect the mountpoint
	// to be a path, where as a different engine might expect it to be a folder
	// name, or the name of an environement variable pointing to the folder.
	//
	// If the mountpoint is invalid because it's already in use a
	// MalformedPayloadError is also appropriate.
	//
	// If the engine doesn't support mutable or immutable volume attachments, it
	// should return ErrMutableMountNotSupported or ErrImmutableMountNotSupported,
	// respectively.
	//
	// Non-fatal errors: MalformedPayloadError, ErrMutableMountNotSupported,
	// ErrImmutableMountNotSupported, ErrFeatureNotSupported, ErrNamingConflict
	AttachVolume(mountpoint string, volume Volume, readOnly bool) error

	// Attach a proxy to the sandbox.
	//
	// The hostname is a engine-specific format. If the given hostname violates
	// engine-specific format, a MalformedPayloadError should be returned.
	// For example a docker engine may expect the hostname to be an actual
	// hostname, where as a different engine could have it being the path of a
	// unix-domain socket, or the prefix of a URL path.
	//
	// To ensure that all plugins works with all engines, AttachProxy should
	// always allow hostnames matching /[a-z]{3,22}/.
	//
	// It is the engines responsbility to ensure that requests aimed at the given
	// name is forwarded to the handler. And to ensure that no other processes are
	// able to forward requests to the handler.
	// When forarding the hostname should be set to what was given on attachment,
	// the any path prefixes should also be removed.
	//
	// If the engine doesn't support proxy attachments, it should return
	// ErrFeatureNotSupported.
	//
	// Non-fatal errors: MalformedPayloadError, ErrFeatureNotSupported,
	// ErrNamingConflict
	AttachProxy(hostname string, handler http.Handler) error

	// Set an environement variable.
	//
	// If the format of the environment variable name is invalid this method
	// should return a MalformedPayloadError with explaining why the name is
	// invalid.
	//
	// If the environment variable have previously been declared, this method
	// must return ErrNamingConflict.
	//
	// Non-fatal errors: ErrFeatureNotSupported, MalformedPayloadError,
	// ErrNamingConflict
	SetEnvironmentVariable(name string, value string) error

	// Start execution of task in sandbox. After a call to this method resources
	// held by the SandboxBuilder instance should be released or transferred to
	// the Sandbox implementation.
	//
	// Non-fatal errors: MalformedPayloadError, ErrSandboxBuilderDiscarded
	StartSandbox() (Sandbox, error)

	// Discard must free all resources held by the SandboxBuilder interface.
	// Any error returned is fatal, so do not return an error unless there is
	// something very wrong.
	Discard() error
}

// SandboxBuilderBase is a base implemenation of SandboxBuilder. It will
// implement all optional methods such that they return ErrFeatureNotSupported.
//
// Note: This will not implement StartSandbox() and other required methods.
//
// Implementors of SandBoxBuilder should embed this struct to ensure source
// compatibility when we add more optional methods to SandBoxBuilder.
type SandboxBuilderBase struct{}

// AttachVolume returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (SandboxBuilderBase) AttachVolume(string, Volume, bool) error {
	return ErrFeatureNotSupported
}

// AttachProxy returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (SandboxBuilderBase) AttachProxy(string, http.Handler) error {
	return ErrFeatureNotSupported
}

// SetEnvironmentVariable return ErrFeatureNotSupported indicating that the
// feature isn't supported.
func (SandboxBuilderBase) SetEnvironmentVariable(string, string) error {
	return ErrFeatureNotSupported
}

// Discard returns nil, indicating that resources have been released.
func (SandboxBuilderBase) Discard() error {
	return nil
}
