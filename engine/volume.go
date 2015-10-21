package engine

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
	// Dispose deletes all resources used by the Volume
	Dispose() error
}
