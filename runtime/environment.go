package runtime

import (
	"os"
	"runtime"

	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

// Environment is a collection of objects that makes up a runtime environment.
type Environment struct {
	GarbageCollector *gc.GarbageCollector
	//TODO: Add some sort of interface to the system logger
	//TODO: Add some interface to submit statistics for influxdb/signalfx
	//TODO: Add some interface to attach a http.Handler to public facing server
	TemporaryStorage TemporaryStorage
}

// NewTestEnvironment creates a new Environment suitable for use in tests.
//
// This function should only be used in testing
func NewTestEnvironment() *Environment {
	storage, err := NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic(err)
	}
	folder, err := storage.NewFolder()
	if err != nil {
		panic(err)
	}
	// Set finalizer so that we always get the temporary folder removed.
	// This is should really only be used in tests, otherwise it would better to
	// call Remove() manually.
	runtime.SetFinalizer(folder, func(f TemporaryFolder) {
		f.Remove()
	})
	return &Environment{
		GarbageCollector: &gc.GarbageCollector{},
		TemporaryStorage: folder,
	}
}
