package engines

import "io"

// The VolumeBuilder interface wraps the process of building a volume.
// Notably, it permits writing of files and folders into the volume before it
// is created.
//
// Once a BuildVolume() or Discard() have been called the VolumeBuilder object
// is invalid and resources held by it should be considered transferred or
// released.
type VolumeBuilder interface {
	// Write a folder to the volume being built.
	//
	// Name must be a slash separated path, there is no requirement that
	// intermediate folders have been created.
	WriteFolder(name string) error

	// Write a file to the volime being built.
	//
	// Name must be a slash separated path, there is no requirement that
	// intermediate folders have been created.
	WriteFile(name string) io.WriteCloser

	// Build a volume from the information passed in.
	//
	// This invalidates the VolumeBuilder, which cannot be reused.
	BuildVolume() (Volume, error)

	// Discard resources held by the VolumeBuilder
	//
	// This invalidates the VolumeBuilder, which cannot be reused.
	Discard() error
}

// Volume that we can modify and mount on a Sandbox.
//
// Note, that engine implementations are not responsible for tracking the
// Volume, deletion and/or if it's mounted on more than one Sandbox at
// the same time.
//
// The engine is responsible for creating it, mounting it in sandboxes, loading
// data through the defined interface, extracting data through the defined
// interface and deleting the underlying storage when Dispose is called.
type Volume interface {
	// Dispose deletes all resources used by the Volume.
	Dispose() error
}

// VolumeBuilderBase is a base implemenation of VolumeBuilder. It will implement
// all optional methods such that they return ErrFeatureNotSupported.
//
// Implementors of VolumeBuilder should embed this struct to ensure source
// compatibility when we add more optional methods to VolumeBuilder.
type VolumeBuilderBase struct{}

// Discard returns nil, indicating that resources was released
func (VolumeBuilderBase) Discard() error {
	return nil
}

// VolumeBase is a base implemenation of Volume. It will implement all
// optional methods such that they return ErrFeatureNotSupported.
//
// Implementors of Volume should embed this struct to ensure source
// compatibility when we add more optional methods to Volume.
type VolumeBase struct{}

// Dispose returns nil indicating that resources were released.
func (VolumeBase) Dispose() error {
	return nil
}
